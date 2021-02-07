
all: sniffit

export GOBIN := $(PWD)/bin
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

bench:
	go test -bench ./index

TEST_PACKAGE := ./...

filter :=

ifneq "$(TEST_FOCUS)" ""
	filter := $(filter) -goblin.run='$(TEST_FOCUS)'
endif

.PHONY: test
test:
	go test --tags=test -v $(TEST_PACKAGE) $(filter)


COVERAGE_OUT:=/tmp/cover
COVERAGE_RESULT:=/tmp/cover.html
coverage:
	go test -coverprofile $(COVERAGE_OUT) ./...
	go tool cover -html=$(COVERAGE_OUT) -o $(COVERAGE_RESULT)


# run the tests and run them again when a source file is changed
watch:
	find . -name "*.go" | entr -c make test
