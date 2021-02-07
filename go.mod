module github.com/schmurfy/sniffit

go 1.14

require (
	github.com/cenkalti/backoff/v4 v4.1.0
	github.com/dgraph-io/badger/v3 v3.2011.1
	github.com/franela/goblin v0.0.0-20210113153425-413781f5e6c8
	github.com/getkin/kin-openapi v0.38.0
	github.com/go-chi/chi v4.1.2+incompatible
	github.com/golang/protobuf v1.4.3
	github.com/google/gopacket v1.1.18
	github.com/heetch/confita v0.9.2
	github.com/lightstep/otel-launcher-go v0.16.1
	github.com/rs/cors v1.7.0
	github.com/rs/xid v1.2.1
	github.com/schmurfy/chipi v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.6.1
	github.com/xujiajun/nutsdb v0.5.0
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.16.0
	go.opentelemetry.io/otel v0.16.0
	go.opentelemetry.io/otel/exporters/trace/jaeger v0.16.0
	go.opentelemetry.io/otel/sdk v0.16.0
	google.golang.org/grpc v1.35.0
	google.golang.org/protobuf v1.25.0
)

// replace github.com/xujiajun/nutsdb => /Users/schmurfy/Dev/personal/forks/nutsdb
replace github.com/xujiajun/nutsdb => github.com/schmurfy/nutsdb v0.5.1-0.20210204080048-b3851b16604f

replace github.com/schmurfy/chipi => /Users/schmurfy/Dev/personal/chipi
