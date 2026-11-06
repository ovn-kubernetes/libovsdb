package client

// Adapted from the TCP-proxy reproduction in
// https://github.com/ovn-kubernetes/libovsdb/pull/459 (also carried by #460).
// Unlike the original, proxy workers are joined, timeouts fail the test, and a
// bounded version runs in short mode rather than skipping race-detector CI.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/stretchr/testify/require"
)

type disconnectProxy struct {
	listener net.Listener
	target   string
	mu       sync.Mutex
	conns    map[net.Conn]struct{}
	accepted chan struct{}
	workers  sync.WaitGroup
}

func newDisconnectProxy(t *testing.T, target string) *disconnectProxy {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	p := &disconnectProxy{listener: listener, target: target, conns: make(map[net.Conn]struct{}), accepted: make(chan struct{})}
	go func() {
		defer close(p.accepted)
		for {
			client, err := listener.Accept()
			if err != nil {
				return
			}
			server, err := net.DialTimeout("unix", target, time.Second)
			if err != nil {
				client.Close()
				continue
			}
			p.mu.Lock()
			p.conns[client] = struct{}{}
			p.conns[server] = struct{}{}
			p.mu.Unlock()
			p.workers.Add(2)
			forward := func(dst, src net.Conn) {
				defer p.workers.Done()
				// Forced disconnects are expected to interrupt forwarding.
				_, _ = io.Copy(dst, src)
				dst.Close()
				src.Close()
				p.mu.Lock()
				delete(p.conns, src)
				delete(p.conns, dst)
				p.mu.Unlock()
			}
			go forward(client, server)
			go forward(server, client)
		}
	}()
	return p
}
func (p *disconnectProxy) disconnect() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for conn := range p.conns {
		conn.Close()
	}
}
func (p *disconnectProxy) close() {
	p.listener.Close()
	<-p.accepted // no more worker Add calls
	p.disconnect()
	p.workers.Wait()
}

func TestTransactDuringAbruptDisconnectNoPanic(t *testing.T) {
	var defSchema ovsdb.DatabaseSchema
	require.NoError(t, json.Unmarshal([]byte(schema), &defSchema))
	iterations := 100
	if testing.Short() {
		iterations = 10
	}
	for i := 0; i < iterations; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			server, sock := newOVSDBServer(t, defDB, defSchema)
			defer server.Close()
			proxy := newDisconnectProxy(t, sock)
			defer proxy.close()
			o, err := newOVSDBClient(defDB, WithEndpoint("tcp:"+proxy.listener.Addr().String()), WithInactivityCheck(200*time.Millisecond, 100*time.Millisecond, backoff.NewConstantBackOff(time.Millisecond)))
			require.NoError(t, err)
			defer o.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			require.NoError(t, o.Connect(ctx))
			stop := make(chan struct{})
			var workers sync.WaitGroup
			for n := 0; n < 16; n++ {
				workers.Add(1)
				go func() {
					defer workers.Done()
					for {
						select {
						case <-stop:
							return
						default:
						}
						ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
						_, _ = o.Transact(ctx, ovsdb.Operation{Op: ovsdb.OperationSelect, Table: "Bridge"})
						cancel()
					}
				}()
			}
			for n := 0; n < 4; n++ {
				workers.Add(1)
				go func() {
					defer workers.Done()
					ticker := time.NewTicker(time.Millisecond)
					defer ticker.Stop()
					for {
						select {
						case <-stop:
							return
						case <-ticker.C:
							proxy.disconnect()
						}
					}
				}()
			}
			time.Sleep(100 * time.Millisecond)
			close(stop)
			joined := make(chan struct{})
			go func() { workers.Wait(); close(joined) }()
			select {
			case <-joined:
			case <-time.After(3 * time.Second):
				t.Fatal("transaction/disconnect workers failed to stop")
			}
			require.Eventually(t, o.Connected, 2*time.Second, time.Millisecond)
			ctx, cancel = context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			require.NoError(t, o.Echo(ctx), "client must remain usable after repeated abrupt disconnects")
		})
	}
}
