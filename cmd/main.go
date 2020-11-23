package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/lightstep/otel-launcher-go/launcher"
	"github.com/newrelic/opentelemetry-exporter-go/newrelic"
	"go.opentelemetry.io/otel/exporters/trace/jaeger"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/schmurfy/sniffit/agent"
	"github.com/schmurfy/sniffit/archivist"
	"github.com/schmurfy/sniffit/config"
	hs "github.com/schmurfy/sniffit/http"
	"github.com/schmurfy/sniffit/index"
	"github.com/schmurfy/sniffit/stats"
	"github.com/schmurfy/sniffit/store"
)

var (
	_errMissingArgument = errors.New("missing required arguments")
)

func runArchivist() error {
	cfg := &config.ArchivistConfig{
		DataRetention: "168h", // one week
	}

	err := config.Load(cfg)
	if err != nil {
		flag.Usage()
		fmt.Print("\n")
		return err
	}

	dataStore, err := store.NewBboltStore(cfg.DataPath)
	if err != nil {
		return err
	}

	indexStore, err := index.NewBboltIndex(cfg.IndexFilePath)
	if err != nil {
		return err
	}

	st := stats.NewStats()

	arc, err := archivist.New(dataStore, indexStore, st, cfg)
	if err != nil {
		return err
	}

	go hs.Start(cfg.ListenHTTPAddress, arc, indexStore, dataStore, st)

	flush, err := initTracer("archivist", &cfg.Config)
	if err != nil {
		return err
	}
	defer flush()

	fmt.Printf("Archivist started...\n")

	return arc.Start(cfg.ListenGRPCAddress)
}

func runAgent() error {
	cfg := &config.AgentConfig{}

	err := config.Load(cfg)
	if err != nil {
		return err
	}

	ag, err := agent.New(cfg.InterfanceName, cfg.Filter, cfg.ArchivistAddress, cfg.AgentName)
	if err != nil {
		return err
	}

	flush, err := initTracer("agent", &cfg.Config)
	if err != nil {
		return err
	}
	defer flush()

	fmt.Printf("Agent started...\n")

	return ag.Start()
}

func usage() {
	fmt.Printf("Usage: %s <archivist|agent>\n", os.Args[0])
}

func initTracer(serviceName string, cfg *config.Config) (func(), error) {
	if cfg.JaegerEndpoint != "" {
		return jaeger.InstallNewPipeline(
			jaeger.WithCollectorEndpoint(cfg.JaegerEndpoint),
			jaeger.WithProcessFromEnv(),
			jaeger.WithSDK(&sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
			jaeger.WithDisabledFromEnv(),
		)

	} else if cfg.ExportTracesToNewRelic {
		controller, err := newrelic.InstallNewPipeline(serviceName)
		return controller.Stop, err
	} else if cfg.LightStep {

		token, found := os.LookupEnv("LIGHTSTEP_TOKEN")
		if !found {
			return nil, errors.New("lightstep token missing")
		}

		otel := launcher.ConfigureOpentelemetry(
			launcher.WithServiceName(serviceName),
			// launcher.WithLogLevel("debug"),
			launcher.WithMetricExporterEndpoint("ingest.lightstep.com:443"),
			launcher.WithAccessToken(token),
		)
		return otel.Shutdown, nil
	}

	return func() {}, nil
}

func main() {
	var err error

	if len(os.Args) < 2 {
		usage()
		return
	}

	app := os.Args[1]

	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
	switch app {
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
