FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN go mod tidy && CGO_ENABLED=0 go build -o /accounts-api .

FROM gcr.io/distroless/static-debian12
COPY --from=build /accounts-api /accounts-api
EXPOSE 8080
ENTRYPOINT ["/accounts-api"]
