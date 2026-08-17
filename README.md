# 🏦 Diverge Bank Demo

> **Multi-repo preview environments in action.** React Module Federation frontend, Go BFF, header-based routing, PreviewGroups — powered by the Diverge controller.

## What This Demo Shows

- **5 microservices**: `web-app` shell, `gateway` BFF, `payments-api`, `accounts-api`, and `payments-module`.
- **React Module Federation**: The `web-app` shell (host) dynamically loads the `payments-module` (remote).
- **PreviewGroup CRD**: Ties together `payments-api` and `payments-module` across repositories to test full-stack features.
- **Header-based routing**: Passing `x-preview-id: 42` hits the preview environment; no header hits the baseline.
- **The "dramatic change"**: A new fraud detection banner and fee column appear **only** in the preview.
- **DevSpace**: For fast in-cluster file synchronization.
- **Air**: For Go hot-reloading.
- **`diverge dev`**: CLI command for local traffic routing.

## Architecture

```
                    Browser
                      │
              ┌───────▼───────┐
              │   web-app     │  React Shell (Host)
              └───────┬───────┘
                      │
              ┌───────▼───────┐
              │   gateway     │  Go BFF + Module Registry
              └───────┬───────┘
                      │
        ┌─────────────┼─────────────┐
        │             │             │
   ┌────▼────┐  ┌─────▼─────┐  ┌───▼──────────┐
   │payments │  │accounts   │  │payments      │
   │-api (Go)│  │-api (Go)  │  │-module(React)│
   └─────────┘  └───────────┘  └──────────────┘

   Preview: payments-api + payments-module replaced
   Baseline: gateway + accounts-api + web-app unchanged
```

## Prerequisites

| Tool | Note |
|------|------|
| **Docker** | Required for building container images. |
| **kubectl** | For interacting with the cluster. |
| **helm** | To install Diverge and dependencies. |
| **Go 1.26+** | Required for the Go backend services. |
| **Node.js 22+** | Required for the React frontend services. |
| **Diverge CLI** | To run `diverge dev` and manage previews. |
| **Air** | Go hot-reload tool for local development. |
| **jq** | For parsing JSON output. |

## Setup (GKE)

Follow these steps to deploy the demo to a GKE cluster:

### 1. Create GKE Autopilot cluster
```bash
nix develop -c gcloud container clusters create-auto diverge-demo \
    --region us-central1 \
    --project my-gcp-project
nix develop -c gcloud container clusters get-credentials diverge-demo \
    --region us-central1 \
    --project my-gcp-project
```

### 2. Install Diverge controller via Helm
```bash
nix develop -c helm repo add diverge https://charts.divergedev.com
nix develop -c helm install diverge diverge/diverge-controller -n diverge-system --create-namespace
```

### 3. Build and push container images
We use `ghcr.io/divergedev` as our container registry.
```bash
for svc in web-app gateway payments-api accounts-api payments-module; do
  nix develop -c docker build -t ghcr.io/divergedev/demo-$svc:baseline ./bank-demo/services/$svc
  nix develop -c docker push ghcr.io/divergedev/demo-$svc:baseline
done
```

### 4. Deploy baseline services
```bash
nix develop -c kubectl apply -k ./bank-demo/manifests/baseline
```

### 5. Verify with curl
Wait for the gateway to get an external IP, then test the baseline endpoints:
```bash
export GATEWAY_IP=$(kubectl get svc gateway -n demo-bank -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
nix develop -c curl -s http://$GATEWAY_IP/api/payments | jq .
```

## Demo Walkthrough

### 1. Verify Baseline

```bash
nix develop -c curl -s http://$GATEWAY_IP/api/payments | jq .
# → {"service": "payments-api", "version": "baseline", ...}

nix develop -c curl -s http://$GATEWAY_IP/api/accounts | jq .
# → {"service": "accounts-api", "version": "baseline", "balance": 10542.87}
```

### 2. Create PreviewGroup

We simulate a developer working on a "fraud detection" feature that requires changes in both the backend API and the frontend module.

```bash
nix develop -c diverge preview create --name fraud-detection \
  --service payments-api=ghcr.io/divergedev/demo-payments-api:preview-42 \
  --service payments-module=ghcr.io/divergedev/demo-payments-module:preview-42 \
  --header-key x-preview-id --header-value 42
```

**Expected output:**
```
🚀 Created PreviewGroup fraud-detection
Routing: x-preview-id: 42
Services:
  - payments-api (ghcr.io/divergedev/demo-payments-api:preview-42)
  - payments-module (ghcr.io/divergedev/demo-payments-module:preview-42)
```

### 3. Header Routing

Test routing with and without the preview header:

```bash
# Baseline
nix develop -c curl -s http://$GATEWAY_IP/api/payments | jq .version
# → "baseline"

# Preview
nix develop -c curl -s -H "x-preview-id: 42" http://$GATEWAY_IP/api/payments | jq .version
# → "preview-42"
```

In the browser, passing the header via a browser extension (or inspecting the network) will reveal the dramatic change: a new fraud detection banner and a transaction fee column.

### 4. Local Dev with `diverge dev` and Air

To work on the `payments-api` locally while routing traffic from the cluster down to your laptop:

```bash
cd bank-demo/services/payments-api
nix develop -c diverge preview dev --service payments-api -- air
```

This intercepts preview traffic for `payments-api` and sends it to your local Air process.

### 5. Local Dev with DevSpace

For the React module, DevSpace provides in-cluster file sync:

```bash
cd bank-demo/services/payments-module
nix develop -c diverge preview dev --service payments-module -- devspace dev
```

Any changes to your React files are instantly synced to the container in the cluster, triggering a Vite HMR update.

### 6. diverge status + logs

Check the status of your previews:
```bash
nix develop -c diverge preview status
```

View logs for your preview environment:
```bash
nix develop -c diverge preview logs fraud-detection
```

### 7. Teardown

Remove the preview environment:
```bash
nix develop -c diverge preview delete fraud-detection
```

### 8. Cleanup

Tear down the entire demo baseline:
```bash
nix develop -c kubectl delete -k ./bank-demo/manifests/baseline
```

## For azraONE Design Partners

| Bank Demo | azraONE | Notes |
|-----------|---------|-------|
| `web-app` (React Shell) | app-shell | Vite Module Federation host |
| `gateway` (Go BFF) | shell-bff | Module registry + API aggregation |
| `payments-module` (React Remote) | rad-ui / product modules | Independently deployable UI |
| `payments-api` (Go) | rad-api / product APIs | Core domain service |
| `accounts-api` (Go) | Other domain APIs | Stays on baseline; unchanged |
| PreviewGroup CR | Cross-repo MR testing | Deploys API + UI under one header |

## Service Repos

| Service | Path / Repo |
|---------|-------------|
| **web-app** | `bank-demo/services/web-app` |
| **gateway** | `bank-demo/services/gateway` |
| **payments-api** | `bank-demo/services/payments-api` |
| **accounts-api** | `bank-demo/services/accounts-api` |
| **payments-module** | `bank-demo/services/payments-module` |

## Troubleshooting

- **`diverge preview: command not found`**: Ensure the Diverge CLI is installed and in your `$PATH`. If using Nix, ensure you ran `nix develop`.
- **Traffic always hits baseline**: Verify that the header `x-preview-id` matches exactly what was configured in the `PreviewGroup`, and that your API clients (or SDKs) are correctly propagating the header down the stack.
- **React Module Federation fails to load remote**: Check the browser console. The `gateway` module registry must properly return the preview URL for `payments-module` when the `x-preview-id` header is present.
