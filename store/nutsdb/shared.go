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

	// initialize buckets
	err = db.Update(func(tx *nutsdb.Tx) error {
		return tx.Put(_indexBucket, []byte("_"), []byte("dummy"), nutsdb.Persistent)
	})

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

func (n *NutsStore) Close() {
	if n.db != nil {
		n.db.Close()
	}
}

func (n *NutsStore) listKeys(bucket string) (ret []string, err error) {
	err = n.db.View(func(tx *nutsdb.Tx) error {
		entries, err := tx.GetAll(bucket)
		if err != nil {
			if err == nutsdb.ErrBucketEmpty {
				ret = []string{}
				return nil
			}

			return err
		}

		ret = make([]string, 0, len(entries))

		for _, e := range entries {
			ret = append(ret, string(e.Key))
		}

		return nil
	})

	if err != nil {
		return
	}

	return
}
