FROM golang:1.26-alpine AS build
ARG APP_VERSION=baseline
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
# Replace version placeholder in JS module at build time
RUN sed -i "s/__APP_VERSION__/${APP_VERSION}/" remoteEntry.js
RUN CGO_ENABLED=0 go build -o /payments-module .

FROM gcr.io/distroless/static-debian12
COPY --from=build /payments-module /payments-module
EXPOSE 8080
ENTRYPOINT ["/payments-module"]
