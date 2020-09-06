
all: sniffit

export PATH := ./bin:$(PATH)

.PHONY: proto sniffit

generated_pb:
	mkdir generated_pb

proto: tools generated_pb
	protoc \
  --go_out=plugins=grpc:generated_pb \
  --go_opt=paths=source_relative \
  proto/*.proto

sniffit: proto
	go build -o sniffit cmd/main.go

tools:
	go install github.com/golang/protobuf/protoc-gen-go