package recorder

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFIFOAndShutdownFlush(t *testing.T) {
	r := New(nil, 42, slog.New(slog.NewTextHandler(io.Discard, nil)))
	assert.Equal(t, int64(42), r.RunID())
	var got []int
	for i := range 10 {
		r.enqueue("test", true, func(ctx context.Context) error {
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
		r.enqueue("noop", true, func(context.Context) error { return nil })
	}
	assert.Len(t, r.jobs, queueSize)
}

func fastRetries(t *testing.T) {
	t.Helper()
	oldMin, oldMax, oldFlush := retryMin, retryMax, flushTimeout
	retryMin, retryMax, flushTimeout = time.Millisecond, 4*time.Millisecond, 2*time.Second
	t.Cleanup(func() { retryMin, retryMax, flushTimeout = oldMin, oldMax, oldFlush })
}

func TestTransientFailureRetriesInOrder(t *testing.T) {
	fastRetries(t)
	r := New(nil, 1, slog.New(slog.NewTextHandler(io.Discard, nil)))
	var got []string
	failures := 3
	r.enqueue("order", true, func(ctx context.Context) error {
		if failures > 0 {
			failures--
			return fmt.Errorf("insert order: %w", context.DeadlineExceeded)
		}
		got = append(got, "order")
		return nil
	})
	r.enqueue("fill", true, func(context.Context) error { got = append(got, "fill"); return nil })
	ctx, cancel := context.WithCancel(context.Background())
	go r.Run(ctx)
	require.Eventually(t, func() bool { return len(r.jobs) == 0 }, 2*time.Second, time.Millisecond)
	cancel()
	r.Wait()
	assert.Equal(t, []string{"order", "fill"}, got, "the fill must wait for its order")
	assert.Zero(t, failures)
}

func TestPermanentAndAmbiguousFailuresAreNotRepeated(t *testing.T) {
	fastRetries(t)
	r := New(nil, 1, slog.New(slog.NewTextHandler(io.Discard, nil)))
	calls := map[string]int{}
	r.enqueue("constraint", true, func(context.Context) error {
		calls["constraint"]++
		return &pgconn.PgError{Code: "23505"}
	})
	// A trade insert that timed out may have committed; repeating it could
	// store the trade twice.
	r.enqueue("trade", false, func(context.Context) error {
		calls["trade"]++
		return context.DeadlineExceeded
	})
	r.enqueue("trade-unreachable", false, func(context.Context) error {
		calls["trade-unreachable"]++
		if calls["trade-unreachable"] == 1 {
			return &pgconn.ConnectError{}
		}
		return nil
	})
	r.enqueue("trade-rejected", false, func(context.Context) error {
		calls["trade-rejected"]++
		if calls["trade-rejected"] == 1 {
			return &pgconn.PgError{Code: "57P01"}
		}
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	go r.Run(ctx)
	r.Wait()
	assert.Equal(t, map[string]int{"constraint": 1, "trade": 1, "trade-unreachable": 2, "trade-rejected": 2}, calls)
}

func TestShutdownDuringRetryFlushesPendingWrite(t *testing.T) {
	fastRetries(t)
	r := New(nil, 1, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	var got []string
	attempts := 0
	r.enqueue("order", true, func(context.Context) error {
		attempts++
		if attempts == 1 {
			cancel() // shutdown arrives while the database is down
			return errors.New("connection reset")
		}
		got = append(got, "order")
		return nil
	})
	r.enqueue("fill", true, func(context.Context) error { got = append(got, "fill"); return nil })
	go r.Run(ctx)
	r.Wait()
	assert.Equal(t, []string{"order", "fill"}, got)
}

func TestFlushGivesUpWhenDatabaseStaysDown(t *testing.T) {
	fastRetries(t)
	flushTimeout = 50 * time.Millisecond
	r := New(nil, 1, slog.New(slog.NewTextHandler(io.Discard, nil)))
	r.enqueue("order", true, func(context.Context) error { return errors.New("connection refused") })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	go r.Run(ctx)
	select {
	case <-r.done:
	case <-time.After(2 * time.Second):
		t.Fatal("flush must give up after flushTimeout")
	}
}
