# Demo Gateway

API gateway for the bank demo.

## Proxy Routes

| Route | Target |
| ----- | ------ |
| `/api/payments` | `PAYMENTS_API_URL/api/payments` |
| `/api/payments/transactions` | `PAYMENTS_API_URL/api/payments/transactions` |
| `/api/accounts` | `ACCOUNTS_API_URL/api/accounts/balance` |

## Header Forwarding

The gateway automatically forwards the `x-preview-id` header to downstream services to ensure proper routing in preview environments.

## Environment Variables

| Variable | Description | Default |
| -------- | ----------- | ------- |
| `PORT` | Port to listen on | `8080` |
| `APP_VERSION` | Version of the application | `baseline` |
| `PAYMENTS_API_URL` | URL of the Payments API | `http://payments-api:8080` |
| `ACCOUNTS_API_URL` | URL of the Accounts API | `http://accounts-api:8080` |

## Building the Image

```sh
docker build -t divergedev/demo-gateway .
```
