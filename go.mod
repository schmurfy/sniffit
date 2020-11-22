module github.com/schmurfy/sniffit

go 1.14

require (
	github.com/franela/goblin v0.0.0-20200825194134-80c0062ed6cd
	github.com/go-chi/chi v4.1.2+incompatible
	github.com/golang/protobuf v1.4.3
	github.com/google/gopacket v1.1.18
	github.com/heetch/confita v0.9.2
	github.com/lightstep/otel-launcher-go v0.13.0
	github.com/newrelic/opentelemetry-exporter-go v0.13.0
	github.com/peterbourgon/diskv v2.0.1+incompatible
	github.com/rs/xid v1.2.1
	github.com/stretchr/testify v1.6.1
	go.etcd.io/bbolt v1.3.5
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.13.0
	go.opentelemetry.io/otel v0.13.0
	go.opentelemetry.io/otel/exporters/trace/jaeger v0.13.0
	go.opentelemetry.io/otel/sdk v0.13.0
	google.golang.org/grpc v1.33.2
	google.golang.org/protobuf v1.25.0
)
