package media

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/localdb"
)

// Validated segments reach a subscriber in validation order.
func TestNewSegmentFanoutKeepsOrder(t *testing.T) {
	mm := &MediaManager{}
	ch := mm.NewSegment()
	const n = 200
	go func() {
		for i := 0; i < n; i++ {
			mm.notifySubscribers(context.Background(), &NewSegmentNotification{Segment: &localdb.Segment{ID: fmt.Sprint(i)}})
		}
	}()
	for i := 0; i < n; i++ {
		require.Equal(t, fmt.Sprint(i), (<-ch).Segment.ID)
	}
}
