package main

import (
	"context"
	"log"
	"os"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/ovn-kubernetes/libovsdb/client"
	"github.com/ovn-kubernetes/libovsdb/example/vswitchd"
	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
)

// OVSDB connection information
const (
	ovsdbEndpoint = "unix:/var/run/openvswitch/ovsdb.sock" // Modify based on your environment
	ovsTable      = "Open_vSwitch"
)

func main() {
	ctx := context.Background()

	// Setup logger
	stdr.SetVerbosity(1)
	logger := stdr.New(log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile)).WithName("ovsdb-select-example")

	// In this simplified example, we only need the Open_vSwitch model
	clientDBModel, err := model.NewClientDBModel("Open_vSwitch",
		map[string]model.Model{ovsTable: &vswitchd.OpenvSwitch{}})
	if err != nil {
		log.Fatal("Unable to create DB model ", err)
	}

	ovs, err := client.NewOVSDBClient(clientDBModel, client.WithEndpoint(ovsdbEndpoint))
	if err != nil {
		logger.Error(err, "Failed to create OVSDB client")
		os.Exit(1)
	}

	err = ovs.Connect(ctx)
	if err != nil {
		logger.Error(err, "Failed to connect to OVSDB")
		os.Exit(1)
	}
	defer ovs.Disconnect()

	logger.Info("Successfully connected to OVSDB", "endpoint", ovs.CurrentEndpoint())

	// This simplified example demonstrates how to select all rows from the Open_vSwitch table.
	// This table is guaranteed to exist and have exactly one row.
	selectAndPrintOpenvSwitch(ctx, ovs, logger)

	logger.Info("Example finished successfully!")
}

func selectAndPrintOpenvSwitch(ctx context.Context, ovs client.Client, logger logr.Logger) {
	logger.Info("--- Selecting all rows from Open_vSwitch table ---")
	var ovsRows []vswitchd.OpenvSwitch

	// 1. Generate the Select operation for the entire table.
	//    Use Where(&Model{}) to specify the table for select-all.
	ops, queryID, err := ovs.Where(&vswitchd.OpenvSwitch{}).Select()
	if err != nil {
		logger.Error(err, "Failed to generate select operation")
		os.Exit(1)
	}

	// 2. Execute the operation using Transact
	results, err := ovs.Transact(ctx, ops...)
	if err != nil {
		logger.Error(err, "Transact failed")
		os.Exit(1)
	}

	// 3. Check transact results for errors
	opErrs, err := ovsdb.CheckOperationResults(results, ops)
	if err != nil {
		logger.Error(err, "Error in transact results", "op_errors", opErrs)
		os.Exit(1)
	}

	// 4. Parse the select results from the operation result
	err = ovs.GetSelectResults(ops, results, map[string]interface{}{queryID: &ovsRows})
	if err != nil {
		logger.Error(err, "Failed to parse select results")
		os.Exit(1)
	}

	if len(ovsRows) > 0 {
		logger.Info("Successfully selected from Open_vSwitch table", "count", len(ovsRows))
		// Log details of the first row
		firstRow := ovsRows[0]
		logger.Info("Open_vSwitch details", "uuid", firstRow.UUID, "db_version", firstRow.DbVersion, "ovs_version", firstRow.OVSVersion)
	} else {
		// This should not happen in a standard OVS deployment
		logger.Info("No rows found in Open_vSwitch table. This is unexpected.")
	}
}
