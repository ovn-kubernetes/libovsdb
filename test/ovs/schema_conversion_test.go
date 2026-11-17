package ovs

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/ovn-kubernetes/libovsdb/client"
	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/stretchr/testify/require"
)

// conversionWireMessage observes JSON-RPC without changing its contents. In
// particular, this test must not manufacture found=false or reset transaction IDs.
type conversionWireMessage struct {
	Method string            `json:"method"`
	Params []json.RawMessage `json:"params"`
	Result json.RawMessage   `json:"result"`
}

func conversionProxy(t *testing.T, upstream string) (string, <-chan conversionWireMessage) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	messages := make(chan conversionWireMessage, 128)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			downstream, err := listener.Accept()
			if err != nil {
				return
			}
			server, err := net.DialTimeout("tcp", upstream, time.Second)
			if err != nil {
				downstream.Close()
				continue
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer downstream.Close()
				defer server.Close()
				done := make(chan struct{}, 2)
				forward := func(dst, src net.Conn) {
					defer func() { done <- struct{}{} }()
					decoder, encoder := json.NewDecoder(src), json.NewEncoder(dst)
					for {
						var raw json.RawMessage
						if decoder.Decode(&raw) != nil {
							return
						}
						var message conversionWireMessage
						if json.Unmarshal(raw, &message) != nil {
							return
						}
						select {
						case messages <- message:
						case <-stop:
							return
						}
						if encoder.Encode(raw) != nil {
							return
						}
					}
				}
				go forward(server, downstream)
				go forward(downstream, server)
				select {
				case <-stop:
				case <-done:
					// Put the completion back so both forwarding goroutines
					// are joined below, including on connection loss.
					done <- struct{}{}
				}
				server.Close()
				downstream.Close()
				<-done
				<-done
			}()
		}
	}()
	t.Cleanup(func() {
		close(stop)
		listener.Close()
		wg.Wait()
	})
	return "tcp:" + listener.Addr().String(), messages
}

func awaitConversionMessage(t *testing.T, messages <-chan conversionWireMessage, match func(conversionWireMessage) bool) conversionWireMessage {
	t.Helper()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for {
		select {
		case message := <-messages:
			if match(message) {
				return message
			}
		case <-timer.C:
			t.Fatal("timed out waiting for schema-conversion protocol message")
			return conversionWireMessage{}
		}
	}
}

func conversionMonitorReply(t *testing.T, messages <-chan conversionWireMessage) bool {
	t.Helper()
	var found bool
	awaitConversionMessage(t, messages, func(message conversionWireMessage) bool {
		var result []json.RawMessage
		return json.Unmarshal(message.Result, &result) == nil && len(result) == 3 && json.Unmarshal(result[0], &found) == nil
	})
	return found
}

type conversionRow struct {
	UUID    string `ovsdb:"_uuid"`
	Name    string `ovsdb:"name"`
	Removed string `ovsdb:"removed"`
	Added   string `ovsdb:"added"`
}

type conversionExtra struct {
	UUID string `ovsdb:"_uuid"`
	Name string `ovsdb:"name"`
}

// TestSchemaConversionRecovery exercises real ovsdb-server conversion, not the
// in-process server (which always returns found=false). No inactivity timeout or
// explicit disconnect is used to recover from conversion: the server must signal
// the schema change. The isolated database avoids disturbing the OVS suite.
func TestSchemaConversionRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real OVS container")
	}
	pool, err := dockertest.NewPool("")
	require.NoError(t, err)
	tag := os.Getenv("OVS_VERSION")
	if tag == "" {
		tag = "latest"
	}
	const oldSchema = `{"name":"Conversion","version":"1.0.0","tables":{"Row":{"isRoot":true,"columns":{"name":{"type":"string"},"removed":{"type":"string"}}}}}`
	const newSchema = `{"name":"Conversion","version":"2.0.0","tables":{"Row":{"isRoot":true,"columns":{"name":{"type":"string"},"added":{"type":"string"}}},"Extra":{"isRoot":true,"columns":{"name":{"type":"string"}}}}}`
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "ghcr.io/ovn-kubernetes/ovs", Tag: tag,
		// Transaction IDs are available in clustered mode, not standalone
		// mode. A one-node cluster exercises real history without ovs-vswitchd.
		// Pass the schema as an argument rather than interpolating it.
		Entrypoint: []string{"sh", "-ec", `printf '%s' "$1" > /tmp/conversion.ovsschema
ovsdb-tool create-cluster /tmp/conversion.db /tmp/conversion.ovsschema tcp:127.0.0.1:6642
exec ovsdb-server --no-chdir --unixctl=/tmp/conversion.ctl --remote=ptcp:6641:0.0.0.0 /tmp/conversion.db`, "sh", oldSchema},
		ExposedPorts: []string{"6641/tcp"},
	}, func(config *docker.HostConfig) { config.AutoRemove = true })
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pool.Purge(resource)) })
	require.NoError(t, resource.Expire(120))
	upstream := resource.GetHostPort("6641/tcp")
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", upstream, 100*time.Millisecond)
		if err != nil {
			return false
		}
		conn.Close()
		return true
	}, 10*time.Second, 20*time.Millisecond)

	endpoint, messages := conversionProxy(t, resource.GetHostPort("6641/tcp"))
	clientModel, err := model.NewClientDBModel("Conversion", map[string]model.Model{"Row": &conversionRow{}, "Extra": &conversionExtra{}})
	require.NoError(t, err)
	cli, err := client.NewOVSDBClient(clientModel, client.WithEndpoint(endpoint), client.WithCompatibility(true), client.WithReconnect(time.Second, &backoff.ConstantBackOff{Interval: 20 * time.Millisecond}))
	require.NoError(t, err)
	t.Cleanup(cli.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	require.NoError(t, cli.Connect(ctx))
	_, err = cli.MonitorAll(ctx)
	require.NoError(t, err)
	require.False(t, conversionMonitorReply(t, messages))

	op := ovsdb.Operation{Op: ovsdb.OperationInsert, Table: "Row", Row: ovsdb.Row{"name": "survivor", "removed": "obsolete"}}
	results, err := cli.Transact(ctx, op)
	require.NoError(t, err)
	opErrs, err := ovsdb.CheckOperationResults(results, []ovsdb.Operation{op})
	require.NoError(t, err)
	require.Empty(t, opErrs)
	rowID := results[0].UUID.GoUUID
	update := awaitConversionMessage(t, messages, func(m conversionWireMessage) bool { return m.Method == "update3" })
	require.Len(t, update.Params, 3)
	var lastTxn string
	require.NoError(t, json.Unmarshal(update.Params[1], &lastTxn))
	require.NotEqual(t, "00000000-0000-0000-0000-000000000000", lastTxn)
	require.Eventually(t, func() bool { return cli.Cache().Table("Row").Row(rowID) != nil }, time.Second, 10*time.Millisecond)

	// Control: an ordinary reconnect actually resumes from the saved ID.
	cli.Disconnect()
	request := awaitConversionMessage(t, messages, func(m conversionWireMessage) bool { return m.Method == "monitor_cond_since" })
	require.Len(t, request.Params, 4)
	var requestedTxn string
	require.NoError(t, json.Unmarshal(request.Params[3], &requestedTxn))
	require.Equal(t, lastTxn, requestedTxn)
	require.True(t, conversionMonitorReply(t, messages))
	require.Eventually(t, cli.Connected, time.Second, 10*time.Millisecond)
	require.Equal(t, "obsolete", cli.Cache().Table("Row").Row(rowID).(*conversionRow).Removed)

	// Conversion cancels monitors and destroys transaction history. The client
	// must recover without the test forcing another disconnect or monitor call.
	admin, err := net.DialTimeout("tcp", upstream, time.Second)
	require.NoError(t, err)
	defer admin.Close()
	require.NoError(t, admin.SetDeadline(time.Now().Add(10*time.Second)))
	encoder, decoder := json.NewEncoder(admin), json.NewDecoder(admin)
	// Only the administrative connection opts into change awareness, to keep
	// the conversion reply readable. The client under test remains unmodified.
	for id, rpc := range []struct {
		method string
		params []any
	}{
		{"set_db_change_aware", []any{true}},
		{"convert", []any{"Conversion", json.RawMessage(newSchema)}},
	} {
		require.NoError(t, encoder.Encode(map[string]any{"method": rpc.method, "params": rpc.params, "id": id}))
		var reply struct {
			ID    int `json:"id"`
			Error any `json:"error"`
		}
		require.NoError(t, decoder.Decode(&reply))
		require.Equal(t, id, reply.ID)
		require.Nil(t, reply.Error)
	}
	request = awaitConversionMessage(t, messages, func(m conversionWireMessage) bool { return m.Method == "monitor_cond_since" })
	require.Len(t, request.Params, 4)
	require.NoError(t, json.Unmarshal(request.Params[3], &requestedTxn))
	require.Equal(t, lastTxn, requestedTxn, "prove the server rejects the pre-conversion history, rather than the client resetting it")
	require.False(t, conversionMonitorReply(t, messages))
	require.Eventually(t, func() bool {
		if !cli.Connected() || !cli.IsTableSupported(&conversionExtra{}) {
			return false
		}
		row := cli.Cache().Table("Row").Row(rowID)
		return row != nil && row.(*conversionRow).Name == "survivor" && row.(*conversionRow).Removed == ""
	}, 5*time.Second, 10*time.Millisecond)
	require.Empty(t, cli.Cache().Table("Row").Row(rowID).(*conversionRow).Added, "new columns receive their schema default")
	require.NotNil(t, cli.Cache().Table("Extra"))

	op = ovsdb.Operation{Op: ovsdb.OperationUpdate, Table: "Row", Where: []ovsdb.Condition{ovsdb.NewCondition("_uuid", ovsdb.ConditionEqual, ovsdb.UUID{GoUUID: rowID})}, Row: ovsdb.Row{"added": "populated"}}
	extra := ovsdb.Operation{Op: ovsdb.OperationInsert, Table: "Extra", Row: ovsdb.Row{"name": "new-table"}}
	results, err = cli.Transact(ctx, op, extra)
	require.NoError(t, err)
	opErrs, err = ovsdb.CheckOperationResults(results, []ovsdb.Operation{op, extra})
	require.NoError(t, err, "%+v", results)
	require.Empty(t, opErrs)
	extraID := results[1].UUID.GoUUID
	require.Eventually(t, func() bool {
		row := cli.Cache().Table("Row").Row(rowID).(*conversionRow)
		newRow := cli.Cache().Table("Extra").Row(extraID)
		return row.Added == "populated" && newRow != nil && newRow.(*conversionExtra).Name == "new-table"
	}, 5*time.Second, 10*time.Millisecond)
}
