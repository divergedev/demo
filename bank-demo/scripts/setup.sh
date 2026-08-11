#!/usr/bin/env bash
# Diverge Bank Demo — Full Setup Script
# Creates a k3d cluster with Envoy Gateway, deploys the Diverge controller,
# installs baseline bank services, and prepares for preview environments.
#
# Usage: ./setup.sh
# Teardown: ./setup.sh teardown
#
# Prerequisites: docker, k3d, kubectl, helm

set -euo pipefail

CLUSTER_NAME="diverge-demo"
DEMO_NS="demo-bank"
DIVERGE_NS="diverge-system"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEMO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
SERVICES_DIR="${DEMO_DIR}/.services"
DIVERGE_DIR="${DIVERGE_DIR:-}"

# Auto-discover Diverge source: check sibling dirs, then parent dirs
if [[ -z "$DIVERGE_DIR" ]]; then
    for candidate in "${DEMO_DIR}/../../diverge" "${DEMO_DIR}/../diverge" "${DEMO_DIR}/../../divergedev/diverge"; do
        if [[ -d "$candidate/charts/diverge" ]]; then
            DIVERGE_DIR="$(cd "$candidate" && pwd)"
            break
        fi
    done
fi

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

step() { echo -e "\n${CYAN}━━━ $* ━━━${NC}"; }
log()  { echo -e "  ${BLUE}▸${NC} $*"; }
ok()   { echo -e "  ${GREEN}✅${NC} $*"; }
warn() { echo -e "  ${YELLOW}⚠️${NC} $*"; }
err()  { echo -e "  ${RED}❌${NC} $*"; exit 1; }

# ─── Teardown ──────────────────────────────────────────────
if [[ "${1:-}" == "teardown" ]]; then
    step "Tearing down demo"
    k3d cluster delete "$CLUSTER_NAME" 2>/dev/null || true
    ok "Cluster deleted"
    echo ""
    exit 0
fi

# ─── Banner ────────────────────────────────────────────────
echo ""
echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}${CYAN}║                                                      ║${NC}"
echo -e "${BOLD}${CYAN}║    🏦  Diverge Bank Demo                             ║${NC}"
echo -e "${BOLD}${CYAN}║    Multi-repo preview environments in 5 minutes      ║${NC}"
echo -e "${BOLD}${CYAN}║                                                      ║${NC}"
echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════════════════╝${NC}"
echo ""

# ─── Prerequisites ─────────────────────────────────────────
step "Step 1/9 · Checking prerequisites"
command -v k3d    >/dev/null || err "k3d not found. Install: https://k3d.io"
command -v kubectl >/dev/null || err "kubectl not found"
command -v helm    >/dev/null || err "helm not found"
command -v docker  >/dev/null || err "docker not found"
ok "All tools found"

# ─── Clone service repos ──────────────────────────────────
step "Step 2/9 · Fetching bank microservices"
SERVICES=(demo-payments-api demo-accounts-api demo-gateway demo-web-app)

mkdir -p "$SERVICES_DIR"
for svc in "${SERVICES[@]}"; do
    if [[ -d "${SERVICES_DIR}/${svc}" ]]; then
        log "${svc} (cached, pulling latest)"
        git -C "${SERVICES_DIR}/${svc}" pull --ff-only 2>/dev/null || true
    else
        log "Cloning ${svc}..."
        git clone "https://github.com/divergedev/${svc}.git" "${SERVICES_DIR}/${svc}" --depth 1 2>/dev/null
    fi
done
ok "4 services ready"

# ─── Create k3d cluster ──────────────────────────────────
step "Step 3/9 · Creating k3d cluster"
if k3d cluster list 2>/dev/null | grep -q "$CLUSTER_NAME"; then
    warn "Cluster '$CLUSTER_NAME' already exists, reusing"
else
    log "Creating cluster with Envoy Gateway ports..."
    k3d cluster create "$CLUSTER_NAME" \
        --port "8080:80@loadbalancer" \
        --port "8443:443@loadbalancer" \
        --k3s-arg "--disable=traefik@server:0" \
        --wait
    ok "k3d cluster created"
fi
kubectl config use-context "k3d-${CLUSTER_NAME}" >/dev/null 2>&1

# ─── Install Gateway API + Envoy Gateway ──────────────────
step "Step 4/9 · Installing Envoy Gateway"
log "Installing Gateway API CRDs..."
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.1/standard-install.yaml 2>/dev/null
log "Installing Envoy Gateway via Helm..."
helm upgrade --install eg oci://docker.io/envoyproxy/gateway-helm \
    --version v1.2.4 \
    --namespace envoy-gateway-system \
    --create-namespace \
    --wait 2>/dev/null
ok "Envoy Gateway installed"

# ─── Install Diverge Controller ───────────────────────────
step "Step 5/9 · Installing Diverge controller"

# Build controller image
if [[ -n "$DIVERGE_DIR" && -d "$DIVERGE_DIR" ]]; then
    log "Building controller image from local source..."
    docker build -t divergedev/diverge-controller:dev "$DIVERGE_DIR" --quiet
    k3d image import divergedev/diverge-controller:dev -c "$CLUSTER_NAME"
    ok "Controller image built and loaded"
else
    warn "Diverge source not found locally, using published image"
fi

# Install CRDs
log "Installing Diverge CRDs..."
if [[ -n "$DIVERGE_DIR" && -d "${DIVERGE_DIR}/config/crd/bases" ]]; then
    kubectl apply -f "${DIVERGE_DIR}/config/crd/bases/" 2>/dev/null
else
    kubectl apply -f "https://raw.githubusercontent.com/divergedev/diverge/main/config/crd/bases/diverge.io_environments.yaml" 2>/dev/null
fi
ok "CRDs installed"

# Deploy controller via Helm
log "Deploying Diverge controller..."
HELM_ARGS=(
    --set deploy.provider=direct
    --set deploy.manifestSourceType=serviceconfig
    --set routingProvider=gateway
    --set "defaultNamespace=${DEMO_NS}"
    --set podDisruptionBudget.enabled=false
    --set database.provider=schema
    --set database.host=postgres.demo-bank.svc.cluster.local
    --set database.port=5432
    --set database.user=postgres
    --set database.name=diverge_preview
    --set database.sslMode=disable
    --set database.passwordSecretName=demo-postgres-credentials
    --set database.passwordSecretKey=password
    --namespace "$DIVERGE_NS"
    --create-namespace
    --wait
)

# Use local image if built, otherwise pull from registry
if [[ -n "$DIVERGE_DIR" && -d "$DIVERGE_DIR" ]]; then
    HELM_ARGS+=(--set image.repository=divergedev/diverge-controller --set image.tag=dev --set image.pullPolicy=Never)
    CHART_PATH="${DIVERGE_DIR}/charts/diverge"
else
    CHART_PATH="oci://ghcr.io/divergedev/diverge-helm"
fi

if ! helm upgrade --install diverge "$CHART_PATH" "${HELM_ARGS[@]}" 2>/dev/null; then
    err "Failed to deploy Diverge controller via Helm"
fi
ok "Diverge controller running"

# ─── Create demo namespace + Gateway ──────────────────────
step "Step 6/9 · Setting up demo namespace"
kubectl create namespace "$DEMO_NS" --dry-run=client -o yaml | kubectl apply -f - 2>/dev/null

kubectl apply -f - <<'EOF'
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: diverge-demo
spec:
  controllerName: gateway.envoyproxy.io/gatewayclass-controller
---
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: demo-gateway
  namespace: demo-bank
spec:
  gatewayClassName: diverge-demo
  listeners:
    - name: http
      protocol: HTTP
      port: 80
EOF
ok "Gateway created"

# ─── Deploy PostgreSQL ────────────────────────────────────
step "Step 7/9 · Deploying PostgreSQL"
log "Creating PostgreSQL StatefulSet..."
kubectl apply -n "$DEMO_NS" -f - <<'EOF'
apiVersion: v1
kind: Secret
metadata:
  name: demo-postgres-credentials
stringData:
  password: "diverge-demo-pass"
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres
  labels:
    app: postgres
spec:
  serviceName: postgres
  replicas: 1
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
        - name: postgres
          image: postgres:16-alpine
          ports:
            - containerPort: 5432
          env:
            - name: POSTGRES_DB
              value: diverge_preview
            - name: POSTGRES_USER
              value: postgres
            - name: POSTGRES_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: demo-postgres-credentials
                  key: password
          readinessProbe:
            exec:
              command: ["pg_isready", "-U", "postgres"]
            initialDelaySeconds: 5
            periodSeconds: 5
          volumeMounts:
            - name: pgdata
              mountPath: /var/lib/postgresql/data
  volumeClaimTemplates:
    - metadata:
        name: pgdata
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 1Gi
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
spec:
  selector:
    app: postgres
  ports:
    - port: 5432
      targetPort: 5432
EOF

log "Waiting for PostgreSQL to be ready..."
kubectl wait --for=condition=Ready pod -l app=postgres -n "$DEMO_NS" --timeout=60s 2>/dev/null || err "PostgreSQL did not become ready"
ok "PostgreSQL running"

# Seed baseline data
log "Seeding baseline transactions table..."
kubectl exec -n "$DEMO_NS" postgres-0 -- psql -U postgres -d diverge_preview -c "
CREATE TABLE IF NOT EXISTS transactions (
    id SERIAL PRIMARY KEY,
    from_account VARCHAR(20) NOT NULL,
    to_account VARCHAR(20) NOT NULL,
    amount DECIMAL(10,2) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
INSERT INTO transactions (from_account, to_account, amount) VALUES
    ('ACC-001', 'ACC-002', 150.00),
    ('ACC-003', 'ACC-001', 75.50),
    ('ACC-002', 'ACC-004', 200.00),
    ('ACC-005', 'ACC-001', 50.00),
    ('ACC-001', 'ACC-003', 300.00),
    ('ACC-004', 'ACC-002', 125.75),
    ('ACC-002', 'ACC-005', 89.99),
    ('ACC-003', 'ACC-004', 175.00),
    ('ACC-001', 'ACC-005', 250.00),
    ('ACC-005', 'ACC-003', 60.25)
ON CONFLICT DO NOTHING;
" 2>/dev/null
ok "10 baseline transactions seeded"

# ─── Build and deploy baseline services ───────────────────
step "Step 8/9 · Building and deploying baseline services"

for svc in "${SERVICES[@]}"; do
    SVC_DIR="${SERVICES_DIR}/${svc}"
    IMAGE="divergedev/${svc}:baseline"
    log "Building ${svc}..."
    docker build -t "$IMAGE" "$SVC_DIR" --quiet
    k3d image import "$IMAGE" -c "$CLUSTER_NAME"
done
ok "Images built and loaded"

log "Deploying baseline pods..."
kubectl apply -n "$DEMO_NS" -f - <<'EOF'
# ── Accounts API ──
apiVersion: apps/v1
kind: Deployment
metadata:
  name: accounts-api
  labels:
    app: accounts-api
    diverge.io/role: baseline
spec:
  replicas: 1
  selector:
    matchLabels:
      app: accounts-api
  template:
    metadata:
      labels:
        app: accounts-api
        diverge.io/role: baseline
    spec:
      containers:
        - name: accounts-api
          image: divergedev/demo-accounts-api:baseline
          imagePullPolicy: Never
          ports:
            - containerPort: 8080
          env:
            - name: APP_VERSION
              value: baseline
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: accounts-api
spec:
  selector:
    app: accounts-api
  ports:
    - port: 8080
      targetPort: 8080
---
# ── Payments API ──
apiVersion: apps/v1
kind: Deployment
metadata:
  name: payments-api
  labels:
    app: payments-api
    diverge.io/role: baseline
spec:
  replicas: 1
  selector:
    matchLabels:
      app: payments-api
  template:
    metadata:
      labels:
        app: payments-api
        diverge.io/role: baseline
    spec:
      containers:
        - name: payments-api
          image: divergedev/demo-payments-api:baseline
          imagePullPolicy: Never
          ports:
            - containerPort: 8080
          env:
            - name: APP_VERSION
              value: baseline
            - name: ACCOUNTS_API_URL
              value: http://accounts-api:8080
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: payments-api
spec:
  selector:
    app: payments-api
  ports:
    - port: 8080
      targetPort: 8080
---
# ── Gateway (BFF) ──
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gateway
  labels:
    app: gateway
    diverge.io/role: baseline
spec:
  replicas: 1
  selector:
    matchLabels:
      app: gateway
  template:
    metadata:
      labels:
        app: gateway
        diverge.io/role: baseline
    spec:
      containers:
        - name: gateway
          image: divergedev/demo-gateway:baseline
          imagePullPolicy: Never
          ports:
            - containerPort: 8080
          env:
            - name: APP_VERSION
              value: baseline
            - name: PAYMENTS_API_URL
              value: http://payments-api:8080
            - name: ACCOUNTS_API_URL
              value: http://accounts-api:8080
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: gateway
spec:
  selector:
    app: gateway
  ports:
    - port: 8080
      targetPort: 8080
---
# ── Web App ──
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-app
  labels:
    app: web-app
    diverge.io/role: baseline
spec:
  replicas: 1
  selector:
    matchLabels:
      app: web-app
  template:
    metadata:
      labels:
        app: web-app
        diverge.io/role: baseline
    spec:
      containers:
        - name: web-app
          image: divergedev/demo-web-app:baseline
          imagePullPolicy: Never
          ports:
            - containerPort: 8080
          env:
            - name: APP_VERSION
              value: baseline
            - name: GATEWAY_URL
              value: http://gateway:8080
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: web-app
spec:
  selector:
    app: web-app
  ports:
    - port: 8080
      targetPort: 8080
EOF

# ─── Create baseline HTTPRoutes ───────────────────────────
kubectl apply -n "$DEMO_NS" -f - <<'EOF'
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: baseline-routes
spec:
  parentRefs:
    - name: demo-gateway
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /api/payments
      backendRefs:
        - name: gateway
          port: 8080
    - matches:
        - path:
            type: PathPrefix
            value: /api/accounts
      backendRefs:
        - name: gateway
          port: 8080
    - matches:
        - path:
            type: PathPrefix
            value: /
      backendRefs:
        - name: web-app
          port: 8080
EOF
ok "Baseline services deployed"

# ─── Wait for everything ─────────────────────────────────
step "Step 9/9 · Waiting for pods"
log "Waiting for baseline pods (timeout: 120s)..."
if ! kubectl wait --for=condition=Ready pod -l diverge.io/role=baseline \
    -n "$DEMO_NS" --timeout=120s 2>/dev/null; then
    kubectl get pods -n "$DEMO_NS" 2>/dev/null || true
    err "Baseline pods did not become Ready within 120s"
fi
log "Waiting for controller (timeout: 60s)..."
if ! kubectl wait --for=condition=Available deployment -l app.kubernetes.io/name=diverge \
    -n "$DIVERGE_NS" --timeout=60s 2>/dev/null; then
    kubectl get pods -n "$DIVERGE_NS" 2>/dev/null || true
    err "Diverge controller did not become Available within 60s"
fi
ok "All pods ready"

# ─── Summary ──────────────────────────────────────────────
echo ""
echo -e "${BOLD}${GREEN}╔══════════════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}${GREEN}║                                                      ║${NC}"
echo -e "${BOLD}${GREEN}║    ✅  Diverge Bank Demo — Ready!                     ║${NC}"
echo -e "${BOLD}${GREEN}║                                                      ║${NC}"
echo -e "${BOLD}${GREEN}╚══════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "  ${BOLD}Baseline services:${NC}"
echo -e "    ${CYAN}Payments API${NC}  →  http://localhost:8080/api/payments"
echo -e "    ${CYAN}Accounts API${NC}  →  http://localhost:8080/api/accounts"
echo -e "    ${CYAN}Web App${NC}       →  http://localhost:8080/"
echo ""
echo -e "  ${BOLD}Infrastructure:${NC}"
echo -e "    ${CYAN}Diverge controller${NC}  running in ${YELLOW}${DIVERGE_NS}${NC}"
echo -e "    ${CYAN}Envoy Gateway${NC}       running in ${YELLOW}envoy-gateway-system${NC}"
echo ""
echo -e "  ${BOLD}${YELLOW}Next: Create a preview environment${NC}"
echo -e "    ${BOLD}./scripts/preview.sh 42${NC}"
echo ""
echo -e "  ${BOLD}Teardown:${NC}"
echo -e "    ./scripts/setup.sh teardown"
echo ""
