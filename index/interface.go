package index

import (
	"net"

	"github.com/schmurfy/sniffit/models"
)

type IndexInterface interface {
	IndexPacket(pkt *models.Packet) error
	AnyKeys() ([]string, error)
	FindPackets(ip net.IP) ([]string, error)
	Close()
}
