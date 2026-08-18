# Diverge Demo Infrastructure

This directory contains the Kubernetes configuration files for the Diverge demo infrastructure. These manifests are designed for a successful Istio Ambient + Waypoint + Tailscale setup on GKE ARM64.

## Prerequisites

Before starting, ensure you have the following:

- GKE Standard cluster with ARM64 nodes
- A Tailscale account and auth key
- `helm` CLI installed
- `istioctl` CLI installed
- GHCR image pull secret in each app namespace (create with `kubectl create secret docker-registry ghcr-creds ...`)

## Setup Order

Follow this order to properly configure the infrastructure:

1. **Tailscale:** Deploy the Tailscale subnet router.
2. **Istio Ambient:** Install Istio in ambient mode and apply the ARM64 patches.
3. **Waypoint:** Set up the waypoint proxy for your namespace.
4. **Local Dev Routing:** (Reference) Examples of routing resources used during local development.

## Quick Start Commands

### 1. Tailscale Setup
Ensure you have created a secret named `tailscale-auth` in the `tailscale` namespace containing your `TS_AUTHKEY`. Then apply the daemonset:
```bash
kubectl apply -f tailscale-daemonset.yaml
```

### 2. Istio Ambient Patches
After running `istioctl install --set profile=ambient -y`, run the patch script:
```bash
./istio-ambient-patches.sh
```

### 3. Waypoint Setup
Deploy the waypoint proxy for the target namespace (e.g., `demo-bank`):
```bash
./waypoint-setup.sh demo-bank
```

## Detailed Documentation
*(Link to detailed docs artifact here when available)*

## KNative Serverless Previews

### Install KNative + Kourier

```bash
./knative-install.sh
```

### Deploy baseline services

```bash
# Copy GHCR credentials to demo-knative namespace
kubectl get secret ghcr-creds -n demo-bank -o json | \
  python3 -c "import json,sys; d=json.load(sys.stdin); d['metadata']={'name':'ghcr-creds','namespace':'demo-knative'}; print(json.dumps(d))" | \
  kubectl apply -f -

# Deploy KNative services
kubectl apply -f ../knative/baseline-services.yaml
```

### Key configuration for ARM64

- `kubernetes.podspec-tolerations` must be enabled in `config-features` ConfigMap
- All KNative system deployments need ARM64 tolerations
- Kourier gateway in `kourier-system` also needs ARM64 toleration

