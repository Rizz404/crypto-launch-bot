package exchange

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDialErrorNamesStage(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close()) // nothing listens there now

	s := &WSStream{URL: "ws://" + addr + "/stream"}
	s.withDefaults()
	err = s.session(context.Background(), nil, func([]byte) {})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stage connect")
}
