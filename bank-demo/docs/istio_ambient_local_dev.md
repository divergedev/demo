# Istio Ambient + Waypoint: Local Dev Routing on GKE ARM64

> Header-based routing from cluster pods → developer's laptop via Tailscale, with zero sidecars.

## Table of Contents

- [Quick Setup Checklist](#quick-setup-checklist)
- [Architecture](#architecture)
- [Traffic Flow](#traffic-flow)
- [Hard-Won Lessons](#hard-won-lessons)
- [Cluster Details](#cluster-details)
- [Setup Steps](#setup-steps-from-scratch)
- [Mesh Config](#mesh-config)
- [Controller Reference (Future Work)](#controller-reference-future-work)
- [KNative Serverless Previews](#knative-serverless-previews-alongside-ambient)
- [Verification Commands](#verification-commands)

---

## Quick Setup Checklist

Before diving into the details, you can provision the entire environment using the automated scripts in `k8s/infra/`:

- [ ] Run `k8s/infra/01-install-istio.sh` to install Istio Ambient profile and CNI.
- [ ] Run `k8s/infra/02-deploy-tailscale.sh` to install Tailscale DaemonSet.
- [ ] Run `k8s/infra/03-setup-knative.sh` to install KNative Serving with Kourier.
- [ ] Run `k8s/infra/04-deploy-demo-apps.sh` to deploy the baseline services.

---

## Architecture

```
Developer Laptop (macOS)                    GKE Cluster (ARM64, t2a-standard-4)
┌──────────────────────┐                    ┌─────────────────────────────────────────────┐
│                      │                    │  Istio Ambient Mesh                         │
│  payments-api :9090  │◄───Tailscale───────│  ┌──────────────────────────────────────┐   │
│  (hot reload)        │    100.86.x.x      │  │ demo-bank namespace                  │   │
│                      │                    │  │                                      │   │
│  Tailscale           │                    │  │  curl → ztunnel → waypoint (L7)      │   │
│  100.86.105.10       │                    │  │           │                          │   │
└──────────────────────┘                    │  │           ├── header match?           │   │
                                            │  │           │   YES → diverge-local-ep  │   │
                                            │  │           │         (100.86.105.10)   │   │
                                            │  │           │   NO  → payments-api pod  │   │
                                            │  │           │         (10.4.x.x)        │   │
                                            │  └──────────────────────────────────────┘   │
                                            │                                             │
                                            │  ┌──────────────────────────────────────┐   │
                                            │  │ demo-knative namespace (NO ambient)  │   │
                                            │  │                                      │   │
                                            │  │  order-api (scale-to-zero)           │   │
                                            │  │  order-api-pr-123 (scale-to-zero)    │   │
                                            │  └──────────────────────────────────────┘   │
                                            │                                             │
                                            │  ztunnel (DaemonSet, hostNetwork)            │
                                            │  tailscale (DaemonSet, hostNetwork)          │
                                            │  istio-cni (DaemonSet, global.platform=gke) │
                                            └─────────────────────────────────────────────┘
```

## Traffic Flow

```
1. App pod sends request to payments-api:8080
2. ztunnel (L4) intercepts, sees namespace has waypoint → forwards to waypoint
3. Waypoint (Envoy, L7) matches HTTPRoute:
   - Header "x-diverge-env: main" → route to diverge-local-ep:9090
   - No header → route to payments-api:8080
4. Waypoint sends to diverge-local-ep:9090
5. ztunnel (on waypoint's node, hostNetwork) forwards to 100.86.105.10:9090
6. Tailscale (on same node, hostNetwork) tunnels to developer's laptop
```

---

## Hard-Won Lessons

> [!CAUTION]
> These lessons cost hours of debugging. Do not skip them.

### Lesson 1: Headless Services → ORIGINAL_DST → Broken

**Problem**: Istio ambient treats headless Services (`clusterIP: None`) as `ORIGINAL_DST` clusters. The waypoint ignores EndpointSlice addresses and tries to connect to the DNS-resolved IP.

**Fix**: Use a **ClusterIP Service** (not headless) with a manually-created EndpointSlice.

**Why**: This forces Istio to use **EDS** (Endpoint Discovery Service) and respect the EndpointSlice addresses.

```yaml
# ✅ CORRECT: ClusterIP service → EDS → respects EndpointSlice
apiVersion: v1
kind: Service
metadata:
  name: diverge-local-ep
spec:
  ports:
  - name: http
    port: 9090

# ❌ WRONG: Headless service → ORIGINAL_DST → ignores EndpointSlice
apiVersion: v1
kind: Service
metadata:
  name: diverge-local-ep
spec:
  clusterIP: None   # DON'T DO THIS
```

### Lesson 2: VirtualService vs HTTPRoute in Ambient

**Problem**: In ambient mode, ztunnel does NOT process VirtualService rules. Ztunnel only escalates traffic to the waypoint when the destination service has an HTTPRoute/AuthorizationPolicy attached via the Gateway API.

**Fix**: Use `HTTPRoute` with `parentRefs` pointing to the target Service, not VirtualService.

**Why**: VirtualServices are largely deprecated in ambient routing, which fully embraces Gateway API.

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: payments-api-diverge
spec:
  parentRefs:
  - name: payments-api     # The DESTINATION service
    kind: Service
    group: ""
    port: 8080
  rules:
  - matches:
    - headers:
      - name: x-diverge-env
        value: main
    backendRefs:
    - name: diverge-local-ep
      port: 9090
  - backendRefs:
    - name: payments-api
      port: 8080
```

### Lesson 3: Ztunnel Must Be hostNetwork (for Tailscale)

**Problem**: Ztunnel runs as a DaemonSet. Without `hostNetwork: true`, ztunnel is in its own pod network namespace and cannot reach Tailscale IPs (even if the Tailscale subnet router runs on the same node).

**Fix**: Patch ztunnel DaemonSet with `hostNetwork: true` and `dnsPolicy: ClusterFirstWithHostNet`.

**Why**: ztunnel needs access to the host's networking stack to route traffic into the Tailscale interface.

### Lesson 4: Tailscale Must Run on ALL Nodes

**Problem**: A single Tailscale router pod only gives Tailscale connectivity to one node. Pods on other nodes (and their ztunnel) can't reach the developer's laptop.

**Fix**: Deploy Tailscale as a **DaemonSet** with `hostNetwork: true` AND `dnsPolicy: ClusterFirstWithHostNet`.

**Why**: Cross-node Tailscale routing requires every node's ztunnel to have local access to a Tailscale interface.

> **Critical**: When using `hostNetwork: true`, pods default to using the host's DNS resolver, which can't resolve cluster-internal service names (e.g., `payments-api.demo-bank.svc.cluster.local`). Setting `dnsPolicy: ClusterFirstWithHostNet` forces the pod to use the cluster DNS (CoreDNS) first, falling back to host DNS. Without this, any pod doing DNS lookups (like router pods or sidecars) will fail to resolve cluster services.

```yaml
spec:
  hostNetwork: true
  dnsPolicy: ClusterFirstWithHostNet  # REQUIRED with hostNetwork
```

### Lesson 5: GKE CNI Compatibility

**Problem**: GKE uses a managed CNI (Dataplane V2 / Cilium). Istio's CNI plugin will conflict if installed normally.

**Fix**: Istio's CNI plugin must coexist by using `global.platform=gke`, which writes binaries to an alternate path.

**Why**: GKE restricts standard CNI binary paths; this flag tells Istio to use the GKE-specific paths.

```bash
helm upgrade istio-cni istio/cni -n istio-system --set global.platform=gke
```

### Lesson 6: ARM64 Tolerations Everywhere

**Problem**: GKE ARM nodes have a `kubernetes.io/arch=arm64:NoSchedule` taint, preventing normal pods from scheduling.

**Fix**: Every workload needs an ARM64 toleration.

**Why**: Without it, pods remain Pending. This includes istiod, ztunnel, istio-cni, waypoint, Tailscale, test pods, and app pods:


```yaml
tolerations:
- key: kubernetes.io/arch
  operator: Equal
  value: arm64
  effect: NoSchedule
```

This includes: istiod, ztunnel, istio-cni, waypoint, Tailscale, test pods, and app pods.

### Lesson 7: GKE system-node-critical Priority Quota

**Problem**: GKE limits `system-node-critical` priority pods. Istio's ztunnel and CNI DaemonSets default to this priority class, which can hit quota limits.

**Fix**: Remove `priorityClassName` from the DaemonSets if they fail to schedule.

**Why**: The quota is hardcoded in GKE Standard, and DaemonSets will be permanently stuck without this change.

### Lesson 8: ServiceEntry + K8s Service Name Conflicts

**Problem**: If a K8s Service and a ServiceEntry share the same FQDN, the K8s Service wins and the ServiceEntry endpoints are ignored.

**Fix**: Use distinct names, or don't mix them for the same host.

**Why**: Istio prioritizes native Kubernetes Services over custom ServiceEntries when resolving internal cluster domains.

---

## Cluster Details

| Resource | Value |
|----------|-------|
| **GCP Project** | `austin-demo-diverge` |
| **Cluster** | `diverge-demo-arm` |
| **Region** | `us-central1` |
| **Node type** | `t2a-standard-4` (ARM64) |
| **Nodes** | 3 |
| **K8s version** | v1.35.6-gke.1641000 |
| **Istio version** | istiod 1.25.2, CNI/ztunnel 1.30.3 |
| **Tailscale** | DaemonSet, all nodes |

---

## Setup Steps (from scratch)

Instead of running commands manually, use the automated scripts provided in `k8s/infra/`:

### 1. Install Istio Ambient

```bash
./k8s/infra/01-install-istio.sh
```

### 2. Enable Ambient + Waypoint

```bash
kubectl label namespace demo-bank istio.io/dataplane-mode=ambient
istioctl waypoint apply -n demo-bank --enroll-namespace
```

### 3. Deploy Tailscale DaemonSet

```bash
./k8s/infra/02-deploy-tailscale.sh
```

### 4. Setup KNative

```bash
./k8s/infra/03-setup-knative.sh
```

### 5. Create Routing Resources and Demo Apps

```bash
./k8s/infra/04-deploy-demo-apps.sh
kubectl apply -f k8s/local-dev-routing.yaml
```

---

## Mesh Config

```yaml
# meshConfig in istio ConfigMap
outboundTrafficPolicy:
  mode: ALLOW_ANY
```

---

## Controller Reference (Future Work)

When the automated controller is built in the future, running `diverge dev --service payments-api` should create:

1. **ClusterIP Service** (NOT headless) — e.g., `diverge-local-ep`
2. **EndpointSlice** — pointing to developer's Tailscale IP + port
3. **HTTPRoute** — matching header, routing to the local endpoint service
4. **Waypoint** (if not exists) — `istioctl waypoint apply`

```go
// Pseudocode for LocalDeployer
func (d *LocalDeployer) Deploy(env *Environment) error {
    // 1. Create ClusterIP Service
    svc := &corev1.Service{
        Spec: corev1.ServiceSpec{
            // NO selector, NO clusterIP: None
            Ports: []corev1.ServicePort{{
                Name: "http", Port: env.Spec.ServiceConfig.Port,
            }},
        },
    }

    // 2. Create EndpointSlice
    eps := &discoveryv1.EndpointSlice{
        AddressType: discoveryv1.AddressTypeIPv4,
        Endpoints: []discoveryv1.Endpoint{{
            Addresses: []string{env.Spec.ServiceConfig.Endpoint},
        }},
        Ports: []discoveryv1.EndpointPort{{
            Name: ptr("http"), Port: ptr(env.Spec.ServiceConfig.Port),
        }},
    }

    // 3. Create HTTPRoute (Gateway API)
    route := &gatewayv1.HTTPRoute{
        Spec: gatewayv1.HTTPRouteSpec{
            ParentRefs: []gatewayv1.ParentReference{{
                Name:  gatewayv1.ObjectName(targetServiceName),
                Kind:  ptr(gatewayv1.Kind("Service")),
                Group: ptr(gatewayv1.Group("")),
                Port:  ptr(gatewayv1.PortNumber(targetServicePort)),
            }},
            Rules: []gatewayv1.HTTPRouteRule{
                // header match → local endpoint
                // default → original service
            },
        },
    }
}
```

---

## KNative Serverless Previews (Alongside Ambient)

KNative Serving runs alongside Istio Ambient in the same cluster, in a **separate namespace** that is NOT enrolled in ambient mode.

### Lesson 9: KNative Namespaces Must NOT Be Ambient

**Problem**: KNative's scale-to-zero relies on L7 request buffering via the Activator. Ztunnel (L4-only) breaks this flow — cold-start requests timeout because ztunnel can't parse HTTP headers needed for revision routing.

**Fix**: Do not label KNative application namespaces with `istio.io/dataplane-mode=ambient`.

**Why**: The activator must sit directly in the request path to wake up scaled-to-zero pods, and ztunnel bypasses it.

```bash
# CORRECT — no ambient label
kubectl create namespace demo-knative

# WRONG — will break scale-to-zero
kubectl label namespace demo-knative istio.io/dataplane-mode=ambient  # DON'T
```

### Lesson 10: Enable `podspec-tolerations` for ARM64

**Problem**: KNative's admission webhook blocks tolerations by default. On ARM64-only clusters, every KNative Service will fail to create.

**Fix**: Enable the feature flag for tolerations.

**Why**: KNative attempts to keep Service specs minimal, but ARM deployments require the toleration explicitly.

```bash
kubectl patch configmap/config-features -n knative-serving \
  --type merge -p '{"data":{"kubernetes.podspec-tolerations":"enabled"}}'
```

### Lesson 11: Patch ALL KNative System Deployments

**Problem**: KNative has 6 deployments across 2 namespaces that will fail to schedule on ARM nodes.

**Fix**: All these deployments need ARM64 tolerations manually added.

**Why**: The official KNative manifests don't include ARM tolerations out of the box.

```bash
# knative-serving namespace
for deploy in activator autoscaler controller webhook net-kourier-controller; do
  kubectl patch deployment $deploy -n knative-serving --type=json -p='[
    {"op":"add","path":"/spec/template/spec/tolerations","value":[
      {"key":"kubernetes.io/arch","operator":"Equal","value":"arm64","effect":"NoSchedule"}
    ]}
  ]'
done

# kourier-system namespace
kubectl patch deployment 3scale-kourier-gateway -n kourier-system --type=json -p='[
  {"op":"add","path":"/spec/template/spec/tolerations","value":[
    {"key":"kubernetes.io/arch","operator":"Equal","value":"arm64","effect":"NoSchedule"}
  ]}
]'
```

### Lesson 12: Kourier Over net-gateway-api

**Problem**: KNative can integrate with Gateway API via `net-gateway-api`, but it is currently beta and requires source compilation.

**Fix**: Use Kourier as the networking layer instead.

**Why**: Kourier is the KNative default, stable, lightweight, and works seamlessly for our requirements.

### KNative + Diverge Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                  GKE Cluster (ARM64)                            │
│                                                                 │
│  demo-bank (Istio Ambient + Waypoint)                          │
│  ├── gateway, payments-api, accounts-api, web-app (baseline)   │
│  ├── Waypoint proxy (L7 header routing)                        │
│  └── diverge dev → Tailscale → laptop (local mode)             │
│                                                                 │
│  demo-knative (KNative + Kourier, NO ambient)                  │
│  ├── order-api (ksvc, scale-to-zero)                           │
│  ├── notification-api (ksvc, scale-to-zero)                    │
│  └── order-api-pr-123 (ksvc, PR preview, scale-to-zero)       │
│                                                                 │
│  Three deployment modes:                                        │
│  ├── local:      Tailscale → laptop (hot reload, 0s)           │
│  ├── image:      Regular pods (always-on, 0s cold start)       │
│  └── serverless: KNative (scale-to-zero, ~5s cold start)       │
└─────────────────────────────────────────────────────────────────┘
```

---

## Verification Commands

Use these commands to verify your setup is working correctly:

**Check Istio components:**
```bash
kubectl get pods -n istio-system
```

**Check Tailscale connectivity:**
```bash
TS_POD=$(kubectl get pods -n tailscale -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n tailscale $TS_POD -- tailscale ping -c 1 100.86.105.10
```

**Verify KNative scaling:**
```bash
kubectl get pods -n demo-knative
```
