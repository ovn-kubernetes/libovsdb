//go:build concurrency
// +build concurrency

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/stretchr/testify/require"
)

func TestInactivityTimeoutDeadlock(t *testing.T) {
	var defSchema ovsdb.DatabaseSchema
	err := json.Unmarshal([]byte(schema), &defSchema)
	require.NoError(t, err)

	server, sock := newOVSDBServer(t, defDB, defSchema)
	server.SetTransactionDelay(200 * time.Millisecond)
	server.SetEchoDelay(100 * time.Millisecond)

	endpoint := fmt.Sprintf("unix:%s", sock)
	client, err := NewOVSDBClient(defDB,
		WithEndpoint(endpoint),
		WithInactivityCheck(30*time.Millisecond, 20*time.Millisecond, backoff.NewConstantBackOff(time.Millisecond)),
	)
	require.NoError(t, err)

	err = client.Connect(context.Background())
	require.NoError(t, err)
	defer client.Close()

	_, err = client.MonitorAll(context.Background())
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	const numConcurrentTxns = 10
	for i := range numConcurrentTxns {
		go func(id int) {
			bridge := &Bridge{
				UUID:        fmt.Sprintf("deadlock-test-bridge-%d", id),
				Name:        fmt.Sprintf("br-deadlock-%d", id),
				ExternalIDs: map[string]string{"test": "deadlock"},
			}

			ops, err := client.Create(bridge)
			if err != nil {
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			client.Transact(ctx, ops...)
		}(i)
	}

	select {}
}
