package store

import (
	"context"
	"net"

	"github.com/schmurfy/sniffit/models"
)

type IndexInterface interface {
	IndexPackets(context.Context, []*models.Packet) error
	IndexKeys(context.Context) ([]string, error)
	FindPacketsByAddress(context.Context, net.IP) ([]string, error)
}

type DataInterface interface {
	StorePackets(context.Context, []*models.Packet) error
	GetPackets(context.Context, []string, *FindQuery) ([]*models.Packet, error)
	DataKeys(context.Context) ([]string, error)
}

type StoreInterface interface {
	IndexInterface
	DataInterface
}
