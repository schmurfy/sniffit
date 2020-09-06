package store

import (
	"github.com/schmurfy/sniffit/models"
)

type StoreInterface interface {
	StorePacket(pkt *models.Packet) error
	FindPackets(ids []string) ([]*models.Packet, error)
	DeletePacket(pkt *models.Packet) error
}
