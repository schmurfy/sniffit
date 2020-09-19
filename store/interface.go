package store

import (
	"net/http"
	"strconv"
	"time"

	"github.com/schmurfy/sniffit/models"
)

type FindQuery struct {
	From     time.Time
	To       time.Time
	MaxCount int
}

func (q *FindQuery) match(p *models.Packet) bool {
	if q == nil {
		return true
	}

	if !q.From.IsZero() && p.Timestamp.Before(q.From) {
		return false
	}

	if !q.To.IsZero() && p.Timestamp.After(q.To) {
		return false
	}

	return true
}

func QueryFromRequest(r *http.Request) (*FindQuery, error) {
	var ret FindQuery

	if val := r.URL.Query().Get("from"); val != "" {
		unixTime, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return nil, err
		}

		ret.From = time.Unix(unixTime, 0)
	}

	if val := r.URL.Query().Get("to"); val != "" {
		unixTime, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return nil, err
		}

		ret.To = time.Unix(unixTime, 0)
	}

	if val := r.URL.Query().Get("count"); val != "" {
		maxCount, err := strconv.Atoi(val)
		if err != nil {
			return nil, err
		}

		ret.MaxCount = maxCount
	}

	return &ret, nil
}

type StoreInterface interface {
	StorePackets(pkt []*models.Packet) error
	FindPackets(ids []string, q *FindQuery) ([]*models.Packet, error)
	DeletePacket(pkt *models.Packet) error
}
