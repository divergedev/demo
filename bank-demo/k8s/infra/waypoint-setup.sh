#!/usr/bin/env bash
set -euo pipefail
NAMESPACE=${1:-demo-bank}

echo "=== Enabling ambient for $NAMESPACE ==="
kubectl label namespace $NAMESPACE istio.io/dataplane-mode=ambient --overwrite

echo "=== Deploying waypoint ==="
istioctl waypoint apply -n $NAMESPACE --enroll-namespace

echo "=== Patching waypoint for ARM64 ==="
kubectl patch deployment waypoint -n $NAMESPACE --type=json -p='[
  {"op":"add","path":"/spec/template/spec/tolerations","value":[
    {"key":"kubernetes.io/arch","operator":"Equal","value":"arm64","effect":"NoSchedule"}
  ]}
]'

echo "=== Waiting for waypoint ==="
kubectl rollout status deploy/waypoint -n $NAMESPACE --timeout=60s
echo "=== Done ==="
