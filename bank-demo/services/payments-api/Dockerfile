FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /payments-api .

FROM gcr.io/distroless/static-debian12
COPY --from=build /payments-api /payments-api
EXPOSE 8080
ENTRYPOINT ["/payments-api"]
