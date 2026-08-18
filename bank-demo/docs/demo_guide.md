# Diverge Demo Presentation Guide

> **Cluster**: `diverge-demo-arm` (GKE Standard, ARM64, us-central1)
> **Mesh**: Istio Ambient + Waypoint (zero sidecars)
> **Project root**: `~/code/divergedev`
> **Duration**: ~15 minutes

---

## Table of Contents

- [Pre-Demo Checklist](#pre-demo-checklist)
- [Act 1: The Baseline](#act-1-the-baseline-2-min)
- [Act 2: `diverge dev` — Hot Reload in the Cloud](#act-2-diverge-dev--hot-reload-in-the-cloud-5-min)
- [Act 3: Parallel Fullstack Previews](#act-3-parallel-fullstack-previews-3-min)
- [Act 4: Serverless PR Previews](#act-4-serverless-pr-previews-3-min)
- [Act 4b: Local Dev → KNative](#act-4b-local-dev--knative-1-min)
- [Act 5: Cleanup](#act-5-cleanup-1-min)
- [Architecture Diagram](#architecture-diagram)
- [Key Talking Points](#key-talking-points)
- [Quick Reference Card](#quick-reference-card)

---

## Pre-Demo Checklist

**Terminal Setup**
Open 4 terminal tabs and navigate to their starting directories:

- **Tab 1 (Demo commands)**: `~/code/divergedev/demo/bank-demo`
- **Tab 2 (Local API)**: `~/code/divergedev/demo/bank-demo/services/payments-api`
- **Tab 3 (Diverge)**: `~/code/divergedev/diverge`
- **Tab 4 (Dashboard)**: `~/code/divergedev/demo/bank-demo/dashboard`

### Verify infrastructure

```bash
# Tab 1
echo "=== Mesh ===" && kubectl get pods -n istio-system
echo "=== Tailscale ===" && kubectl get ds -n tailscale
echo "=== Baseline ===" && kubectl get pods -n demo-bank
echo "=== Waypoint ===" && kubectl get gateway -n demo-bank
```

**Expected**: istiod (1), ztunnel (3), istio-cni (3), tailscale (3), waypoint (1), 5 app pods.

### Reset to clean state

```bash
# Tab 1
kubectl delete previewgroup --all 2>/dev/null
kubectl delete httproute payments-api-diverge -n demo-bank 2>/dev/null
kubectl delete svc diverge-local-ep -n demo-bank 2>/dev/null
kubectl delete endpointslice diverge-local-ep-manual -n demo-bank 2>/dev/null
kubectl get pods -n demo-bank        # Verify 5 baseline + 1 waypoint
```

### Start the dashboard

```bash
# Tab 4
cd ~/code/divergedev/demo/bank-demo/dashboard
chmod +x serve.sh && ./serve.sh
# Opens at http://localhost:3333
```

> [!TIP]
> Keep the dashboard visible on a second monitor or half-screen during the presentation.

---

## Act 1: The Baseline (2 min)

**Talking point**: *"Here's our bank demo — five microservices running on ARM in GKE. Istio Ambient mesh — no sidecars, pods are all 1/1."*

```bash
# Tab 1
kubectl get pods -n demo-bank -o wide
```

```
NAME                               READY   STATUS    NODE
accounts-api-...                   1/1     Running   gke-diverge-demo-arm-...
gateway-...                        1/1     Running   gke-diverge-demo-arm-...
payments-api-...                   1/1     Running   gke-diverge-demo-arm-...
payments-module-...                1/1     Running   gke-diverge-demo-arm-...
waypoint-...                       1/1     Running   gke-diverge-demo-arm-...
web-app-...                        1/1     Running   gke-diverge-demo-arm-...
```

> [!TIP]
> Point out: **"1/1 — no sidecars."** In ambient mode, the mesh is handled by per-node ztunnel (L4) and the waypoint proxy (L7). Zero injection, zero pod restarts.

```bash
# Hit the baseline payments-api
kubectl run demo-1 --rm --attach --restart=Never -n demo-bank \
  --image=curlimages/curl:latest \
  --overrides='{"spec":{"tolerations":[{"key":"kubernetes.io/arch","operator":"Equal","value":"arm64","effect":"NoSchedule"}]}}' \
  -- curl -s http://payments-api:8080/health 2>&1 | grep -v '^pod\b'
```

```json
{"service":"payments-api","status":"ok","version":"baseline"}
```

**Talking point**: *"Version `baseline`. Now let's say I'm working on a fix and want to test it against the real cluster — without building an image, without deploying."*

---

## Act 2: `diverge dev` — Hot Reload in the Cloud (5 min)

### Step 1: Start local service with live reload

```bash
# Tab 2
cd ~/code/divergedev/demo/bank-demo/services/payments-api

PORT=9090 air
```

> [!TIP]
> `air` watches for file changes and auto-rebuilds. The `.air.toml` config is already in the service directory.

**Talking point**: *"Running payments-api on my laptop with `air` — a live-reload watcher. Every time I save a file, it rebuilds in milliseconds. No Docker, no container, no redeploy."*

### Step 2: Start diverge dev

```bash
# Tab 3
cd ~/code/divergedev/diverge

./bin/diverge dev \
  --service payments-api \
  --namespace demo-bank \
  --port 9090
```

Expected output:
```
Starting dev session for service "payments-api"...
Routing traffic with header x-diverge-env: <branch-name> to 100.86.105.10:9090

✅ PreviewGroup: dev-ab-payments-api
   Phase: Running
   Header: x-diverge-env: <branch-name>

   SERVICE         NAMESPACE  PHASE    URL
   ───────         ─────────  ─────    ───
   ✅ payments-api  demo-bank  Running  -
```

> [!TIP]
> **Point at the dashboard** — it should show the new PreviewGroup appearing in real time.

**Talking point**: *"One command. Diverge detected my Tailscale IP, created a PreviewGroup, a ClusterIP Service with EndpointSlice pointing to my laptop, and an HTTPRoute for header-based routing through the waypoint."*

### Step 3: Show what Diverge created

```bash
# Tab 1
kubectl get previewgroup
kubectl get httproute -n demo-bank
kubectl get svc diverge-local-ep -n demo-bank
kubectl get endpointslice diverge-local-ep-manual -n demo-bank
```

**Talking point**: *"All standard Kubernetes resources — Gateway API HTTPRoute, Service, EndpointSlice. No CRD magic, no vendor lock-in."*

### Step 4: Prove traffic routing — the magic moment ✨

```bash
# Tab 1 — baseline (no header)
kubectl run demo-2 --rm --attach --restart=Never -n demo-bank \
  --image=curlimages/curl:latest \
  --overrides='{"spec":{"tolerations":[{"key":"kubernetes.io/arch","operator":"Equal","value":"arm64","effect":"NoSchedule"}]}}' \
  -- curl -s http://payments-api:8080/health 2>&1 | grep -v '^pod\b'
```
```json
{"service":"payments-api","status":"ok","version":"baseline"}
```

```bash
# Tab 1 — with header → YOUR LAPTOP
kubectl run demo-3 --rm --attach --restart=Never -n demo-bank \
  --image=curlimages/curl:latest \
  --overrides='{"spec":{"tolerations":[{"key":"kubernetes.io/arch","operator":"Equal","value":"arm64","effect":"NoSchedule"}]}}' \
  -- curl -s -H "x-diverge-env: main" \
  http://payments-api:8080/health 2>&1 | grep -v '^pod\b'
```
```json
{"service":"payments-api","status":"ok","version":"local-dev"}
```

> [!IMPORTANT]
> **This is the magic moment.** Same cluster, same service name — but the header routes through Tailscale WireGuard tunnel to your Mac. No Docker build, no push, no deploy. Edit your code, save, instant hot reload.
> 
> *[pause for effect]*

### Step 5: Show the network path

**Talking point**: *"How does this work?"*

```
Request + header x-diverge-env: main
  → ztunnel (L4, per-node DaemonSet)
    → Waypoint proxy (L7, Envoy)
      → HTTPRoute matches header
        → ClusterIP Service (EDS → 100.86.105.10:9090)
          → ztunnel (outbound, hostNetwork)
            → Tailscale WireGuard tunnel
              → My Mac, port 9090
                → payments-api (version: local-dev) ✅
```

```bash
# Tab 1 — prove Tailscale is direct (not relayed)
TS_POD=$(kubectl get pods -n tailscale -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n tailscale $TS_POD -- tailscale ping -c 1 100.86.105.10
```
```
pong from austins-macbook-air-1 (100.86.105.10) via 24.6.16.123:60688 in 57ms
```

**Talking point**: *"Direct WireGuard connection. 57ms. Encrypted. No VPN appliance. Tailscale runs on every node as a DaemonSet."*

### Step 6: Show hot reload — the money shot 🔥

```bash
# Tab 2 — open main.go in your editor and change the version string
# Find this line:
#   version := getEnv("APP_VERSION", "baseline")
# Change "baseline" to "fix-v2" (or any string)
```

> [!IMPORTANT]
> **Don't touch Tab 2** — `air` is still running. Just edit `main.go` in your editor.
> Watch Tab 2: air detects the change, rebuilds in ~200ms, restarts the server.

```bash
# Tab 1 — immediately test again (air already rebuilt)
kubectl run demo-hr --rm --attach --restart=Never -n demo-bank \
  --image=curlimages/curl:latest \
  --overrides='{"spec":{"tolerations":[{"key":"kubernetes.io/arch","operator":"Equal","value":"arm64","effect":"NoSchedule"}]}}' \
  -- curl -s -H "x-diverge-env: main" \
  http://payments-api:8080/health 2>&1 | grep -v '^pod\b'
```
```json
{"service":"payments-api","status":"ok","version":"fix-v2"}
```

**Talking point**: *"I edited one line. Saved. Air rebuilt in 200ms. The cluster is already serving the new version — through a WireGuard tunnel from Kubernetes to my laptop. No image build. No push. No deploy. No manual restart. Just save."*

---

## Act 3: Parallel Fullstack Previews (3 min)

**Talking point**: *"Now imagine another developer on a different team is working on a completely different feature."*

### Step 1: Apply Sarah's PreviewGroup manifest

```bash
# Tab 1
kubectl apply -f manifests/previews/account-alerts.yaml
```

> Sarah's feature touches **accounts-api** AND **gateway** — two services in one PreviewGroup.

**Talking point**: *"Sarah's team is building account alerts. Their feature touches two services — fullstack change, fullstack preview. One manifest, applied in one command."*

### Step 2: Show both previews coexisting

```bash
# Tab 1
kubectl get previewgroup
```

```
NAME                  PHASE     SERVICES   HEADER
dev-ab-payments-api   Running   1          main                     ← You (local dev)
feat-account-alerts   Running   2          account-alerts           ← Sarah (image deploy)
```

> [!TIP]
> **Point at the dashboard** — both PreviewGroups are visible, each with their own services and headers.

```bash
# Tab 1 — show the pods
kubectl get pods -n demo-bank
```

```
NAME                                    READY   STATUS    RESTARTS   AGE
accounts-api-6c78f845b7-zqgc7          1/1     Running   0          60m
accounts-api-preview-74c6547f9d-fr2vf   1/1     Running   0          30s   ← Sarah's preview
gateway-54bfbccb84-f97rv                1/1     Running   0          60m
payments-api-654567d4c6-kxqsd           1/1     Running   0          60m
payments-module-689ff4fd8-ljmrh         1/1     Running   0          60m
waypoint-9f65f85d9-kvtpr                1/1     Running   0          15m
web-app-9f9bc7546-9cwl8                 1/1     Running   0          60m
```

**Talking point**: *"Notice — Sarah's preview spun up a new pod. But mine didn't. My code runs on my laptop. Two deployment modes, same routing infrastructure."*

### Step 3: Demonstrate independent routing

```bash
# No header → shared baseline
kubectl run demo-4 --rm --attach --restart=Never -n demo-bank \
  --image=curlimages/curl:latest \
  --overrides='{"spec":{"tolerations":[{"key":"kubernetes.io/arch","operator":"Equal","value":"arm64","effect":"NoSchedule"}]}}' \
  -- curl -s http://payments-api:8080/health 2>&1 | grep -v '^pod\b'
# → {"version":"baseline"}

# Your header → your local laptop
kubectl run demo-5 --rm --attach --restart=Never -n demo-bank \
  --image=curlimages/curl:latest \
  --overrides='{"spec":{"tolerations":[{"key":"kubernetes.io/arch","operator":"Equal","value":"arm64","effect":"NoSchedule"}]}}' \
  -- curl -s -H "x-diverge-env: main" \
  http://payments-api:8080/health 2>&1 | grep -v '^pod\b'
# → {"version":"local-dev"}

# Sarah's header → her deployed preview
kubectl run demo-6 --rm --attach --restart=Never -n demo-bank \
  --image=curlimages/curl:latest \
  --overrides='{"spec":{"tolerations":[{"key":"kubernetes.io/arch","operator":"Equal","value":"arm64","effect":"NoSchedule"}]}}' \
  -- curl -s -H "x-diverge-env: account-alerts" \
  http://accounts-api:8080/health 2>&1 | grep -v '^pod\b'
# → {"version":"account-alerts-v1"}
```

**Talking point**: *"Three results, three realities — all on the same cluster, same namespace, same service names. The only difference is the header."*

---

## Act 3b: Browser UI — See It Live (2 min)

**Talking point**: *"curl is great for proving the routing. But your users don't use curl. Let me show you what this actually looks like."*

### Step 1: Port-forward the gateway

```bash
# Tab 1
kubectl port-forward svc/gateway -n demo-bank 8080:8080
```

### Step 2: Open baseline in browser

Open: **http://localhost:8080**

> [!TIP]
> The app shows "🏦 Diverge Bank" with a gray **BASELINE** badge. The topology diagram shows all services running baseline versions. The payments panel loads from the baseline module.

**Talking point**: *"This is the app as it exists today. Shared baseline. Every developer sees this."*

### Step 3: Switch to fraud detection preview

Add the preview ID to the URL: **http://localhost:8080/?x-preview-id=fraud-detection**

> [!IMPORTANT]
> Watch what happens:
> 1. Badge changes from gray **BASELINE** to cyan **PREVIEW fraud-detection**
> 2. Topology diagram highlights `payments-api` node in cyan — it's running a different version
> 3. Payments panel reloads from the **preview module** — now showing:
>    - 🚨 **Fraud Detection Active** alert banner (animated pulse)
>    - New "Fraud Risk" column in the transactions table
>    - Suspicious transactions flagged
>
> *[pause — let the audience absorb]*

**Talking point**: *"Same URL, same app shell. I just added a query parameter. The gateway resolved the preview service, loaded a different micro-frontend module, and the fraud detection feature appeared — without touching the baseline. Open another tab without the parameter — baseline. Side by side."*

### Step 4: Show the preview controls

> The app has built-in preview controls (top-right). Type `account-alerts` and click Apply — the app routes to Sarah's preview instead. Click Clear — back to baseline.

**Talking point**: *"Developers can share preview URLs with reviewers. QA can toggle between environments. PMs can see features before they're merged. No staging environment needed."*

## Act 4: Serverless PR Previews (3 min)

**Talking point**: *"Now the big one. What happens when a PR is opened? Traditional preview environments spin up pods and leave them running 24/7. With Diverge + KNative, previews cost zero when nobody's looking."*

### Step 1: Show KNative services at zero

```bash
# Tab 1
kubectl get ksvc -n demo-knative
```

```
NAME               URL                                                      READY
order-api          http://order-api.demo-knative.svc.cluster.local          True
notification-api   http://notification-api.demo-knative.svc.cluster.local   True
```

```bash
# Zero pods!
kubectl get pods -n demo-knative
# No resources found in demo-knative namespace.
```

**Talking point**: *"Two KNative services. Ready. But zero pods. Zero cost."*

### Step 2: Simulate a PR opening

```bash
# Tab 1 — a PR is opened, Diverge creates a serverless preview
kubectl apply -f manifests/previews/order-api-pr-123.yaml
```

```bash
# Still zero pods!
kubectl get pods -n demo-knative
# No resources found
```

**Talking point**: *"PR opened. Preview created. Still zero pods. The service exists but nothing is running — it only wakes up when someone sends a request."*

### Step 3: Reviewer opens the preview — cold start ✨

```bash
# Tab 1 — simulate a reviewer hitting the preview
kubectl run kn-review --rm --attach --restart=Never -n demo-knative \
  --image=curlimages/curl:latest \
  --overrides='{"spec":{"tolerations":[{"key":"kubernetes.io/arch","operator":"Equal","value":"arm64","effect":"NoSchedule"}]}}' \
  -- curl -s --max-time 15 \
  http://order-api-pr-123.demo-knative.svc.cluster.local/health 2>&1 | grep -v '^pod\b'
```

```json
{"service":"payments-api","status":"ok","version":"pr-123-fix-order-validation"}
```

(~5 second pause while Activator wakes the pod)

> [!IMPORTANT]
> **This is the serverless magic.** The request arrived, the Activator buffered it, spun up a pod, and forwarded the response. The reviewer sees `pr-123-fix-order-validation`. After 30 seconds of idle, the pod vanishes.

### Step 4: Show the contrast — baseline vs PR

```bash
# Baseline
kubectl run kn-base --rm --attach --restart=Never -n demo-knative \
  --image=curlimages/curl:latest \
  --overrides='{"spec":{"tolerations":[{"key":"kubernetes.io/arch","operator":"Equal","value":"arm64","effect":"NoSchedule"}]}}' \
  -- curl -s --max-time 15 \
  http://order-api.demo-knative.svc.cluster.local/health 2>&1 | grep -v '^pod\b'
# → {"version":"knative-baseline"}

# PR preview
kubectl run kn-pr --rm --attach --restart=Never -n demo-knative \
  --image=curlimages/curl:latest \
  --overrides='{"spec":{"tolerations":[{"key":"kubernetes.io/arch","operator":"Equal","value":"arm64","effect":"NoSchedule"}]}}' \
  -- curl -s --max-time 15 \
  http://order-api-pr-123.demo-knative.svc.cluster.local/health 2>&1 | grep -v '^pod\b'
# → {"version":"pr-123-fix-order-validation"}
```

**Talking point**: *"Same service, different versions. The baseline runs the merged code. The PR preview runs the branch code. Both scale to zero independently."*

### Step 5: Cost comparison

**Talking point**: *"Imagine 50 open PRs. Traditional approach: 50 pods running 24/7 — about $1,200/month. With KNative: only the PRs being actively reviewed have pods. Overnight? Zero pods. Zero cost."*

| Scenario | Always-on pods | KNative serverless |
|----------|---------------|--------------------|
| 50 PRs, 3 active reviewers | 50 pods 24/7 | 3 pods |
| Overnight | 50 pods | **0 pods** |
| Monthly cost (t2a-standard-4) | ~$1,200 | ~$15 |

---

## Act 4b: Local Dev → KNative (1 min)

**Talking point**: *"Before a PR, developers need a fast inner loop. The same service that deploys as serverless KNative in CI can be developed locally with hot reload."*

```bash
# Tab 2 — run the payments-api locally (it serves as order-api in KNative context)
cd ~/code/divergedev/demo/bank-demo/services/payments-api
PORT=9090 APP_VERSION=local-order-fix SERVICE_NAME=order-api go run main.go
```

```bash
# Tab 3 — diverge dev routes cluster traffic to laptop
./bin/diverge dev --service payments-api --namespace demo-bank --port 9090
```

**Talking point**: *"Same code. Locally, it's hot reload via Tailscale. Push a PR, it becomes a serverless KNative preview. Merge, it updates the baseline. Three modes, one codebase, zero config changes."*

---

## Act 5: Cleanup (1 min)

```bash
# Tab 3 — Ctrl+C the diverge dev session (auto-cleans up)

# Tab 2 — stop the local payments-api
kill $(lsof -ti :9090)   # or Ctrl+C

# Tab 1 — verify cleanup
kubectl get previewgroup        # No resources found
kubectl get httproute -n demo-bank   # Only waypoint remains
kubectl delete ksvc order-api-pr-123 -n demo-knative 2>/dev/null # Clean up KNative PR
```

**Talking point**: *"Ctrl+C. Everything is cleaned up. No zombie environments."*

---

## Architecture Diagram

```
┌───────────────────────────────────────────────────────────────────┐
│                    GKE Cluster (ARM64, Istio Ambient)             │
│                                                                   │
│  Per-node (DaemonSets):                                          │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────────┐    │
│  │   ztunnel    │  │  istio-cni   │  │  tailscale-router    │    │
│  │ (L4, mTLS)  │  │ (GKE compat) │  │ (hostNet, WireGuard) │    │
│  │  hostNetwork │  │              │  │                      │    │
│  └─────────────┘  └──────────────┘  └──────────────────────┘    │
│                                                                   │
│  demo-bank namespace:                                            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌───────────────┐   │
│  │ gateway  │  │ web-app  │  │ accounts │  │ payments-api  │   │
│  │   1/1    │  │   1/1    │  │   -api   │  │     1/1       │   │
│  │ no sidecar│  │no sidecar│  │   1/1    │  │  no sidecar   │   │
│  └──────────┘  └──────────┘  └──────────┘  └───────────────┘   │
│                                                                   │
│  demo-knative namespace (Serverless):                            │
│  ┌─────────────────┐ ┌────────────────┐ ┌──────────────────┐     │
│  │   order-api     │ │notification-api│ │ order-api-pr-123 │     │
│  │  (scale-to-zero)│ │ (scale-to-zero)│ │ (scale-to-zero)  │     │
│  └─────────────────┘ └────────────────┘ └──────────────────┘     │
│                                                                   │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │                Waypoint Proxy (Envoy, L7)                │    │
│  │                                                          │    │
│  │  HTTPRoute: payments-api-diverge                         │    │
│  │    header "x-diverge-env: main"                         │    │
│  │    → diverge-local-ep:9090 (EDS → 100.86.105.10)       │    │
│  │    default → payments-api:8080 (baseline)               │    │
│  └──────────────────────────────────────────────────────────┘    │
│                         │                                         │
│            ztunnel → Tailscale WireGuard                         │
│                         │                                         │
└─────────────────────────│─────────────────────────────────────────┘
                          │  direct P2P, ~57ms, encrypted
                          ▼
              ┌──────────────────────┐
              │   Developer Laptop   │
              │                      │
              │  go run main.go      │
              │  payments-api:9090   │
              │  version: local-dev  │
              │                      │
              │  Tailscale           │
              │  100.86.105.10       │
              └──────────────────────┘
```

---

## Key Talking Points

1. **Zero-sidecar mesh**: Istio Ambient — no injection, no restarts. Pods stay 1/1. L4 via ztunnel, L7 via waypoint.

2. **Hot reload inner loop**: `diverge dev` — no Docker build, no image push. Local code receives real cluster traffic instantly.

3. **Gateway API HTTPRoute**: Standard Kubernetes routing. Header match → local endpoint. No vendor lock-in.

4. **Tailscale mesh**: WireGuard encrypted tunnel, direct P2P (~57ms), runs on every node as a DaemonSet. Works through NAT/firewalls.

5. **Independent team previews**: Each fullstack team gets their own PreviewGroup. Multiple services per preview. Zero coordination.

6. **ARM64 native**: Entire demo on Google T2A nodes — 40% cheaper than x86.

7. **ClusterIP + EDS**: Diverge uses ClusterIP Services (not headless) with manual EndpointSlices. This forces Istio to use EDS and correctly resolve to the Tailscale IP.

8. **Serverless PR previews**: KNative scale-to-zero for PR previews. 50 PRs, only active reviewers cost compute. ~$15/month vs ~$1,200.

9. **Three deployment modes**: Local dev (hot reload), image deploy (always-on), serverless (scale-to-zero). Same routing, same headers.

---

## Routing Resources Created by `diverge dev`

| Resource | Name | Purpose |
|----------|------|---------|
| **Service** (ClusterIP) | `diverge-local-ep` | Backend for HTTPRoute, triggers EDS resolution |
| **EndpointSlice** | `diverge-local-ep-manual` | Points to developer's Tailscale IP (100.86.105.10:9090) |
| **HTTPRoute** | `payments-api-diverge` | Header match → local ep, default → baseline |
| **PreviewGroup** (CRD) | `dev-ab-payments-api` | Tracks the dev session lifecycle |

---

## File Reference

| File | Purpose |
|------|---------|
| [`manifests/previews/account-alerts.yaml`](file:///Users/ab/code/divergedev/demo/bank-demo/manifests/previews/account-alerts.yaml) | Sarah's PreviewGroup (Act 3) |
| [`services/payments-api/main.go`](file:///Users/ab/code/divergedev/demo/bank-demo/services/payments-api/main.go) | Local payments-api (Act 2) |
| [`dashboard/serve.sh`](file:///Users/ab/code/divergedev/demo/bank-demo/dashboard/serve.sh) | Dashboard server |
| [`dashboard/index.html`](file:///Users/ab/code/divergedev/demo/bank-demo/dashboard/index.html) | Dashboard UI |
| [`~/code/divergedev/diverge/bin/diverge`](file:///Users/ab/code/divergedev/diverge/bin/diverge) | CLI binary |

---

## Troubleshooting

| Issue | Fix |
|-------|-----|
| Pods stuck `Pending` | ARM toleration missing: `kubectl describe pod <name>` |
| Waypoint `NodePorts` crash loop | Remove `hostNetwork: true` from waypoint deployment |
| Traffic not reaching waypoint | Verify: `istioctl proxy-config routes <waypoint-pod>.demo-bank` |
| Ztunnel can't reach Tailscale | Verify ztunnel has `hostNetwork: true` and Tailscale DS is on same node |
| `ORIGINAL_DST` in proxy config | Using headless service — switch to ClusterIP (see Lesson #1 in [istio_ambient_local_dev.md](file:///Users/ab/.gemini/antigravity/brain/e8f0441b-6aba-42b6-87e8-a50ae3af5ee3/istio_ambient_local_dev.md)) |
| Baseline returned for header test | Check waypoint endpoints: `istioctl proxy-config endpoints <wp>.demo-bank \| grep diverge` |
| `diverge dev` provider error | Patch CRD enum: see [walkthrough](file:///Users/ab/.gemini/antigravity/brain/e8f0441b-6aba-42b6-87e8-a50ae3af5ee3/walkthrough.md) |
| Dashboard not updating | Check `serve.sh` is running and kubectl context is correct |
| GKE priority quota limits | Remove `priorityClassName` from ztunnel/istio-cni DaemonSets |
| Istio CNI `read-only filesystem` | Re-install with `--set global.platform=gke` |
| KNative pods stuck Pending | Enable tolerations: `kubectl patch configmap/config-features -n knative-serving --type merge -p '{"data":{"kubernetes.podspec-tolerations":"enabled"}}'` |
| KNative `must not set tolerations` | Same fix — enable `kubernetes.podspec-tolerations` in config-features |
| KNative cold start > 10s | Check node resources, image pull time. Pre-warm with `min-scale: 1` if needed |

---

## Quick Reference Card

### Essential curl commands

**Hit the baseline payments-api:**
```bash
kubectl run demo-curl --rm --attach --restart=Never -n demo-bank \
  --image=curlimages/curl:latest \
  --overrides='{"spec":{"tolerations":[{"key":"kubernetes.io/arch","operator":"Equal","value":"arm64","effect":"NoSchedule"}]}}' \
  -- curl -s http://payments-api:8080/health 2>&1 | grep -v '^pod\b'
```

**Hit the local-dev payments-api (via header):**
```bash
kubectl run demo-curl --rm --attach --restart=Never -n demo-bank \
  --image=curlimages/curl:latest \
  --overrides='{"spec":{"tolerations":[{"key":"kubernetes.io/arch","operator":"Equal","value":"arm64","effect":"NoSchedule"}]}}' \
  -- curl -s -H "x-diverge-env: main" http://payments-api:8080/health 2>&1 | grep -v '^pod\b'
```

**Hit the KNative PR preview:**
```bash
kubectl run kn-pr --rm --attach --restart=Never -n demo-knative \
  --image=curlimages/curl:latest \
  --overrides='{"spec":{"tolerations":[{"key":"kubernetes.io/arch","operator":"Equal","value":"arm64","effect":"NoSchedule"}]}}' \
  -- curl -s --max-time 15 http://order-api-pr-123.demo-knative.svc.cluster.local/health 2>&1 | grep -v '^pod\b'
```
