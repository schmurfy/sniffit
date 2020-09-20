package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"go.opentelemetry.io/otel/exporters/trace/jaeger"
	"go.opentelemetry.io/otel/label"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/schmurfy/sniffit/agent"
	"github.com/schmurfy/sniffit/archivist"
	hs "github.com/schmurfy/sniffit/http"
	"github.com/schmurfy/sniffit/index"
	"github.com/schmurfy/sniffit/store"
)

var (
	_errMissingArgument = errors.New("missing required arguments")
)

func runArchivist() error {
	var grpcListenAddr, httpListenAddr, dataPath, indexFile string

	fs := flag.NewFlagSet("archivist", flag.ExitOnError)
	fs.StringVar(&grpcListenAddr, "listenGRPC", "", "GRPC listen address")
	fs.StringVar(&httpListenAddr, "listenHTTP", "", "HTTP Listen address")
	fs.StringVar(&dataPath, "data", "", "data path")
	fs.StringVar(&indexFile, "idx", "", "index file path")

	fs.Parse(os.Args[2:])

	if (grpcListenAddr == "") || (httpListenAddr == "") || (dataPath == "") || (indexFile == "") {
		fmt.Printf("arguments required\n")
		fs.Usage()
		return _errMissingArgument
	}

	dataStore, err := store.NewBboltStore(dataPath)
	if err != nil {
		return err
	}

	indexStore, err := index.NewBboltIndex(indexFile)
	if err != nil {
		return err
	}

	arc := archivist.New(
		dataStore, indexStore,
	)

	fmt.Printf("Starting Archivist...\n")

	go hs.Start(httpListenAddr, arc, indexStore, dataStore)

	return arc.Start(grpcListenAddr)
}

func runAgent() error {
	var archivistAddress, filter, ifName, agentName string

	fs := flag.NewFlagSet("agent", flag.ExitOnError)

	fs.StringVar(&archivistAddress, "addr", "", "archivist address")
	fs.StringVar(&filter, "filter", "", "set bpf filter")
	fs.StringVar(&ifName, "intf", "", "set interface to capture on")
	fs.StringVar(&agentName, "name", "", "set agent name")

	fs.Parse(os.Args[2:])

	if agentName == "" {
		fmt.Printf("agent name is required\n")
		fs.Usage()
		return _errMissingArgument
	}

	if (filter == "") || (ifName == "") {
		fmt.Printf("Filter and interface are required\n")
		fs.Usage()
		return _errMissingArgument
	}

	ag, err := agent.New(ifName, filter, archivistAddress, agentName)
	if err != nil {
		return err
	}

	fmt.Printf("Agent started...\n")

	return ag.Start()
}

func usage() {
	fmt.Printf("Usage: %s <archivist|agent>\n", os.Args[0])
}

func initTracer(serviceName string) func() {

	// Create and install Jaeger export pipeline
	flush, err := jaeger.InstallNewPipeline(
		jaeger.WithCollectorEndpoint("http://127.0.0.1:14268/api/traces"),
		jaeger.WithProcess(jaeger.Process{
			ServiceName: serviceName,
			Tags: []label.KeyValue{
				label.String("exporter", "jaeger"),
			},
		}),
		jaeger.WithSDK(&sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
	)
	if err != nil {
		log.Fatal(err)
	}

	return flush
}

func main() {
	var err error

	if len(os.Args) < 2 {
		usage()
		return
	}

	fn := initTracer(os.Args[1])
	defer fn()

	switch os.Args[1] {
	case "archivist":
		err = runArchivist()
	case "agent":
		err = runAgent()
	default:
		usage()
	}

	if err != nil {
		log.Fatalf(err.Error())
	}
}
