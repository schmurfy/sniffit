package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/lightstep/otel-launcher-go/launcher"
	"go.opentelemetry.io/otel/exporters/trace/jaeger"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/schmurfy/sniffit/agent"
	"github.com/schmurfy/sniffit/archivist"
	"github.com/schmurfy/sniffit/config"
	hs "github.com/schmurfy/sniffit/http"
	"github.com/schmurfy/sniffit/index_encoder"
	"github.com/schmurfy/sniffit/stats"
	badgerStore "github.com/schmurfy/sniffit/store/badger"
)

var (
	_errMissingArgument = errors.New("missing required arguments")
)

func runArchivist() error {
	cfg := &config.ArchivistConfig{
		DataRetention: 7 * 24 * time.Hour, // one week
	}

	err := config.Load(cfg)
	if err != nil {
		flag.Usage()
		fmt.Print("\n")
		return err
	}

	encoder, err := index_encoder.NewProto()
	if err != nil {
		return err
	}

	//index store
	opts := badgerStore.DefaultOptions
	opts.Path = cfg.IndexPath
	opts.TTL = cfg.DataRetention
	opts.Encoder = encoder

	indexStore, err := badgerStore.New(&opts)
	if err != nil {
		return err
	}
	defer indexStore.Close()

	// data store
	opts = badgerStore.DefaultOptions
	opts.Path = cfg.DataPath
	opts.TTL = cfg.DataRetention
	opts.Encoder = encoder

	dataStore, err := badgerStore.New(&opts)
	if err != nil {
		return err
	}
	defer dataStore.Close()
	// ------

	st := stats.NewStats()

	arc, err := archivist.New(dataStore, indexStore, st, cfg)
	if err != nil {
		return err
	}

	go func() {
		err := hs.Start(cfg.ListenHTTPAddress, arc, indexStore, dataStore, st)
		if err != nil {
			fmt.Printf("http server failed to start: %s\n", err.Error())
		}
	}()

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

	ag, err := agent.New(cfg.InterfaceName, cfg.Filter, cfg.ArchivistAddress, cfg.AgentName)
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
			jaeger.WithProcess(jaeger.Process{
				ServiceName: serviceName,
			}),
			jaeger.WithSDK(&sdktrace.Config{
				DefaultSampler: sdktrace.AlwaysSample(),
			}),
		)

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
