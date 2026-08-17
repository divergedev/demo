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
COPY go.mod ./
RUN go mod download
COPY . .
COPY --from=frontend /frontend/dist ./dist
RUN CGO_ENABLED=0 go build -o /web-app .

# Stage 3: Runtime
FROM gcr.io/distroless/static-debian12
COPY --from=builder /web-app /web-app
EXPOSE 8080
ENTRYPOINT ["/web-app"]
