package store

import (
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
