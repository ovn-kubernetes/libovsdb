package inmemory

import (
	"context"
	"testing"

	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/ovn-kubernetes/libovsdb/test"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTransactionTelemetry(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previous)
		_ = provider.Shutdown(context.Background())
	})
	ctx, parent := provider.Tracer("test").Start(context.Background(), "parent")
	defer parent.End()
	dbModel, err := test.GetModel()
	require.NoError(t, err)
	db := NewDatabase(map[string]model.ClientDBModel{"Open_vSwitch": dbModel.Client()}, nil)
	require.NoError(t, db.CreateDatabase(ctx, "Open_vSwitch", dbModel.Schema))
	timeout, durable, comment, lock := 0, false, "test", "test"
	for _, name := range []string{"Insert", "Select", "Update", "Mutate", "Delete", "Wait", "Commit", "Abort", "Comment", "Assert"} {
		t.Run(name, func(t *testing.T) {
			op := ovsdb.Operation{Table: "Open_vSwitch", Row: ovsdb.Row{}, Timeout: &timeout, Until: "==", Durable: &durable, Comment: &comment, Lock: &lock}
			switch name {
			case "Insert":
				op.Op = ovsdb.OperationInsert
			case "Select":
				op.Op = ovsdb.OperationSelect
			case "Update":
				op.Op = ovsdb.OperationUpdate
			case "Mutate":
				op.Op = ovsdb.OperationMutate
			case "Delete":
				op.Op = ovsdb.OperationDelete
			case "Wait":
				op.Op = ovsdb.OperationWait
			case "Commit":
				op.Op = ovsdb.OperationCommit
			case "Abort":
				op.Op = ovsdb.OperationAbort
			case "Comment":
				op.Op = ovsdb.OperationComment
			case "Assert":
				op.Op = ovsdb.OperationAssert
			}
			before := len(recorder.Ended())
			txn := db.NewTransaction(ctx, "Open_vSwitch")
			_, _ = txn.Transact(ctx, op)
			spans := recorder.Ended()[before:]
			var transactionID string
			for _, span := range spans {
				require.Equal(t, parent.SpanContext().TraceID(), span.SpanContext().TraceID(), span.Name())
				if span.Name() == "Transact" {
					transactionID = span.SpanContext().SpanID().String()
					require.Equal(t, parent.SpanContext().SpanID(), span.Parent().SpanID())
				}
			}
			require.NotEmpty(t, transactionID)
			found := false
			for _, span := range spans {
				if span.Name() == name {
					found = true
					require.Equal(t, transactionID, span.Parent().SpanID().String())
				}
			}
			require.True(t, found, "missing operation span %s", name)
		})
	}
}
