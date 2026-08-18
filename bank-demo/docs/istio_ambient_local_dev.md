# Istio Ambient + Waypoint: Local Dev Routing on GKE ARM64

> Header-based routing from cluster pods → developer's laptop via Tailscale, with zero sidecars.

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

### 1. Headless Services → ORIGINAL_DST → Broken

Istio ambient treats headless Services (`clusterIP: None`) as `ORIGINAL_DST` clusters.
The waypoint ignores EndpointSlice addresses and tries to connect to the DNS-resolved IP.

**Fix**: Use a **ClusterIP Service** (not headless) with a manually-created EndpointSlice.
This forces Istio to use **EDS** (Endpoint Discovery Service) and respect the EndpointSlice addresses.

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

### 2. VirtualService vs HTTPRoute in Ambient

In ambient mode, **ztunnel does NOT process VirtualService rules**. Ztunnel only escalates
traffic to the waypoint when the destination service has an HTTPRoute/AuthorizationPolicy
attached via the Gateway API.

**Fix**: Use `HTTPRoute` with `parentRefs` pointing to the target Service, not VirtualService.

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

### 3. Ztunnel Must Be hostNetwork (for Tailscale)

Ztunnel runs as a DaemonSet. Without `hostNetwork: true`, ztunnel is in its own pod
network namespace and **cannot reach Tailscale IPs** (even if the Tailscale subnet router
runs on the same node).

**Fix**: Patch ztunnel DaemonSet with `hostNetwork: true` and `dnsPolicy: ClusterFirstWithHostNet`.

### 4. Tailscale Must Run on ALL Nodes

A single Tailscale router pod only gives Tailscale connectivity to one node. Pods on other
nodes (and their ztunnel) can't reach the developer's laptop.

**Fix**: Deploy Tailscale as a **DaemonSet** with `hostNetwork: true` AND `dnsPolicy: ClusterFirstWithHostNet`.

> **Critical**: When using `hostNetwork: true`, pods default to using the host's DNS resolver, which can't resolve cluster-internal service names (e.g., `payments-api.demo-bank.svc.cluster.local`). Setting `dnsPolicy: ClusterFirstWithHostNet` forces the pod to use the cluster DNS (CoreDNS) first, falling back to host DNS. Without this, any pod doing DNS lookups (like router pods or sidecars) will fail to resolve cluster services.

```yaml
spec:
  hostNetwork: true
  dnsPolicy: ClusterFirstWithHostNet  # REQUIRED with hostNetwork
```

### 5. GKE CNI Compatibility

GKE uses a managed CNI (Dataplane V2 / Cilium). Istio's CNI plugin must coexist by using
`global.platform=gke`, which writes binaries to an alternate path.

```bash
helm upgrade istio-cni istio/cni -n istio-system --set global.platform=gke
```

### 6. ARM64 Tolerations Everywhere

GKE ARM nodes have a `kubernetes.io/arch=arm64:NoSchedule` taint. Every workload needs:

```yaml
tolerations:
- key: kubernetes.io/arch
  operator: Equal
  value: arm64
  effect: NoSchedule
```

This includes: istiod, ztunnel, istio-cni, waypoint, Tailscale, test pods, and app pods.

### 7. GKE system-node-critical Priority Quota

GKE limits `system-node-critical` priority pods. Istio's ztunnel and CNI DaemonSets
default to this priority class, which can hit quota limits.

**Fix**: Remove `priorityClassName` from the DaemonSets if they fail to schedule.

### 8. ServiceEntry + K8s Service Name Conflicts

If a K8s Service and a ServiceEntry share the same FQDN, the K8s Service wins and
the ServiceEntry endpoints are ignored.

**Fix**: Use distinct names, or don't mix them for the same host.

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

### 1. Install Istio Ambient

```bash
# Download istioctl
curl -L https://istio.io/downloadIstio | ISTIO_VERSION=1.25.2 sh -

# Install base + istiod with ambient profile
istioctl install --set profile=ambient -y

# Patch istiod for ARM
kubectl patch deployment istiod -n istio-system --type=json -p='[
  {"op":"add","path":"/spec/template/spec/tolerations","value":[
    {"key":"kubernetes.io/arch","operator":"Equal","value":"arm64","effect":"NoSchedule"}
  ]}
]'

# Install CNI via Helm (GKE-compatible)
helm repo add istio https://istio-release.storage.googleapis.com/charts
helm install istio-cni istio/cni -n istio-system \
  --set global.platform=gke \
  --set cni.ambient.enabled=true

# Install ztunnel via Helm
helm install ztunnel istio/ztunnel -n istio-system

# Patch ztunnel for hostNetwork + ARM
kubectl patch ds ztunnel -n istio-system --type=json -p='[
  {"op":"add","path":"/spec/template/spec/hostNetwork","value":true},
  {"op":"add","path":"/spec/template/spec/dnsPolicy","value":"ClusterFirstWithHostNet"},
  {"op":"add","path":"/spec/template/spec/tolerations","value":[
    {"key":"kubernetes.io/arch","operator":"Equal","value":"arm64","effect":"NoSchedule"}
  ]}
]'
```

### 2. Enable Ambient + Waypoint

```bash
# Label namespace for ambient
kubectl label namespace demo-bank istio.io/dataplane-mode=ambient

# Deploy waypoint + enroll namespace
istioctl waypoint apply -n demo-bank --enroll-namespace

# Patch waypoint for ARM
kubectl patch deployment waypoint -n demo-bank --type=json -p='[
  {"op":"add","path":"/spec/template/spec/tolerations","value":[
    {"key":"kubernetes.io/arch","operator":"Equal","value":"arm64","effect":"NoSchedule"}
  ]}
]'
```

### 3. Deploy Tailscale DaemonSet

```bash
kubectl create namespace tailscale

# See k8s/tailscale-daemonset.yaml in the config repo
kubectl apply -f k8s/tailscale-daemonset.yaml
```

### 4. Create Routing Resources

```bash
# See k8s/local-dev-routing.yaml in the config repo
kubectl apply -f k8s/local-dev-routing.yaml
```

### 5. Run Local Service

```bash
cd services/payments-api
PORT=9090 APP_VERSION=local-dev go run main.go
```

### 6. Test

```bash
# With header → laptop
kubectl exec curl-test -n demo-bank -- \
  curl -s -H "x-diverge-env: main" http://payments-api:8080/health
# → {"service":"payments-api","status":"ok","version":"local-dev"}

# Without header → baseline
kubectl exec curl-test -n demo-bank -- \
  curl -s http://payments-api:8080/health
# → {"service":"payments-api","status":"ok","version":"baseline"}
```

---

## Mesh Config

```yaml
# meshConfig in istio ConfigMap
outboundTrafficPolicy:
  mode: ALLOW_ANY
```

---

## What Diverge Controller Needs to Create

When a developer runs `diverge dev --service payments-api`, the controller should create:

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

### Lesson #9: KNative Namespaces Must NOT Be Ambient

KNative's scale-to-zero relies on L7 request buffering via the Activator. Ztunnel (L4-only) breaks this flow — cold-start requests timeout because ztunnel can't parse HTTP headers needed for revision routing.

```bash
# CORRECT — no ambient label
kubectl create namespace demo-knative

# WRONG — will break scale-to-zero
kubectl label namespace demo-knative istio.io/dataplane-mode=ambient  # DON'T
```

### Lesson #10: Enable `podspec-tolerations` for ARM64

KNative's admission webhook blocks tolerations by default. On ARM64-only clusters, every KNative Service will fail to create unless you enable the feature flag:

```bash
kubectl patch configmap/config-features -n knative-serving \
  --type merge -p '{"data":{"kubernetes.podspec-tolerations":"enabled"}}'
```

### Lesson #11: Patch ALL KNative System Deployments

KNative has 6 deployments across 2 namespaces that all need ARM64 tolerations:

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

### Lesson #12: Kourier Over net-gateway-api

`net-gateway-api` is beta and requires building from source with `ko`. Kourier is the KNative default, stable, and lightweight. Use Kourier unless you have a specific reason not to.

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
