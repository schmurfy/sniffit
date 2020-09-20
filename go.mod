module github.com/schmurfy/sniffit

go 1.14

require (
	github.com/franela/goblin v0.0.0-20200825194134-80c0062ed6cd
	github.com/go-chi/chi v4.1.2+incompatible
	github.com/golang/protobuf v1.4.2
	github.com/google/btree v1.0.0 // indirect
	github.com/google/gopacket v1.1.18
	github.com/peterbourgon/diskv v2.0.1+incompatible
	github.com/rs/xid v1.2.1
	github.com/stretchr/testify v1.6.1
	go.etcd.io/bbolt v1.3.5
	go.opencensus.io v0.22.4 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc v0.11.0
	go.opentelemetry.io/otel v0.11.0
	go.opentelemetry.io/otel/exporters/stdout v0.11.0
	go.opentelemetry.io/otel/exporters/trace/jaeger v0.11.0
	go.opentelemetry.io/otel/sdk v0.11.0
	google.golang.org/grpc v1.32.0
	google.golang.org/protobuf v1.25.0
)
