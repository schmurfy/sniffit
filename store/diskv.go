package store

import (
	"sort"

	"github.com/peterbourgon/diskv"
	"github.com/schmurfy/sniffit/models"
)

type DiskvStore struct {
	dv *diskv.Diskv
}

func NewDiskvStore(path string) *DiskvStore {

	return &DiskvStore{
		dv: diskv.New(diskv.Options{
			BasePath: path,
		}),
	}
}

func (ds *DiskvStore) StorePacket(pkt *models.Packet) error {
	data, err := pkt.Serialize()
	if err != nil {
		return err
	}

	return ds.dv.Write(pkt.Id, data)
}

func (ds *DiskvStore) DeletePacket(pkt *models.Packet) error {
	return ds.dv.Erase(pkt.Id)
}

func (ds *DiskvStore) FindPackets(ids []string, q *FindQuery) ([]*models.Packet, error) {
	count := 0
	ret := make(models.PacketSlice, 0)

	for _, id := range ids {
		data, err := ds.dv.Read(id)
		if err != nil {
			return nil, err
		}

		pp, err := models.UnserializePacket(data)
		if err != nil {
			return nil, err
		}

		if q.match(pp) {
			ret = append(ret, pp)
			count++
		}
	}

	// take last X if MaxCount is present
	if q.MaxCount > 0 {
		sort.Sort(ret)
		return ret[len(ret)-q.MaxCount:], nil
	}

	return ret, nil
}
