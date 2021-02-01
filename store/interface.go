package store

import (
	"context"
	"net"

	"github.com/schmurfy/sniffit/models"
)

type IndexInterface interface {
	IndexPackets(ctx context.Context, pkt []*models.Packet) error
	AnyKeys() ([]string, error)
	FindPacketsByAddress(ctx context.Context, ip net.IP) ([]string, error)
}

type DataInterface interface {
	StorePackets(ctx context.Context, pkt []*models.Packet) error
	GetPackets(ctx context.Context, ids []string, q *FindQuery) ([]*models.Packet, error)
}

type StoreInterface interface {
	IndexInterface
	DataInterface
}
