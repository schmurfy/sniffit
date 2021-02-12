package badger_store

import (
	"fmt"
	"strconv"
	"time"

	badger "github.com/dgraph-io/badger/v3"
	"github.com/pkg/errors"
	"github.com/schmurfy/sniffit/index_encoder"
	"github.com/schmurfy/sniffit/store"
)

var (
	DefaultOptions = Options{
		TimeFormat: "2006:01:02",
	}
)

type BadgerStore struct {
	db         *badger.DB
	encoder    index_encoder.Interface
	timeFormat string
	ttl        time.Duration
}

type Options struct {
	Path       string
	Encoder    index_encoder.Interface
	TimeFormat string
	TTL        time.Duration
}

func New(o *Options) (*BadgerStore, error) {
	opts := badger.DefaultOptions(o.Path)
	db, err := badger.Open(opts)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &BadgerStore{
		db:         db,
		encoder:    o.Encoder,
		timeFormat: o.TimeFormat,
		ttl:        o.TTL,
	}, nil
}


func (b *BadgerStore) GetStats() (*store.Stats, error) {
	lsmSize, vlogSize := b.db.Size()

	b.db.PrintHistogram([]byte(""))

	err := b.db.RunValueLogGC(0.5)
	if err != nil {
		fmt.Printf("err: %s\n", err.Error())
	}

	ret := &store.Stats{
		"lsmSize":  strconv.FormatInt(lsmSize, 10),
		"vlogSize": strconv.FormatInt(vlogSize, 10),
	}

	return ret, nil
}

func (b *BadgerStore) Close() {
	if b.db != nil {
		b.db.Close()
	}
}
