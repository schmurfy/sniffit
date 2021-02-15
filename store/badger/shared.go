package badger_store

import (
	"context"
	"strconv"
	"sync"
	"time"

	badger "github.com/dgraph-io/badger/v3"
	"github.com/pkg/errors"
	"github.com/schmurfy/sniffit/index_encoder"
	"github.com/schmurfy/sniffit/store"
)

var (
	DefaultOptions = Options{
		CachedIndexKeysInterval: 15 * time.Minute,
	}
)

type BadgerStore struct {
	db        *badger.DB
	encoder   index_encoder.Interface
	ttl       time.Duration
	ctx       context.Context
	cancelCtx func()

	cachedIndexKeys         []string
	cachedIndexKeysInterval time.Duration
	cachedIndexKeysMutex    sync.Mutex
	lastIndexKeysScan       time.Time
}

type Options struct {
	Path                    string
	Encoder                 index_encoder.Interface
	CachedIndexKeysInterval time.Duration
	TTL                     time.Duration
}

func New(o *Options) (*BadgerStore, error) {
	opts := badger.DefaultOptions(o.Path)
	db, err := badger.Open(opts)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	ret := &BadgerStore{
		db:                      db,
		encoder:                 o.Encoder,
		ttl:                     o.TTL,
		cachedIndexKeysInterval: o.CachedIndexKeysInterval,
		ctx:                     ctx,
		cancelCtx:               cancel,
	}

	go ret.backgroundCleanup(1*time.Hour, 0.7)

	return ret, nil
}

func (b *BadgerStore) backgroundCleanup(frequency time.Duration, ratio float64) {
	ticker := time.NewTicker(frequency)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
		again:
			err := b.db.RunValueLogGC(ratio)
			if err == nil {
				goto again
			}

		case <-b.ctx.Done():
			return
		}

	}
}

func (b *BadgerStore) GetStats() (*store.Stats, error) {
	lsmSize, vlogSize := b.db.Size()

	// b.db.PrintHistogram([]byte(""))

	ret := &store.Stats{
		"lsmSize":  strconv.FormatInt(lsmSize, 10),
		"vlogSize": strconv.FormatInt(vlogSize, 10),
	}

	return ret, nil
}

func (b *BadgerStore) Close() {
	if b.db != nil {
		b.db.Close()
		b.cancelCtx()
	}
}
