package main

import (
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

func runArchivist() {
	var listenAddr, dataPath, indexFile string

	fs := flag.NewFlagSet("archivist", flag.ExitOnError)
	fs.StringVar(&listenAddr, "listen", "", "Listen address")
	fs.StringVar(&dataPath, "data", "", "data path")
	fs.StringVar(&indexFile, "idx", "", "index file path")

	fs.Parse(os.Args[2:])

	if (listenAddr == "") || (dataPath == "") || (indexFile == "") {
		fmt.Printf("arguments required\n")
		fs.Usage()
		return
	}

	dataStore := store.NewDiskvStore(dataPath)
	indexStore, err := index.NewBboltIndex(indexFile)
	if err != nil {
		log.Fatalf(err.Error())
		return
	}

	arc := archivist.New(
		dataStore, indexStore,
	)

	fmt.Printf("Starting Archivist...\n")

	go hs.Start(":9999", indexStore, dataStore)

	arc.Start(listenAddr)
}

func runAgent() {
	var archivistAddress, filter, ifName string

	fs := flag.NewFlagSet("agent", flag.ExitOnError)

	fs.StringVar(&archivistAddress, "addr", "", "archivist address")
	fs.StringVar(&filter, "filter", "", "set bpf filter")
	fs.StringVar(&ifName, "intf", "", "set interface to capture on")

	fs.Parse(os.Args[2:])

	if (filter == "") || (ifName == "") {
		fmt.Printf("Filter and interface are required\n")
		fs.Usage()
		return
	}

	ag, err := agent.New(ifName, filter, archivistAddress)
	if err != nil {
		log.Fatalf(err.Error())
		return
	}

	fmt.Printf("Starting agent...\n")

	err = ag.Start()
	if err != nil {
		log.Fatalf(err.Error())
		return
	}

}

func usage() {
	fmt.Printf("Usage: %s <archivist|agent>\n", os.Args[0])
}

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}

	switch os.Args[1] {
	case "archivist":
		runArchivist()
	case "agent":
		runAgent()
	default:
		usage()
	}
}
