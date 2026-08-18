#!/usr/bin/env bash
# Build and tag Docker images for the Diverge Bank Demo.
# Run from the repo root: ./scripts/build-images.sh [baseline|preview-42]
#
# For services with a local `replace` directive pointing at the Diverge SDK,
# the script temporarily removes it and regenerates go.sum so Docker can
# fetch the published module from proxy.golang.org.

set -euo pipefail

TAG="${1:-baseline}"
REGISTRY="${REGISTRY:-ghcr.io/divergedev}"
PLATFORM="${PLATFORM:-linux/amd64}"
SERVICES=(gateway payments-api accounts-api web-app payments-module)

echo "🏗  Building images with tag: ${TAG}"
echo "   Registry: ${REGISTRY}"
echo ""

for svc in "${SERVICES[@]}"; do
  dir="bank-demo/services/${svc}"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  Building: ${svc} → ${REGISTRY}/demo-${svc}:${TAG}"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

  RESTORE_GOMOD=0

  # Remove the local replace directive and regenerate go.sum
  if grep -q "replace github.com/divergedev/diverge" "${dir}/go.mod"; then
    cp "${dir}/go.mod" "${dir}/go.mod.bak"
    cp "${dir}/go.sum" "${dir}/go.sum.bak" 2>/dev/null || true
    sed -i.tmp '/replace github.com\/divergedev\/diverge/d' "${dir}/go.mod"
    rm -f "${dir}/go.mod.tmp"
    # Regenerate go.sum for the published module
    (cd "${dir}" && go mod tidy 2>/dev/null || true)
    RESTORE_GOMOD=1
  fi

  docker build \
    --platform "${PLATFORM}" \
    ${NO_CACHE:+--no-cache} \
    --build-arg APP_VERSION="${TAG}" \
    -t "${REGISTRY}/demo-${svc}:${TAG}" \
    "${dir}"

  # Restore original go.mod and go.sum
  if [ "${RESTORE_GOMOD}" = "1" ]; then
    mv "${dir}/go.mod.bak" "${dir}/go.mod"
    if [ -f "${dir}/go.sum.bak" ]; then
      mv "${dir}/go.sum.bak" "${dir}/go.sum"
    fi
  fi

  echo "  ✅ ${svc} built successfully"
  echo ""
done

echo "🎉 All images built!"
echo ""
echo "To push all:"
for svc in "${SERVICES[@]}"; do
  echo "  docker push ${REGISTRY}/demo-${svc}:${TAG}"
done
