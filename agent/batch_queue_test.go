package agent

import (
	"testing"
	"time"

	. "github.com/franela/goblin"
	"github.com/stretchr/testify/assert"

	pb "github.com/schmurfy/sniffit/generated_pb/proto"
)

func TestBatchQueue(t *testing.T) {
	g := Goblin(t)

	g.Describe("BatchQueue", func() {
		var q *BatchQueue
		var received []*pb.Packet

		g.BeforeEach(func() {
			q = NewBatchQueue(10, 200*time.Millisecond, func(pkts []*pb.Packet) {
				received = pkts
			})
		})

		g.It("should call function when queue is full (x3)", func() {
			for i := 0; i < 3; i++ {
				for i := 0; i < 9; i++ {
					q.Add(&pb.Packet{})
				}

				// the queue is not full yet so the function should not have
				// been called yet
				assert.Emptyf(g, received, "size: %d", len(received))

				// add the last one
				q.Add(&pb.Packet{})

				assert.Len(g, received, 10)

				// clear the received data for next run
				received = received[:0]
			}
		})

		g.It("should call function when timeout is reached", func() {
			for j := 0; j < 3; j++ {
				for i := 0; i < 5; i++ {
					q.Add(&pb.Packet{})
				}
				time.Sleep(300 * time.Millisecond)

				assert.Lenf(g, received, 5, "run %d received %d pkts", j, len(received))

				// clear the received data for next run
				received = received[:0]
			}
		})
	})
}
