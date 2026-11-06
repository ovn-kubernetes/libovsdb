// Adapted from https://github.com/ovn-kubernetes/libovsdb/pull/499.
package server

import (
	"encoding/json"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/ovn-kubernetes/libovsdb/database/inmemory"
	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/stretchr/testify/require"
)

func TestCloseBeforeServe(t *testing.T) {
	logger := logr.Discard()
	db := inmemory.NewDatabase(map[string]model.ClientDBModel{}, &logger)
	s, err := NewOvsdbServer(db, &logger)
	require.NoError(t, err)
	s.Close()
	require.NoError(t, s.Serve("tcp", "127.0.0.1:0"))
	require.False(t, s.Ready())
	s.Close()
}

func TestCloseConcurrentWithAccept(t *testing.T) {
	logger := logr.Discard()
	db := inmemory.NewDatabase(map[string]model.ClientDBModel{}, &logger)
	s, err := NewOvsdbServer(db, &logger)
	require.NoError(t, err)
	t.Cleanup(s.Close)
	served := make(chan error, 1)
	go func() { served <- s.Serve("tcp", "127.0.0.1:0") }()
	require.Eventually(t, s.Ready, time.Second, time.Millisecond)
	addr := s.listener.Addr().String()
	var workers sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 20; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			conn, err := net.DialTimeout("tcp", addr, time.Second)
			if err != nil {
				return
			}
			defer conn.Close()
			// Close races every stage of this exchange; transport errors are expected.
			if err := conn.SetDeadline(time.Now().Add(time.Second)); err != nil {
				return
			}
			if err := json.NewEncoder(conn).Encode(map[string]any{"method": "list_dbs", "params": []any{}, "id": 1}); err != nil {
				return
			}
			var reply any
			_ = json.NewDecoder(conn).Decode(&reply)
		}()
	}
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func() { defer workers.Done(); <-start; s.Close() }()
	}
	close(start)
	workers.Wait()
	require.NoError(t, <-served)
	require.False(t, s.Ready())
	require.Eventually(t, func() bool { s.readyMutex.RLock(); defer s.readyMutex.RUnlock(); return len(s.conns) == 0 }, time.Second, time.Millisecond)
}

// Close must close the connections of connected clients, as a stopping
// ovsdb-server does, and not only stop accepting new ones.
func TestCloseClosesClientConnections(t *testing.T) {
	logger := logr.Discard()
	db := inmemory.NewDatabase(map[string]model.ClientDBModel{}, &logger)
	s, err := NewOvsdbServer(db, &logger)
	require.NoError(t, err)
	t.Cleanup(s.Close)
	go func() {
		if err := s.Serve("tcp", "127.0.0.1:0"); err != nil {
			t.Error(err)
		}
	}()
	require.Eventually(t, s.Ready, time.Second, 10*time.Millisecond)

	conn, err := net.Dial("tcp", s.listener.Addr().String())
	require.NoError(t, err)
	defer conn.Close()

	// Wait for a reply, so that the server is serving this connection
	// rather than holding it in the listen backlog.
	require.NoError(t, json.NewEncoder(conn).Encode(map[string]any{"method": "list_dbs", "params": []any{}, "id": 1}))
	var reply map[string]any
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	dec := json.NewDecoder(conn)
	require.NoError(t, dec.Decode(&reply))

	s.Close()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	err = dec.Decode(&reply)
	require.ErrorIs(t, err, io.EOF, "the client connection is still open after Close")
}
