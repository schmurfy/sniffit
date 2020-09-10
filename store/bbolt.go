package store

import (
	"fmt"
	"sort"

	"github.com/schmurfy/sniffit/models"
	bolt "go.etcd.io/bbolt"
)

var (
	_packetsBucketKey = []byte("packets")
	_buckets          = [][]byte{_packetsBucketKey}
)

type BboltStore struct {
	db *bolt.DB
}

func NewBboltStore(path string) (*BboltStore, error) {
	var ret BboltStore

	var err error

	ret.db, err = bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}

	// create buckets
	err = ret.db.Update(func(tx *bolt.Tx) error {
		for _, bucketName := range _buckets {
			_, err = tx.CreateBucketIfNotExists(bucketName)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &ret, nil
}

func (bs *BboltStore) StorePacket(pkt *models.Packet) error {
	data, err := pkt.Serialize()
	if err != nil {
		return err
	}

	return bs.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(_packetsBucketKey)

		return b.Put([]byte(pkt.Id), data)
	})
}

func (bs *BboltStore) DeletePacket(pkt *models.Packet) error {
	return bs.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(_packetsBucketKey)

		return b.Delete([]byte(pkt.Id))
	})
}

func (bs *BboltStore) FindPackets(ids []string, q *FindQuery) ([]*models.Packet, error) {
	count := 0
	ret := make(models.PacketSlice, 0)

	err := bs.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(_packetsBucketKey)

		for _, id := range ids {
			data := b.Get([]byte(id))
			if data == nil {
				return fmt.Errorf("missing data for packet %s", id)
			}
			pp, err := models.UnserializePacket(data)
			if err != nil {
				return err
			}

			if q.match(pp) {
				ret = append(ret, pp)
				count++
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// take last X if MaxCount is present
	if q.MaxCount > 0 {
		sort.Sort(ret)
		return ret[len(ret)-q.MaxCount:], nil
	}

	return ret, nil
}
