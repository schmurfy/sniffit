package index

import (
	"context"
	"net"
	"time"

	"github.com/recoilme/sniper"
	"go.opentelemetry.io/otel"

	"github.com/schmurfy/sniffit/index_encoder"
	"github.com/schmurfy/sniffit/models"
)

var (
	_sniperTracer = otel.Tracer("index:sniper")
)

type SniperIndex struct {
	store   *sniper.Store
	encoder index_encoder.Interface
}

func NewSniperIndex(path string, encoder index_encoder.Interface) (*SniperIndex, error) {
	store, err := sniper.Open(
		sniper.Dir(path),
	)

	if err != nil {
		return nil, err
	}

	return &SniperIndex{
		store:   store,
		encoder: encoder,
	}, nil
}

func (i *SniperIndex) IndexPackets(ctx context.Context, pkts []*models.Packet) error {
	indexes, err := buildIdList(pkts)
	if err != nil {
		return err
	}

	for key, ids := range indexes {
		var list index_encoder.ValueInterface

		addr := []byte(key)

		data, err := i.store.Get(addr)
		if (err != nil) && (err != sniper.ErrNotFound) {
			return err
		}

		// load existing data
		if err != sniper.ErrNotFound {
			list, err = i.encoder.NewFromData(data)
		} else {
			list, err = i.encoder.NewEmpty()
		}

		if err != nil {
			return err
		}

		// now add the new entry
		list.Add(ids...)

		// and serialize it back
		newData, err := list.Serialize()
		if err != nil {
			return err
		}

		t := time.Now().Add(3 * time.Hour).Unix()
		err = i.store.Set(addr, newData, uint32(t))
		if err != nil {
			return err
		}
	}

	return nil
}

func (i *SniperIndex) AnyKeys() ([]string, error) {
	return []string{}, nil
}

func (i *SniperIndex) FindPackets(ctx context.Context, ip net.IP) (ret []string, err error) {
	var data []byte

	ctx, span := _sniperTracer.Start(ctx, "FindPackets")
	defer func() {
		if err != nil {
			span.RecordError(err)
		}
		span.End()
	}()

	data, err = i.store.Get(ip)
	if err != nil {
		return
	}

	list, err := i.encoder.NewFromData(data)
	if err != nil {
		return
	}

	return list.GetIds()
}

func (i *SniperIndex) DeletePackets(ctx context.Context, pkts []*models.Packet) error {
	return nil
}
