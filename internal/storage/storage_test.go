package storage

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// After a host reboot the database container may still refuse connections
// when the bot starts; Open waits for it instead of exiting at once.
func TestOpenRetriesRefusedConnectionsUntilConnectWait(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close()) // nothing listens: connection refused

	old := ConnectWait
	ConnectWait = 900 * time.Millisecond
	t.Cleanup(func() { ConnectWait = old })
	start := time.Now()
	_, err = Open(context.Background(), "postgres://bot:pw@"+addr+"/tradingbot?sslmode=disable&connect_timeout=1", 2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ping database")
	assert.GreaterOrEqual(t, time.Since(start), 500*time.Millisecond, "retried before giving up")
	assert.Less(t, time.Since(start), 5*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ConnectWait = time.Minute
	start = time.Now()
	_, err = Open(ctx, "postgres://bot:pw@"+addr+"/tradingbot?sslmode=disable", 2)
	require.Error(t, err)
	assert.Less(t, time.Since(start), 2*time.Second, "shutdown interrupts the wait")
}
