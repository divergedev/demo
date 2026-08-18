#!/usr/bin/env bash
set -euo pipefail

echo "=== Installing KNative Serving CRDs ==="
kubectl apply -f https://github.com/knative/serving/releases/latest/download/serving-crds.yaml

echo "=== Installing KNative Serving Core ==="
kubectl apply -f https://github.com/knative/serving/releases/latest/download/serving-core.yaml

echo "=== Installing Kourier ==="
kubectl apply -f https://github.com/knative/net-kourier/releases/latest/download/kourier.yaml

echo "=== Configuring Kourier as default ==="
kubectl patch configmap/config-network -n knative-serving \
  --type merge -p '{"data":{"ingress-class":"kourier.ingress.networking.knative.dev"}}'

echo "=== Enabling podspec-tolerations for ARM64 ==="
kubectl patch configmap/config-features -n knative-serving \
  --type merge -p '{"data":{"kubernetes.podspec-tolerations":"enabled"}}'

echo "=== Patching KNative deployments for ARM64 ==="
for deploy in activator autoscaler controller webhook net-kourier-controller; do
  kubectl patch deployment $deploy -n knative-serving --type=json -p='[
    {"op":"add","path":"/spec/template/spec/tolerations","value":[
      {"key":"kubernetes.io/arch","operator":"Equal","value":"arm64","effect":"NoSchedule"}
    ]}
  ]'
done

kubectl patch deployment 3scale-kourier-gateway -n kourier-system --type=json -p='[
  {"op":"add","path":"/spec/template/spec/tolerations","value":[
    {"key":"kubernetes.io/arch","operator":"Equal","value":"arm64","effect":"NoSchedule"}
  ]}
]'

echo "=== Waiting for KNative pods ==="
kubectl rollout status deploy/activator -n knative-serving --timeout=120s
kubectl rollout status deploy/controller -n knative-serving --timeout=120s
kubectl rollout status deploy/webhook -n knative-serving --timeout=120s
kubectl rollout status deploy/3scale-kourier-gateway -n kourier-system --timeout=120s

echo "=== Creating demo-knative namespace ==="
kubectl create namespace demo-knative 2>/dev/null || true

echo "=== Done! ==="
kubectl get pods -n knative-serving
kubectl get pods -n kourier-system
