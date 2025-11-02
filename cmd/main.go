package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/uptrace/uptrace-go/uptrace"

	"github.com/schmurfy/sniffit/agent"
	"github.com/schmurfy/sniffit/archivist"
	"github.com/schmurfy/sniffit/config"
	hs "github.com/schmurfy/sniffit/http"
	"github.com/schmurfy/sniffit/index_encoder"
	"github.com/schmurfy/sniffit/stats"
	"github.com/schmurfy/sniffit/store"
	badgerStore "github.com/schmurfy/sniffit/store/badger"
	"github.com/schmurfy/sniffit/store/clickhouse"
)

var (
	_errMissingArgument = errors.New("missing required arguments")
	appVersion          = "dev"
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

	var indexStore store.IndexInterface
	var dataStore store.StoreInterface

	switch cfg.StoreType {
	case "badger":
		//index store
		opts := badgerStore.DefaultOptions
		opts.Path = cfg.IndexPath
		opts.TTL = cfg.DataRetention
		opts.Encoder = encoder

		indexBadgerStore, err := badgerStore.New(&opts)
		if err != nil {
			return err
		}
		defer indexBadgerStore.Close()
		indexStore = indexBadgerStore

		// data store
		opts = badgerStore.DefaultOptions
		opts.Path = cfg.DataPath
		opts.TTL = cfg.DataRetention
		opts.Encoder = encoder

		dataBadgerStore, err := badgerStore.New(&opts)
		if err != nil {
			return err
		}
		defer dataBadgerStore.Close()

		dataStore = dataBadgerStore

	case "clickhouse":
		clickStore, err := clickhouse.New(&clickhouse.Options{
			Addr:     []string{cfg.ClickhouseAddr},
			Database: cfg.ClickhouseDatabase,
			Username: cfg.ClickhouseUsername,
			Password: cfg.ClickhousePassword,
			TTL:      cfg.DataRetention,
		})
		if err != nil {
			return err
		}
		defer clickStore.Close()

		indexStore = clickStore
		dataStore = clickStore

	default:
		return fmt.Errorf("unknown store type: %s", cfg.StoreType)
	}

	st := stats.NewStats()

	arc, err := archivist.New(dataStore, indexStore, st, cfg)
	if err != nil {
		return err
	}

	go func() {
		err := hs.Start(cfg.ListenHTTPAddress, arc, indexStore, dataStore, st, cfg)
		if err != nil {
			fmt.Printf("http server failed to start: %s\n", err.Error())
		}
	}()

	flush, err := initTracer("archivist", &cfg.Config)
	if err != nil {
		return err
	}
	defer flush()

	fmt.Printf("[%s] Archivist started...\n", appVersion)

	return arc.Start(cfg.ListenGRPCAddress)
}

func runAgent() error {
	cfg := &config.AgentConfig{}

	err := config.Load(cfg)
	if err != nil {
		flag.Usage()
		fmt.Print("\n")
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

	fmt.Printf("Starting Agent in 2s...\n")

	time.Sleep(2 * time.Second)

	fmt.Printf("Agent started...\n")

	return ag.Start()
}

func usage() {
	fmt.Printf("Usage: %s <archivist|agent>\n", os.Args[0])
}

func initTracer(serviceName string, cfg *config.Config) (func(), error) {
	if cfg.UptraceDSN != "" {
		uptrace.ConfigureOpentelemetry(
			uptrace.WithServiceName(serviceName),
			uptrace.WithServiceVersion(appVersion),
			uptrace.WithDSN(cfg.UptraceDSN),
		)

		fmt.Printf("Uptrace enabled.\n")

		return func() { uptrace.Shutdown(context.Background()) }, nil
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
		log.Fatalf("%+v", err)
	}
}
