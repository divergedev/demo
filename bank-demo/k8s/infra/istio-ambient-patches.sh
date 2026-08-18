#!/usr/bin/env bash
# Patches for Istio Ambient on GKE ARM64
# Run after: istioctl install --set profile=ambient -y
set -euo pipefail

echo "=== Patching istiod for ARM64 ==="
kubectl patch deployment istiod -n istio-system --type=json -p='[
  {"op":"add","path":"/spec/template/spec/tolerations","value":[
    {"key":"kubernetes.io/arch","operator":"Equal","value":"arm64","effect":"NoSchedule"}
  ]}
]'

echo "=== Installing istio-cni with GKE platform ==="
helm repo add istio https://istio-release.storage.googleapis.com/charts 2>/dev/null || true
helm upgrade --install istio-cni istio/cni -n istio-system \
  --set global.platform=gke \
  --set cni.ambient.enabled=true

echo "=== Installing ztunnel ==="
helm upgrade --install ztunnel istio/ztunnel -n istio-system

echo "=== Patching ztunnel for hostNetwork + ARM64 ==="
kubectl patch ds ztunnel -n istio-system --type=json -p='[
  {"op":"add","path":"/spec/template/spec/hostNetwork","value":true},
  {"op":"add","path":"/spec/template/spec/dnsPolicy","value":"ClusterFirstWithHostNet"},
  {"op":"add","path":"/spec/template/spec/tolerations","value":[
    {"key":"kubernetes.io/arch","operator":"Equal","value":"arm64","effect":"NoSchedule"}
  ]}
]'

echo "=== Setting outbound traffic policy ==="
kubectl patch configmap istio -n istio-system --type=merge -p='{
  "data": {
    "mesh": "outboundTrafficPolicy:\n  mode: ALLOW_ANY\n"
  }
}'

echo "=== Done! Verify ==="
kubectl get pods -n istio-system
