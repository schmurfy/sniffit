
all: sniffit

export PATH := ./bin:$(PATH)

.PHONY: proto sniffit

proto: tools
	which protoc-gen-go
	which protoc-gen-go-grpc
	protoc \
  --go_out=plugins=grpc:generated_pb \
  --go_opt=paths=source_relative \
  proto/*.proto

sniffit: proto
	go build -o sniffit cmd/main.go

tools:
	go install github.com/golang/protobuf/protoc-gen-go