package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

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

	dataStore := store.NewDiskvStore(dataPath)
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
	var archivistAddress, filter, ifName string

	fs := flag.NewFlagSet("agent", flag.ExitOnError)

	fs.StringVar(&archivistAddress, "addr", "", "archivist address")
	fs.StringVar(&filter, "filter", "", "set bpf filter")
	fs.StringVar(&ifName, "intf", "", "set interface to capture on")

	fs.Parse(os.Args[2:])

	if (filter == "") || (ifName == "") {
		fmt.Printf("Filter and interface are required\n")
		fs.Usage()
		return _errMissingArgument
	}

	ag, err := agent.New(ifName, filter, archivistAddress)
	if err != nil {
		return err
	}

	fmt.Printf("Agent started...\n")

	return ag.Start()
}

func usage() {
	fmt.Printf("Usage: %s <archivist|agent>\n", os.Args[0])
}

func main() {
	var err error

	if len(os.Args) < 2 {
		usage()
		return
	}

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
