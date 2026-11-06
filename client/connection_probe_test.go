package client

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/cenkalti/rpc2"
	"github.com/cenkalti/rpc2/jsonrpc"
	"github.com/stretchr/testify/require"
)

func TestConnectionManagerProbeUsesConfiguredDeadline(t *testing.T) {
	for _, tc := range []struct {
		name           string
		timeout, delay time.Duration
		wantTimeout    bool
	}{
		{"short", 50 * time.Millisecond, time.Second, true},
		{"long", 3 * time.Second, 2200 * time.Millisecond, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			left, right := net.Pipe()
			peer := rpc2.NewClientWithCodec(jsonrpc.NewJSONCodec(left))
			release := make(chan struct{})
			defer close(release)
			peer.Handle("echo", func(_ *rpc2.Client, args []any, reply *[]any) error {
				select {
				case <-time.After(tc.delay):
				case <-release:
				}
				*reply = args
				return nil
			})
			go peer.Run()
			defer peer.Close()
			opts, err := newOptions()
			require.NoError(t, err)
			opts.inactivityTimeout = tc.timeout
			cm := NewConnectionManager(opts, "Open_vSwitch", nil, make(chan Event, 8), nil)
			cm.rpcClient = rpc2.NewClientWithCodec(jsonrpc.NewJSONCodec(right))
			cm.state.Store(int32(StateConnected))
			go cm.rpcClient.Run()
			go cm.Run()
			defer func() { cm.Do(context.Background(), Command{Type: CmdClose}); <-cm.Done() }()
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			start := time.Now()
			r := cm.Do(ctx, Command{Type: CmdRunInactivityEcho})
			if tc.wantTimeout {
				require.ErrorIs(t, r.Err, context.DeadlineExceeded)
				require.Less(t, time.Since(start), 500*time.Millisecond)
			} else {
				require.NoError(t, r.Err)
				require.True(t, cm.Connected())
			}
		})
	}
}
