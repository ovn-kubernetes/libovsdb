package server

import (
	"encoding/json"
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/ovn-kubernetes/libovsdb/database/inmemory"
	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/stretchr/testify/require"
)

// Close must close the connections of connected clients, as a stopping
// ovsdb-server does, and not only stop accepting new ones.
func TestCloseClosesClientConnections(t *testing.T) {
	logger := logr.Discard()
	db := inmemory.NewDatabase(map[string]model.ClientDBModel{}, &logger)
	s, err := NewOvsdbServer(db, &logger)
	require.NoError(t, err)
	sock := filepath.Join(t.TempDir(), "db.sock")
	go func() {
		if err := s.Serve("unix", sock); err != nil {
			t.Error(err)
		}
	}()
	require.Eventually(t, s.Ready, time.Second, 10*time.Millisecond)

	conn, err := net.Dial("unix", sock)
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
