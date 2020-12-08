package store

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/schmurfy/sniffit/models"
	bolt "go.etcd.io/bbolt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/label"
)

const (
	_tracer = "bbolt.store"
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

func (bs *BboltStore) StorePackets(ctx context.Context, pkts []*models.Packet) error {
	tr := otel.Tracer(_tracer)
	_, span := tr.Start(ctx, "StorePackets")
	defer span.End()

	return bs.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(_packetsBucketKey)

		for _, pkt := range pkts {
			data, err := pkt.Serialize()
			if err != nil {
				return err
			}

			err = b.Put([]byte(pkt.Id), data)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (bs *BboltStore) DeletePackets(ctx context.Context, pkts []*models.Packet) error {
	tr := otel.Tracer(_tracer)
	_, span := tr.Start(ctx, "DeletePackets")
	defer span.End()

	span.SetAttributes(
		label.Int("request.packets_count", len(pkts)),
	)

	err := bs.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(_packetsBucketKey)

		for _, pkt := range pkts {
			// ignore returned error
			b.Delete([]byte(pkt.Id))
		}

		return nil
	})

	if err != nil {
		span.RecordError(err)
	}

	return err
}

func (bs *BboltStore) FindPacketsBefore(ctx context.Context, t time.Time, maxCount int) ([]*models.Packet, error) {
	tr := otel.Tracer(_tracer)
	_, span := tr.Start(ctx, "FindPacketsBefore")
	span.SetAttributes(
		label.String("request.before", t.Format(time.RFC3339)),
		label.Int("request.max_count", maxCount),
	)
	defer span.End()

	ret := make(models.PacketSlice, 0)

	err := bs.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(_packetsBucketKey)

		c := b.Cursor()

		for k, data := c.First(); k != nil; k, data = c.Next() {
			pp, err := models.UnserializePacket(data)
			if err != nil {
				return err
			}

			if pp.Timestamp.Before(t) {
				ret = append(ret, pp)
				if len(ret) >= maxCount {
					return nil
				}
			}

		}

		return nil
	})

	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	span.SetAttributes(
		label.Int("response.count", ret.Len()),
	)

	return ret, nil
}

func (bs *BboltStore) FindPackets(ctx context.Context, ids []string, q *FindQuery) ([]*models.Packet, error) {
	tr := otel.Tracer(_tracer)
	_, span := tr.Start(ctx, "FindPackets")
	defer span.End()

	span.SetAttributes(
		label.Int("request.ids", len(ids)),
	)

	// if ids isempty stop there
	if len(ids) == 0 {
		return []*models.Packet{}, nil
	}

	if !q.From.IsZero() {
		span.SetAttributes(
			label.String("request.from", q.From.Format(time.RFC3339)),
		)
	}

	if !q.To.IsZero() {
		span.SetAttributes(
			label.String("request.to", q.From.Format(time.RFC3339)),
		)
	}

	if q.MaxCount > 0 {
		span.SetAttributes(
			label.Int("request.max_count", q.MaxCount),
		)
	}

	count := 0
	ret := make(models.PacketSlice, 0)

	err := bs.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(_packetsBucketKey)

		for _, id := range ids {
			data := b.Get([]byte(id))
			if data == nil {
				fmt.Printf("missing data for packet %s", id)
				continue
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

	span.SetAttributes(
		label.Int("response.count", len(ret)),
	)

	// take last X if MaxCount is present
	if q.MaxCount > 0 {
		sort.Sort(ret)
		return ret[len(ret)-q.MaxCount:], nil
	}

	return ret, nil
}
