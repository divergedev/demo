#!/usr/bin/env bash
# Automated smoke test for the Diverge Bank Demo.
# Verifies the full lifecycle: baseline → preview → routing → cleanup.
#
# Usage: ./verify.sh

set -euo pipefail

DEMO_NS="demo-bank"
DIVERGE_NS="diverge-system"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

PASS=0
FAIL=0

check() {
    local desc="$1"
    shift
    if "$@" >/dev/null 2>&1; then
        echo -e "  ${GREEN}✓${NC} ${desc}"
        PASS=$((PASS + 1))
    else
        echo -e "  ${RED}✗${NC} ${desc}"
        FAIL=$((FAIL + 1))
    fi
}

check_output() {
    local desc="$1"
    local expected="$2"
    shift 2
    local actual
    actual=$("$@" 2>/dev/null || echo "ERROR")
    if echo "$actual" | grep -q "$expected"; then
        echo -e "  ${GREEN}✓${NC} ${desc} ${BLUE}(${expected})${NC}"
        PASS=$((PASS + 1))
    else
        echo -e "  ${RED}✗${NC} ${desc} — expected '${expected}', got '${actual}'"
        FAIL=$((FAIL + 1))
    fi
}

echo ""
echo -e "${BOLD}${CYAN}🔍 Diverge Bank Demo — Smoke Test${NC}"
echo ""

# ─── Infrastructure ──────────────────────────────────────
echo -e "${YELLOW}Infrastructure:${NC}"
check "k3d cluster exists" k3d cluster list -o json
check "Envoy Gateway running" kubectl get deployment -n envoy-gateway-system envoy-gateway
check "Diverge controller running" kubectl wait --for=condition=Available deployment -l app.kubernetes.io/name=diverge -n "$DIVERGE_NS" --timeout=5s
check "Environment CRD registered" kubectl get crd environments.diverge.io
check "Gateway exists" kubectl get gateway demo-gateway -n "$DEMO_NS"
echo ""

# ─── Baseline services ───────────────────────────────────
echo -e "${YELLOW}Baseline services:${NC}"
check "accounts-api pod running" kubectl wait --for=condition=Ready pod -l app=accounts-api -n "$DEMO_NS" --timeout=5s
check "payments-api pod running" kubectl wait --for=condition=Ready pod -l app=payments-api -n "$DEMO_NS" --timeout=5s
check "gateway pod running" kubectl wait --for=condition=Ready pod -l app=gateway -n "$DEMO_NS" --timeout=5s
check "web-app pod running" kubectl wait --for=condition=Ready pod -l app=web-app -n "$DEMO_NS" --timeout=5s
check "baseline HTTPRoute exists" kubectl get httproute baseline-routes -n "$DEMO_NS"
echo ""

# ─── Baseline routing ────────────────────────────────────
echo -e "${YELLOW}Baseline routing:${NC}"
check_output "GET /api/payments returns baseline" "baseline" \
    curl -s http://localhost:8080/api/payments
check_output "GET /api/accounts returns baseline" "baseline" \
    curl -s http://localhost:8080/api/accounts
echo ""

# ─── Preview lifecycle ────────────────────────────────────
echo -e "${YELLOW}Preview lifecycle:${NC}"

# Create preview
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
echo -e "  ${BLUE}▸${NC} Creating preview environment 99..."
"${SCRIPT_DIR}/preview.sh" 99 >/dev/null 2>&1 || true
sleep 5

check "Environment CR exists" kubectl get environment preview-99 -n "$DEMO_NS"
check "Preview pod created" kubectl get pods -n "$DEMO_NS" -l "diverge.io/environment=preview-99" --no-headers

# Test routing
check_output "Baseline (no header) still returns baseline" "baseline" \
    curl -s http://localhost:8080/api/payments
check_output "Preview (with header) returns preview-99" "preview-99" \
    curl -s -H "x-preview-id: 99" http://localhost:8080/api/payments

# Cleanup
echo -e "  ${BLUE}▸${NC} Cleaning up preview 99..."
"${SCRIPT_DIR}/cleanup.sh" 99 >/dev/null 2>&1 || true
sleep 5

REMAINING=$(kubectl get pods -n "$DEMO_NS" -l "diverge.io/environment=preview-99" --no-headers 2>/dev/null | wc -l | tr -d ' ')
if [[ "$REMAINING" == "0" ]]; then
    echo -e "  ${GREEN}✓${NC} Preview pod cleaned up"
    PASS=$((PASS + 1))
else
    echo -e "  ${RED}✗${NC} Preview pod still exists after cleanup"
    FAIL=$((FAIL + 1))
fi

check_output "Baseline still works after cleanup" "baseline" \
    curl -s http://localhost:8080/api/payments
echo ""

# ─── Summary ─────────────────────────────────────────────
TOTAL=$((PASS + FAIL))
echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
if [[ $FAIL -eq 0 ]]; then
    echo -e "${BOLD}${GREEN}  All ${TOTAL} checks passed ✅${NC}"
else
    echo -e "${BOLD}${RED}  ${FAIL}/${TOTAL} checks failed ❌${NC}"
fi
echo ""

exit $FAIL
