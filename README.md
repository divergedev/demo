# Diverge Demo

> Try multi-repo preview environments in 5 minutes.

## Bank Demo

A 4-service fintech architecture demonstrating header-based routing:

```
web-app → gateway → payments-api → accounts-api
```

Open an MR on **one** service. Diverge deploys a preview pod.
The mesh routes tagged traffic to it; everything else hits the baseline.
Zero wasted resources.

```
┌──────────────┐    ┌──────────────┐    ┌──────────────────────┐
│  web-app     │───▶│  gateway     │───▶│ payments-api         │
│  (baseline)  │    │  (baseline)  │    │ ┌──────────────────┐ │
└──────────────┘    └──────────────┘    │ │ baseline (main)  │ │
                                        │ ├──────────────────┤ │
                                        │ │ PREVIEW (MR-42)  │◀── x-preview-id: 42
                                        │ └──────────────────┘ │
                                        └──────────┬───────────┘
                                                   │
                                        ┌──────────▼───────────┐
                                        │ accounts-api         │
                                        │ (baseline)           │
                                        └──────────────────────┘
```

### Prerequisites

- [k3d](https://k3d.io) — lightweight K8s in Docker
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [helm](https://helm.sh/docs/intro/install/)
- [Docker](https://docs.docker.com/get-docker/)
- [jq](https://jqlang.github.io/jq/) (optional)

### Quick Start

```bash
cd bank-demo

# 1. Setup (~2 minutes)
# Creates k3d cluster, installs Envoy Gateway, clones service repos,
# builds images, deploys 4 baseline services
./scripts/setup.sh

# 2. Test baseline
curl -s http://localhost:8080/api/payments | jq
# → {"service": "payments-api", "version": "baseline", ...}

# 3. Simulate an MR on payments-api
./scripts/simulate-preview.sh 42

# 4. Test preview routing
curl -s http://localhost:8080/api/payments | jq '.version'
# → "baseline"

curl -s -H 'x-preview-id: 42' http://localhost:8080/api/payments | jq '.version'
# → "preview-42"

# 5. Run multiple previews simultaneously
./scripts/simulate-preview.sh 99
curl -s -H 'x-preview-id: 99' http://localhost:8080/api/payments | jq '.version'
# → "preview-99"

# 6. Cleanup
./scripts/cleanup-preview.sh 42
./scripts/cleanup-preview.sh 99
./scripts/setup.sh teardown
```

### Service Repos

Each service is an independent GitHub repo with its own `.diverge.yaml`:

| Repo | Role | Calls |
|------|------|-------|
| [demo-web-app](https://github.com/divergedev/demo-web-app) | Frontend dashboard | → gateway |
| [demo-gateway](https://github.com/divergedev/demo-gateway) | API BFF | → payments, accounts |
| [demo-payments-api](https://github.com/divergedev/demo-payments-api) | Payments service | → accounts |
| [demo-accounts-api](https://github.com/divergedev/demo-accounts-api) | Accounts service | (leaf) |

### How It Maps to Production

| Demo | Production |
|------|-----------|
| k3d cluster | GKE / EKS dev cluster |
| Envoy Gateway | Istio Waypoint Proxy |
| `setup.sh` | ArgoCD manages baseline |
| `simulate-preview.sh` | Diverge controller (webhook-driven) |
| `cleanup-preview.sh` | Diverge controller (MR merge) |
| `curl -H x-preview-id` | OTel Baggage propagation |

### What's `.diverge.yaml`?

Each service repo contains a `.diverge.yaml` that tells Diverge how to create preview pods:

```yaml
apiVersion: diverge.io/v1alpha1
kind: ServicePreview
metadata:
  name: payments-api
spec:
  namespace: demo-bank
  serviceName: payments-api
  port: 8080
  routing:
    headerKey: x-preview-id
```

---

## More Demos (Coming Soon)

- **SaaS Demo** — Multi-tenant preview environments
- **E-commerce Demo** — Frontend + backend polyrepo with shared cart service
