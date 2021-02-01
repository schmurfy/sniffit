package nuts

import (
	"time"

	"github.com/schmurfy/sniffit/index_encoder"
	"github.com/xujiajun/nutsdb"
)

type NutsStore struct {
	db          *nutsdb.DB
	encoder     index_encoder.Interface
	ttl         time.Duration
	timeFormat  string
	currentTime func() time.Time
}

type NutsStoreOptions struct {
	Path        string
	Encoder     index_encoder.Interface
	TimeFormat  string
	CurrentTime func() time.Time
	TTL         time.Duration
}

func New(o *NutsStoreOptions) (*NutsStore, error) {
	opts := nutsdb.DefaultOptions
	opts.Dir = o.Path

	db, err := nutsdb.Open(opts)
	if err != nil {
		return nil, err
	}

	return &NutsStore{
		db:          db,
		encoder:     o.Encoder,
		timeFormat:  o.TimeFormat,
		currentTime: o.CurrentTime,
		ttl:         o.TTL,
	}, nil
}

func (n *NutsStore) computeTTL(t time.Time) uint32 {
	expireTime := n.currentTime().Add(n.ttl).Unix()
	return uint32(expireTime - t.Unix())
}

func (n *NutsStore) Close() {
	if n.db != nil {
		n.db.Close()
	}
}
