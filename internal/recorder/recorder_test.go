package recorder

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFIFOAndShutdownFlush(t *testing.T) {
	r := New(nil, 42, slog.New(slog.NewTextHandler(io.Discard, nil)))
	assert.Equal(t, int64(42), r.RunID())
	var got []int
	for i := range 10 {
		r.enqueue("test", func(ctx context.Context) error {
			assert.NoError(t, ctx.Err(), "shutdown must not cancel queued writes")
			got = append(got, i)
			return nil
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	go r.Run(ctx)
	select {
	case <-r.done:
	case <-time.After(2 * time.Second):
		t.Fatal("flush did not finish")
	}
	require.Len(t, got, 10)
	assert.Equal(t, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, got)
	r.Wait()
}

func TestFullQueueNeverBlocksEngine(t *testing.T) {
	r := New(nil, 1, slog.New(slog.NewTextHandler(io.Discard, nil)))
	for range queueSize + 1 {
		r.enqueue("noop", func(context.Context) error { return nil })
	}
	assert.Len(t, r.jobs, queueSize)
}
