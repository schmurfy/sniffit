package config

import (
	"context"
	"time"

	"github.com/heetch/confita"
	"github.com/heetch/confita/backend/env"
	"github.com/heetch/confita/backend/flags"
)

type Config struct {
	JaegerEndpoint         string `config:"jaeger_endpoint"`
	ExportTracesToNewRelic bool   `config:"newrelic,description=Export traces to NewRelic"`
	LightStep              bool   `config:"lightstep"`
}

type ArchivistConfig struct {
	Config

	ListenGRPCAddress string        `config:"listen_grpc,required,description=GRPC address to listen on"`
	ListenHTTPAddress string        `config:"listen_http,required"`
	DataPath          string        `config:"data_path,required"`
	IndexPath         string        `config:"index_path,required"`
	DataRetention     time.Duration `config:"retention"`
}

type AgentConfig struct {
	Config

	ArchivistAddress string `config:"archivist_address,required"`
	Filter           string `config:"filter,required,description=bpf filter used for capture"`
	InterfaceName    string `config:"interface,required,description=interface to listen on"`
	AgentName        string `config:"agent_name,required,description=the name is used to identify packet source in archivist"`
}

func Load(config interface{}) error {
	loader := confita.NewLoader(
		flags.NewBackend(),
		env.NewBackend(),
	)

	return loader.Load(context.Background(), config)
}
