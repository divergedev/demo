FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN go mod tidy && CGO_ENABLED=0 go build -o /payments-api .

FROM gcr.io/distroless/static-debian12
LABEL org.opencontainers.image.source=https://github.com/divergedev/demo
COPY --from=build /payments-api /payments-api
EXPOSE 8080
ENTRYPOINT ["/payments-api"]
