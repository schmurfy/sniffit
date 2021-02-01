package nuts

import (
	"context"

	"github.com/schmurfy/sniffit/models"
	"github.com/schmurfy/sniffit/store"
	"github.com/xujiajun/nutsdb"
)

const (
	_dataBucket = "data"
)

func (n *NutsStore) StorePackets(ctx context.Context, pkts []*models.Packet) (err error) {
	err = n.db.Update(func(tx *nutsdb.Tx) error {
		var data []byte
		var err error

		for _, pkt := range pkts {

			data, err = pkt.Serialize()
			if err != nil {
				return err
			}

			err = tx.PutWithTimestamp(_dataBucket, []byte(pkt.Id), data, uint32(n.ttl.Seconds()), uint64(pkt.Timestamp.Unix()))
			if err != nil {
				return err
			}
		}

		return nil
	})

	return
}

func (n *NutsStore) GetPackets(ctx context.Context, ids []string, q *store.FindQuery) (pkts []*models.Packet, err error) {
	pkts = make([]*models.Packet, 0, len(ids))

	err = n.db.View(func(tx *nutsdb.Tx) error {
		var err error
		var entry *nutsdb.Entry

		for _, id := range ids {
			entry, err = tx.Get(_dataBucket, []byte(id))
			if err != nil {
				if err == nutsdb.ErrNotFoundKey {
					continue
				}

				return err
			}

			pp, err := models.UnserializePacket(entry.Value)
			if err != nil {
				return err
			}

			pkts = append(pkts, pp)
		}

		return nil
	})

	return
}
