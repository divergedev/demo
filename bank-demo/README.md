# 🏦 Diverge Bank Demo

A fully self-contained demo showcasing [Diverge](https://diverge.dev) — preview environments for multi-repo microservice architectures.

**What you'll see:**
- 🔀 **Header-based routing** — A single header (`x-preview-id`) routes traffic to a preview version of one service while all other services stay on baseline
- 🗄️ **Database schema isolation** — Preview environments get their own Postgres schema with safe, isolated migrations
- 🧩 **Multi-repo support** — Each microservice lives in its own repo, just like production

## Architecture

```
                ┌──────────────────────────────────────┐
                │         Envoy Gateway                │
                │   (header-based routing)             │
                └──────────┬───────────────────────────┘
                           │
              ┌────────────┴────────────┐
              │                         │
    no header / other headers    x-preview-id: 42
              │                         │
              ▼                         ▼
    ┌──────────────────┐    ┌──────────────────────────┐
    │  gateway (baseline)│    │  gateway (baseline)       │
    └────────┬─────────┘    └────────┬─────────────────┘
             │                       │
    ┌────────┼────────┐    ┌────────┼────────┐
    │        │        │    │        │        │
    ▼        ▼        ▼    ▼        ▼        ▼
 payments accounts  web  payments accounts  web
 (v1)     (v1)     (v1) (preview) (v1)     (v1)
    │                       │
    ▼                       ▼
 public                  preview_42
 schema                  schema
 (no fee)                (+ fee column)
```

### Services

| Service | Repo | Port | Description |
|---------|------|------|-------------|
| `gateway` | [demo-gateway](https://github.com/divergedev/demo-gateway) | 8080 | API gateway, proxies to backend services |
| `payments-api` | [demo-payments-api](https://github.com/divergedev/demo-payments-api) | 8080 | Payments service with Postgres (the service we preview) |
| `accounts-api` | built-in | 8080 | Accounts service (stays on baseline) |
| `web-app` | built-in | 8080 | Frontend placeholder |

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [k3d](https://k3d.io/) (v5+)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Helm](https://helm.sh/docs/intro/install/) (v3+)
- The [Diverge controller source](https://github.com/divergedev/diverge) cloned as a sibling directory

### Expected directory layout

```
divergedev/
├── diverge/          # Diverge controller source
└── demo/
    └── bank-demo/    # ← you are here
```

## Quick Start

### 1. Set up the baseline environment

```bash
./scripts/setup.sh
```

This creates a k3d cluster, installs Envoy Gateway + Diverge controller, deploys all baseline services, and seeds the Postgres database. Takes ~2 minutes.

### 2. Create a preview environment

```bash
./scripts/preview.sh 42
```

This builds a preview version of `payments-api`, creates a Diverge `Environment` CR, and the controller automatically:
- Deploys a preview pod with isolated DB schema
- Runs Atlas migrations (creates `transactions` table + `fee` column)
- Creates an HTTPRoute that matches `x-preview-id: 42`

### 3. Test it!

```bash
# Baseline — no header, hits baseline payments-api
curl -s http://localhost:8080/api/payments | jq .version
# → "baseline"

# Preview — header routes to preview payments-api
curl -s -H 'x-preview-id: 42' http://localhost:8080/api/payments | jq .version
# → "preview-42"

# Other services are unaffected
curl -s -H 'x-preview-id: 42' http://localhost:8080/api/accounts | jq .version
# → "baseline"

# Database isolation — preview has fee column
curl -s -H 'x-preview-id: 42' http://localhost:8080/api/payments/transactions | jq '.transactions[0]'
# → { "id": 1, "from": "ACC-001", "to": "ACC-002", "amount": 150, "fee": 2.25 }

# Baseline doesn't have fee column
curl -s http://localhost:8080/api/payments/transactions | jq '.transactions[0]'
# → { "id": 1, "from": "ACC-001", "to": "ACC-002", "amount": 150, "fee": null }
```

### 4. Inspect the environment

```bash
kubectl get environment -n demo-bank
# NAME         PHASE     URL
# preview-42   Running

kubectl get pods -n demo-bank
# payments-api-xxx                (baseline)
# preview-42-payments-api-xxx     (preview)
# accounts-api-xxx                (baseline)
# gateway-xxx                     (baseline)

kubectl get httproute -n demo-bank
# baseline-routes                 (all services)
# preview-42-payments-api         (header-matched preview route)
```

### 5. Teardown

```bash
# Remove just the preview
./scripts/cleanup.sh 42

# Tear down everything (cluster + all resources)
./scripts/setup.sh teardown
```

## How It Works

### Preview Flow

1. `preview.sh` builds a Docker image tagged `preview-42` and loads it into k3d
2. It creates a Diverge `Environment` custom resource with:
   - The preview image to deploy
   - Database migration config (Atlas image + args)
   - Service routing config (pathPrefix, headerKey)
3. The **Diverge controller** reconciles the Environment:
   - Creates a Postgres schema `diverge_env_preview_42_*`
   - Runs the Atlas migration Job (creates tables + adds `fee` column)
   - Deploys a preview Deployment + Service
   - Creates an HTTPRoute matching `x-preview-id: 42` → preview Service
4. Envoy Gateway routes traffic based on the header

### Database Schema Isolation

Each preview environment gets its own Postgres schema:

```
diverge_preview (database)
├── public (baseline schema)
│   └── transactions (id, from, to, amount)
└── diverge_env_preview_42_f871a334 (preview schema)
    └── transactions (id, from, to, amount, fee ← new!)
```

- **Baseline** connects with default `search_path=public` — no `fee` column visible
- **Preview** connects with `search_path=diverge_env_preview_42_*` — sees the migrated schema with `fee`
- The application detects the `fee` column via `information_schema.columns` scoped to `current_schema()`

### Atlas Migrations

Migrations live in `migrations/`:

| File | Description |
|------|-------------|
| `20260811000001_baseline.sql` | Creates `transactions` table + seeds 10 rows |
| `20260811000002_add_fee.sql` | Adds `fee` column + backfills with 1.5% of amount |
| `atlas.sum` | Integrity checksum (regenerate with `atlas migrate hash`) |

## Configuration

### Environment CR (what `preview.sh` creates)

```yaml
apiVersion: diverge.io/v1alpha1
kind: Environment
metadata:
  name: preview-42
  namespace: demo-bank
spec:
  serviceConfig:
    serviceName: payments-api
    image: divergedev/demo-payments-api:preview-42
    port: 8080
    pathPrefix: /api/payments
    headerKey: x-preview-id
    env:
      - name: APP_VERSION
        value: "preview-42"
  database:
    mode: schema
    migrationJob:
      image: divergedev/demo-migrations:latest
      args: ["migrate", "apply", "--url", "$(DATABASE_URL)", "--dir", "file:///migrations"]
```

## Regenerating Migration Checksums

If you modify the SQL migration files, regenerate the Atlas checksum:

```bash
docker run --rm -v $(pwd)/migrations:/migrations \
  arigaio/atlas:latest migrate hash --dir file:///migrations
```

Then rebuild the migration image:

```bash
docker build -t divergedev/demo-migrations:latest migrations/
```

## License

See the [Diverge repository](https://github.com/divergedev/diverge) for license details.
