# Demo Payments API

Demo payments microservice for Diverge previews.

## Endpoints

| Endpoint | Description |
| -------- | ----------- |
| `/health` | Health check endpoint returning status and version |
| `/api/payments` | Returns a list of payments and fetches account balance via the Accounts API |
| `/api/payments/transactions` | Returns a list of transactions, optionally with database integration |

## Environment Variables

| Variable | Description | Default |
| -------- | ----------- | ------- |
| `PORT` | Port to listen on | `8080` |
| `APP_VERSION` | Version of the application | `baseline` |
| `ACCOUNTS_API_URL` | URL of the Accounts API | `http://accounts-api:8080` |
| `DATABASE_URL` | PostgreSQL connection URL | (unset) |

## Database Integration

The service uses the `pgx/v5` driver to connect to PostgreSQL. The `/api/payments/transactions` endpoint features schema-aware column detection by checking the `information_schema` for a `fee` column. This demonstrates database schema isolation in previews. If the `DATABASE_URL` is unset, the service gracefully falls back to returning static mock data.

## Building the Image

```sh
docker build -t divergedev/demo-payments-api .
```
