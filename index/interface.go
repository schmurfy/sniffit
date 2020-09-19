package index

import (
	"net"

	"github.com/schmurfy/sniffit/models"
)

type IndexInterface interface {
	IndexPackets(pkt []*models.Packet) ([]error, bool)
	AnyKeys() ([]string, error)
	FindPackets(ip net.IP) ([]string, error)
	Close()
}
