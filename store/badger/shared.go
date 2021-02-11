package badger_store

import (
	"time"

	badger "github.com/dgraph-io/badger/v3"
	"github.com/pkg/errors"
	"github.com/schmurfy/sniffit/index_encoder"
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

func (b *BadgerStore) Close() {
	if b.db != nil {
		b.db.Close()
	}
}
