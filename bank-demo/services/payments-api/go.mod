module github.com/divergedev/demo-payments-api

go 1.26.4

require github.com/jackc/pgx/v5 v5.10.0

require (
	connectrpc.com/connect v1.20.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/protocgen/proto2mcp v0.13.1 // indirect
	go.opentelemetry.io/otel v1.45.0 // indirect
	go.opentelemetry.io/otel/metric v1.45.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

require (
	github.com/divergedev/diverge v0.15.0
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/text v0.42.0 // indirect
)

replace github.com/divergedev/diverge => ../../../../diverge
