package store

import (
	"github.com/peterbourgon/diskv"

	"github.com/schmurfy/sniffit/models"
)

type StoreInterface interface {
	StorePacket(pkt *models.Packet) error
	FindPackets(ids []string) ([]*models.Packet, error)
	DeletePacket(pkt *models.Packet) error
}

type DiskStore struct {
	dv *diskv.Diskv
}

func NewDiskStore(path string) *DiskStore {

	return &DiskStore{
		dv: diskv.New(diskv.Options{
			BasePath: path,
		}),
	}
}

func (ds *DiskStore) StorePacket(pkt *models.Packet) error {
	data, err := pkt.Serialize()
	if err != nil {
		return err
	}

	return ds.dv.Write(pkt.Id, data)
}

func (ds *DiskStore) DeletePacket(pkt *models.Packet) error {
	return ds.dv.Erase(pkt.Id)
}

func (ds *DiskStore) FindPackets(ids []string) ([]*models.Packet, error) {
	ret := make([]*models.Packet, len(ids))

	for n, id := range ids {
		data, err := ds.dv.Read(id)
		if err != nil {
			return nil, err
		}

		pp, err := models.UnserializePacket(data)
		if err != nil {
			return nil, err
		}

		// ret[n] = gopacket.NewPacket(data, layers.LayerTypeEthernet, gopacket.Default)
		ret[n] = pp
	}

	return ret, nil
}
