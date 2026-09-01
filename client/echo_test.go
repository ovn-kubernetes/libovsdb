package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/stretchr/testify/require"
)

// TestEchoRace reproduces the race condition in Echo() where the function
// falls through after CallWithContext error and reads reply while the RPC
// readLoop might still be writing to it.
func TestEchoRace(t *testing.T) {
	var defSchema ovsdb.DatabaseSchema
	err := json.Unmarshal([]byte(schema), &defSchema)
	require.NoError(t, err)

	server, sock := newOVSDBServer(t, defDB, defSchema)
	defer server.Close()

	endpoint := fmt.Sprintf("unix:%s", sock)

	client, err := NewOVSDBClient(defDB,
		WithEndpoint(endpoint),
		WithInactivityCheck(50*time.Millisecond, 25*time.Millisecond, backoff.NewConstantBackOff(time.Millisecond)),
	)
	require.NoError(t, err)

	err = client.Connect(context.Background())
	require.NoError(t, err)
	defer client.Close()

	echoGoroutines, echoIter, reconnectGoroutines, reconnectIter := 50, 1000, 10, 100
	if testing.Short() {
		echoGoroutines, echoIter, reconnectGoroutines, reconnectIter = 10, 100, 3, 20
	}

	var wg sync.WaitGroup

	// Launch goroutines that continuously call Echo
	for i := 0; i < echoGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < echoIter; j++ {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
				_ = client.Echo(ctx)
				cancel()
			}
		}()
	}

	// Concurrently disconnect/reconnect to trigger errors
	for i := 0; i < reconnectGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < reconnectIter; j++ {
				client.Disconnect()
				time.Sleep(time.Millisecond)
				_ = client.Connect(context.Background())
				time.Sleep(time.Millisecond)
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("Test timeout")
	}
}
