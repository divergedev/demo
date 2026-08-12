FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /web-app .

FROM gcr.io/distroless/static-debian12
COPY --from=build /web-app /web-app
EXPOSE 8080
ENTRYPOINT ["/web-app"]
