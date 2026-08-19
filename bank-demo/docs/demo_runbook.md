# Diverge Bank Demo — Runbook

> **Cluster**: `diverge-demo-arm` (GKE Standard, ARM64, `us-central1`)
> **Mesh**: Istio Ambient (zero sidecars)
> **Tunnel**: Tailscale DaemonSet (WireGuard P2P)
> **Gateway IP**: `http://34.44.49.235`

---

## Prerequisites

The repo includes a `flake.nix` + `.envrc`. With [direnv](https://direnv.net/) installed:

```bash
cd /Users/ab/code/divergedev/demo
direnv allow   # one-time — auto-loads go, air, kubectl, node into PATH
```

If `direnv` doesn't activate in subdirectories, prefix commands with:
```bash
nix develop /Users/ab/code/divergedev/demo -c <command>
```

---

## Terminal Setup (5 Tabs)

Open 5 terminal tabs/panes. Each has a specific role:

### Tab 1 — 🧹 Demo Commands
```
📂 cd /Users/ab/code/divergedev/demo/bank-demo
```
For `kubectl`, `curl`, cleanup, and general commands.

### Tab 2 — 🔥 Local Go API (air hot-reload)
```
📂 cd /Users/ab/code/divergedev/demo/bank-demo/services/payments-api
```
Runs the Go payments-api locally with hot-reload.

### Tab 3 — 🎨 Local React Frontend (air + vite build)
```
📂 cd /Users/ab/code/divergedev/demo/bank-demo/services/payments-module
```
Runs Vite build + Go static server locally with hot-reload.

### Tab 4 — 🌐 Diverge CLI
```
📂 cd /Users/ab/code/divergedev/demo/bank-demo
```
Runs `diverge dev` to register the local endpoint with the cluster.

### Tab 5 — 📊 Dashboard
```
📂 cd /Users/ab/code/divergedev/demo/bank-demo/dashboard
```
Runs the real-time cluster monitoring UI at `http://localhost:3333`.

---

## Step-by-Step

### 1. Start the Dashboard (Tab 5)

```bash
# Tab 5 — Dashboard
cd /Users/ab/code/divergedev/demo/bank-demo/dashboard
./serve.sh
```
Open **http://localhost:3333** — shows cluster status, active previews, Tailscale connectivity.

### 2. Verify Baseline (Tab 1)

```bash
# Tab 1 — Demo Commands
# Verify all services running
kubectl get pods -n demo-bank

# Test baseline (no fraud detection)
curl -s http://34.44.49.235/api/payments | python3 -c "
import sys,json; d=json.load(sys.stdin)
print(f'version={d[\"version\"]}  flagged={d[\"flagged_count\"]}  fees={d[\"total_fees\"]}')
"
# Expected: version=baseline  flagged=0  fees=0
```

Open **http://34.44.49.235** in the browser — shows Diverge Bank with all services at baseline.

### 3. Start Go Hot-Reload (Tab 2)

```bash
# Tab 2 — Local Go API
cd /Users/ab/code/divergedev/demo/bank-demo/services/payments-api
nix develop /Users/ab/code/divergedev/demo -c sh -c 'APP_VERSION=fraud-detection PORT=9090 air'
```

> [!IMPORTANT]
> - `APP_VERSION=fraud-detection` makes `isPreview()` return `true` → enables fraud fields
> - `PORT=9090` avoids conflicting with the cluster's port 8080

Verify it works locally:
```bash
# From Tab 1
curl -s http://localhost:9090/health
# Expected: {"service":"payments-api","status":"ok","version":"fraud-detection"}
```

### 4. Start React Hot-Reload (Tab 3)

```bash
# Tab 3 — Local React Frontend
cd /Users/ab/code/divergedev/demo/bank-demo/services/payments-module
nix develop /Users/ab/code/divergedev/demo -c sh -c 'VITE_BASE_PATH=/modules/dev-local/ APP_VERSION=fraud-detection PORT=5173 air'
```

> [!IMPORTANT]
> - `VITE_BASE_PATH=/modules/dev-local/` matches the gateway's routing path
> - `APP_VERSION=fraud-detection` sets the module version
> - `PORT=5173` serves on the port the ts-proxy forwards
> - Air watches both `.tsx` and `.go` files — rebuilds Vite then Go server (~3s)

Verify it works locally:
```bash
# From Tab 1
curl -s http://localhost:5173/remoteEntry.js | head -1
# Expected: JavaScript module federation code
```

### 5. Start Diverge Dev (Tab 4)

```bash
# Tab 4 — Diverge CLI
cd /Users/ab/code/divergedev/demo/bank-demo
/Users/ab/code/divergedev/diverge/bin/diverge dev --service payments-api --namespace demo-bank --port 9090 --preview-id fraud-detection
```

This registers your local Tailscale IP with the cluster. The gateway routes both Go API and React module traffic to your machine.

### 6. Test Preview Routing (Tab 1)

```bash
# Tab 1 — With preview header
curl -s http://34.44.49.235/api/payments -H "x-preview-id: fraud-detection" | python3 -c "
import sys,json; d=json.load(sys.stdin)
print(f'version={d[\"version\"]}  flagged={d[\"flagged_count\"]}  fees={d[\"total_fees\"]}')
"
# Expected: version=fraud-detection  flagged=3  fees=463.99
```

Open in browser: **http://34.44.49.235/?x-preview-id=fraud-detection**

You should see:
- 🔴 **Fraud Detection Active** banner with 3 suspicious transactions
- Service Topology showing `payments-api` and `payments-module` in cyan
- Fee column with sparkle ✨ indicator

---

## What to Edit for Live Demo

Save the file → tool rebuilds automatically → Refresh browser.

### 🔧 Backend Edits — Go (air ~1s rebuild)

**File**: `services/payments-api/main.go`

> [!IMPORTANT]
> The fraud-detection preview uses `previewTxns` (line 49–58), NOT `baselineTxns` (line 38–47).
> Edits to `baselineTxns` won't show up in the preview view.

#### A. Flag more transactions as fraudulent
**Line 53** — Change `tx_004` (wire transfer) from unflagged to flagged:
```diff
- {ID: "tx_004", From: "checking", To: "wire", Amount: 3500.00, FraudScore: 0.45, FraudFlag: false, Fee: 52.50, Status: "completed"},
+ {ID: "tx_004", From: "checking", To: "wire", Amount: 3500.00, FraudScore: 0.85, FraudFlag: true, Fee: 52.50, Status: "pending"},
```
**Effect**: Fraud alert count goes from 3 → 4, wire transfer row turns red with 85% Risk badge.

#### B. Increase a fraud score to 99%
**Line 54** — Boost the offshore transaction:
```diff
- {ID: "tx_005", From: "acct_unknown", To: "acct_offshore_001", Amount: 15000.00, FraudScore: 0.92, FraudFlag: true, Fee: 225.00, Status: "pending"},
+ {ID: "tx_005", From: "acct_unknown", To: "acct_offshore_001", Amount: 15000.00, FraudScore: 0.99, FraudFlag: true, Fee: 225.00, Status: "pending"},
```
**Effect**: Risk badge changes from `92% Risk` → `99% Risk`.

#### C. Change a transaction amount
**Line 50** — Change the first transaction amount:
```diff
- {ID: "tx_001", From: "savings", To: "checking", Amount: 100.00, FraudScore: 0.05, FraudFlag: false, Fee: 3.75, Status: "completed"},
+ {ID: "tx_001", From: "savings", To: "checking", Amount: 999.99, FraudScore: 0.05, FraudFlag: false, Fee: 3.75, Status: "completed"},
```
**Effect**: Amount column shows $999.99, Total Volume changes.

#### D. Double all fees
**Line 174** — Change fee accumulation:
```diff
- totalFees += t.Fee
+ totalFees += t.Fee * 2
```
**Effect**: Total Fees jumps from ~$464 → ~$928. Shows how a fee policy change propagates instantly.

#### E. Add a brand new suspicious transaction
**After line 57** — Add tx_009:
```go
{ID: "tx_009", From: "checking", To: "casino_offshore", Amount: 50000.00, FraudScore: 0.98, FraudFlag: true, Fee: 750.00, Status: "pending"},
```
**Effect**: New row appears, flagged count goes to 4, Total Fees increases by $750.

---

### 🎨 Frontend Edits — React/TypeScript (air + vite build ~3s rebuild)

**File**: `services/payments-module/frontend/src/PaymentsPanel.tsx`

#### F. Change the fraud banner text
**Line ~105** — Edit the banner title:
```diff
- Fraud Detection Active
+ 🛡️ AI Fraud Shield Active
```
**Effect**: Banner title changes after rebuild.

#### G. Change the subtitle
**Line ~106** — Edit the subtitle:
```diff
- suspected transactions intercepted
+ threats neutralized by AI
```
**Effect**: Subtitle text updates.

#### H. Change risk badge color thresholds
Adjust when dots go red/yellow/green by changing the fraud score boundaries.

---

## Architecture — How Traffic Flows

```
                    ┌─────────────┐
                    │   Browser   │
                    │  ?x-preview │
                    │  -id=fraud  │
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │ LoadBalancer │
                    │ 34.44.49.235│
                    └──────┬──────┘
                           │
                    ┌──────▼──────────────────────────┐
                    │           Gateway                │
                    │  checks x-preview-id header      │
                    │  matches DIVERGE_DEV_PREVIEW_ID?  │
                    └──┬──────────┬──────────┬─────────┘
            no match   │    API match        │ module match
                       │          │          │
              ┌────────▼──┐  ┌───▼────┐  ┌──▼───────┐
              │payments-api│  │ts-proxy│  │ts-proxy  │
              │ (cluster)  │  │ :9090  │  │ :5173    │
              │ baseline   │  └───┬────┘  └──┬───────┘
              └────────────┘      │          │
                            socat + tailscale nc
                                  │          │
                           ┌──────▼──────────▼──────┐
                           │       Your Mac          │
                           │  air :9090  air :5173   │
                           │  (Go API)   (React UI)  │
                           └─────────────────────────┘
```

## Key URLs

| URL | What it shows |
|---|---|
| `http://34.44.49.235` | Baseline — no fraud detection |
| `http://34.44.49.235/?x-preview-id=fraud-detection` | Your local dev — full-stack preview |
| `http://34.44.49.235/?x-preview-id=anything-else` | Falls through to baseline |
| `http://localhost:9090/health` | Local air health check (Go API) |
| `http://localhost:5173/remoteEntry.js` | Local module entry (React) |
| `http://localhost:3333` | Dashboard — cluster monitoring |

## Dev Server Comparison

| | Backend (Go) | Frontend (React) |
|---|---|---|
| **Dev tool** | `air` | `air` (vite build + go serve) |
| **Local port** | `:9090` | `:5173` |
| **Cluster route** | `ts-dev-proxy:9090` | `ts-dev-proxy:5173` |
| **Edit file** | `payments-api/main.go` | `payments-module/.../PaymentsPanel.tsx` |
| **Rebuild speed** | ~1s (Go compile) | ~3s (Vite build + Go compile) |
| **See change** | Refresh browser | Refresh browser |

## Inspect — kubectl Commands for the Demo

Use these during the demo to show what's happening inside the cluster.

### Show pods — zero sidecars (Istio Ambient)
```bash
kubectl get pods -n demo-bank
# All pods show 1/1 — no sidecar injection
# Compare to traditional Istio: would show 2/2 (app + envoy sidecar)
```

### Show the Tailscale tunnel proxy
```bash
# Dual-port service routing to your Mac
kubectl get svc ts-dev-proxy -n demo-bank
# NAME           TYPE        CLUSTER-IP       PORT(S)
# ts-dev-proxy   ClusterIP   34.118.231.119   9090/TCP,5173/TCP

# Verify tunnel is connected
kubectl logs deploy/ts-proxy -n demo-bank --tail=10
# Look for: "Tailscale connected!" and "Proxy :9090 -> air" / "Proxy :5173 -> vite"
```

### Show gateway routing decisions
```bash
# Watch gateway logs in real-time while making requests
kubectl logs deploy/gateway -n demo-bank -f --tail=5
```

### Show the preview environment variables on gateway
```bash
kubectl get deploy/gateway -n demo-bank -o jsonpath='{range .spec.template.spec.containers[0].env[*]}{.name}={.value}{"\n"}{end}' | grep DIVERGE
# DIVERGE_DEV_ENDPOINT=ts-dev-proxy:9090
# DIVERGE_DEV_PREVIEW_ID=fraud-detection
# DIVERGE_DEV_MODULE_ENDPOINT=ts-dev-proxy:5173
```

### Verify baseline vs preview (external curl)
```bash
# Baseline — hits cluster payments-api directly
curl -s http://34.44.49.235/api/payments | python3 -c "
import sys,json; d=json.load(sys.stdin)
print(f'version={d[\"version\"]}  fraud={d[\"fraud_detection\"]}')
"
# version=baseline  fraud=False

# Preview — routes through ts-proxy → Tailscale → your Mac
curl -s http://34.44.49.235/api/payments -H "x-preview-id: fraud-detection" | python3 -c "
import sys,json; d=json.load(sys.stdin)
print(f'version={d[\"version\"]}  fraud={d[\"fraud_detection\"]}  flagged={d[\"flagged_count\"]}')
"
# version=fraud-detection  fraud=True  flagged=3
```

### Show the topology endpoint
```bash
curl -s http://34.44.49.235/topology | python3 -m json.tool
# All "baseline" — no preview

curl -s http://34.44.49.235/topology -H "x-preview-id: fraud-detection" | python3 -m json.tool
# payments-api: "fraud-detection", payments-module: "fraud-detection"
```

### Show Istio ambient mesh (no sidecars, just ztunnel)
```bash
# ztunnel runs as DaemonSet — handles L4 mTLS transparently
kubectl get ds -n istio-system
# NAME      DESIRED   CURRENT   READY
# ztunnel   3         3         3

# Waypoint proxy handles L7 (HTTPRoute) for the namespace
kubectl get pods -n demo-bank -l gateway.networking.k8s.io/gateway-name=waypoint
```

### Show what's running locally
```bash
# Check local processes
lsof -i :9090 -P | head -3   # air — Go payments-api
lsof -i :5173 -P | head -3   # air — React payments-module
lsof -i :3333 -P | head -3   # dashboard

# Check Tailscale connection
tailscale status | grep gke
# gke-diverge-proxy  100.x.x.x  linux  -
```

---

## Cleanup

```bash
# Tab 2: Ctrl+C (stop air — payments-api)
# Tab 3: Ctrl+C (stop air — payments-module)
# Tab 4: Ctrl+C (stop diverge dev — auto-cleans K8s resources)
# Tab 5: Ctrl+C (stop dashboard)
```

### Verify clean state
```bash
# Confirm baseline is restored
curl -s http://34.44.49.235/api/payments | python3 -c "
import sys,json; d=json.load(sys.stdin)
print(f'version={d[\"version\"]}  flagged={d[\"flagged_count\"]}')
"
# Expected: version=baseline  flagged=0
```
