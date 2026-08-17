# Stage 1: Build React
FROM node:22-alpine AS frontend
WORKDIR /frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
ARG APP_VERSION=baseline
ENV VITE_APP_VERSION=${APP_VERSION}
RUN npm run build

# Stage 2: Build Go
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
COPY --from=frontend /frontend/dist ./dist
RUN go mod tidy && CGO_ENABLED=0 go build -o /payments-module .

# Stage 3: Runtime
FROM gcr.io/distroless/static-debian12
COPY --from=builder /payments-module /payments-module
EXPOSE 8080
ENTRYPOINT ["/payments-module"]
