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
	UptraceDSN             string `config:"uptrace_dsn"`
	SnapLen                int32  `config:"snap_len"`
}

type ArchivistConfig struct {
	Config

	ListenGRPCAddress string        `config:"listen_grpc,required,description=GRPC address to listen on"`
	ListenHTTPAddress string        `config:"listen_http,required"`
	DataPath          string        `config:"data_path"`
	IndexPath         string        `config:"index_path"`
	DataRetention     time.Duration `config:"retention"`
	StoreType         string        `config:"store_type,required"`

	// clickhouse
	ClickhouseAddr     string `config:"clickhouse_addr"`
	ClickhouseDatabase string `config:"clickhouse_database"`
	ClickhouseUsername string `config:"clickhouse_username"`
	ClickhousePassword string `config:"clickhouse_password"`
}

type AgentConfig struct {
	Config

	ArchivistAddress string `config:"archivist_address,required"`
	Filter           string `config:"filter,required,description=bpf filter used for capture"`
	InterfaceName    string `config:"interface,required,description=interface to listen on"`
	AgentName        string `config:"agent_name,required,description=the name is used to identify packet source in archivist"`
	BatchSize        int    `config:"batch_size,required"`
}

func Load(config any) error {
	loader := confita.NewLoader(
		flags.NewBackend(),
		env.NewBackend(),
	)

	return loader.Load(context.Background(), config)
}
