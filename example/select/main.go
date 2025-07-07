package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/go-logr/stdr"
	"github.com/ovn-kubernetes/libovsdb/client"
	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/ovn-kubernetes/libovsdb/ovsdb/serverdb"
)

// OVSDB connection information
const (
	ovsdbEndpoint = "unix:/var/run/openvswitch/ovsdb.sock" // Modify based on your environment
	dbName        = "Open_vSwitch"
	tableName     = "Bridge"
)

// --- Bridge model definition for SelectModels ---
type Bridge struct {
	UUID        string            `ovsdb:"_uuid"`
	Name        string            `ovsdb:"name"`
	Ports       []string          `ovsdb:"ports"` // Simplified: stores port UUIDs as strings
	ExternalIDs map[string]string `ovsdb:"external_ids"`
	OtherConfig map[string]string `ovsdb:"other_config"`
	FailMode    *string           `ovsdb:"fail_mode"` // Example of optional field
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Setup logger
	stdr.SetVerbosity(1)
	logger := stdr.New(log.New(os.Stderr, "", log.LstdFlags)).WithName("ovsdb-select-example")

	// Connect to OVSDB server
	dbModel, err := serverdb.FullDatabaseModel()
	if err != nil {
		logger.Error(err, "Failed to get serverdb model")
		os.Exit(1)
	}
	ovs, err := client.NewOVSDBClient(dbModel, client.WithEndpoint(ovsdbEndpoint), client.WithLogger(&logger))
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

	logger.Info("=== ConditionalAPI.Select Examples - Using WhereXxx, Select, Transact, and GetSelectResults ===")

	// The client instance (`ovs`) itself implements the API interface.

	// --- Example 1: Select all bridges (mapped to structs) ---
	logger.Info("--- Example 1: Selecting all bridges (mapped to structs) ---")
	var allBridges []Bridge
	// 1. Generate the Select operation.
	//    Use Where(&Model{}) to specify the table for select-all.
	selectOpAll, err := ovs.Where(&Bridge{}).Select() // Use Where().Select() for select all
	if err != nil {
		logger.Error(err, "Failed to generate select all operation")
		os.Exit(1)
	}

	// 2. Execute the operation using Transact
	reply, err := ovs.Transact(ctx, selectOpAll...)
	if err != nil {
		logger.Error(err, "Transact failed for select all")
		os.Exit(1)
	}

	// 3. Check transact results for errors
	opErrs, err := ovsdb.CheckOperationResults(reply, selectOpAll)
	if err != nil {
		logger.Error(err, "Error in transact results for select all", "op_errors", opErrs)
		os.Exit(1)
	}

	// 4. Parse the select results from the first operation result
	err = ovs.GetSelectResults(reply, &allBridges)
	if err != nil {
		logger.Error(err, "Failed to parse select all results")
		os.Exit(1)
	}

	logger.Info("Found all bridges", "count", len(allBridges))
	for _, br := range allBridges {
		logger.Info("Bridge details", "uuid", br.UUID, "name", br.Name, "port_count", len(br.Ports), "fail_mode", br.FailMode)
	}

	// --- Example 2: Select a specific bridge by name (using model condition) ---
	bridgeNameToFind := "br-int" // Change to a bridge name that exists in your environment
	logger.Info("--- Example 2: Selecting bridge by name (using model condition) ---", "name", bridgeNameToFind)
	var specificBridges []Bridge

	// Create condition using model field pointer
	cond := model.Condition{
		Field:    &(&Bridge{}).Name, // Pointer to field in a zero-value struct
		Function: ovsdb.ConditionEqual,
		Value:    bridgeNameToFind,
	}

	// 1. Generate the Select operation using WhereAll
	selectOpSpecific, err := ovs.WhereAll(&Bridge{}, cond).Select() // Chain Select after WhereAll
	if err != nil {
		logger.Error(err, "Failed to generate select specific operation")
		os.Exit(1)
	}

	// 2. Execute the operation using Transact
	replySpecific, err := ovs.Transact(ctx, selectOpSpecific...)
	if err != nil {
		logger.Error(err, "Transact failed for select specific")
		os.Exit(1)
	}

	// 3. Check transact results
	opErrsSpecific, err := ovsdb.CheckOperationResults(replySpecific, selectOpSpecific)
	if err != nil {
		logger.Error(err, "Error in transact results for select specific", "op_errors", opErrsSpecific)
		os.Exit(1)
	}

	// 4. Parse the select results
	err = ovs.GetSelectResults(replySpecific, &specificBridges)
	if err != nil {
		logger.Error(err, "Failed to parse select specific results")
		os.Exit(1)
	}

	if len(specificBridges) == 0 {
		logger.Info("Bridge not found", "name", bridgeNameToFind)
	} else {
		logger.Info("Found specific bridge(s)", "name", bridgeNameToFind, "count", len(specificBridges))
		for _, br := range specificBridges {
			logger.Info("Specific bridge details",
				"uuid", br.UUID,
				"name", br.Name,
				"ports", br.Ports,
				"external_ids", br.ExternalIDs,
				"other_config", br.OtherConfig,
				"fail_mode", br.FailMode)
		}
	}

	// --- Example 3: Select bridges with multiple conditions (name AND external_id value) ---
	// Note: Assumes a bridge with name bridgeNameToFind exists and has external_ids["key"] = "value"
	extIDKeyToFind := "some_key"     // Change to a key that exists
	extIDValueToFind := "some_value" // Change to the corresponding value
	logger.Info("--- Example 3: Selecting bridge by name AND external_id ---",
		"name", bridgeNameToFind, "external_key", extIDKeyToFind, "external_value", extIDValueToFind)
	var specificBridgesMultiCond []Bridge

	// Create conditions using model field pointers
	condMulti1 := model.Condition{
		Field:    &(&Bridge{}).Name,
		Function: ovsdb.ConditionEqual,
		Value:    bridgeNameToFind,
	}
	condMulti2 := model.Condition{
		Field:    &(&Bridge{}).ExternalIDs,
		Function: ovsdb.ConditionIncludes, // Checks if map contains the key-value pair
		Value:    map[string]string{extIDKeyToFind: extIDValueToFind},
	}

	// 1. Generate the Select operation using WhereAll with multiple conditions
	selectOpMulti, err := ovs.WhereAll(&Bridge{}, condMulti1, condMulti2).Select()
	if err != nil {
		logger.Error(err, "Failed to generate select multi-condition operation")
		os.Exit(1)
	}

	// 2. Execute the operation using Transact
	replyMulti, err := ovs.Transact(ctx, selectOpMulti...)
	if err != nil {
		logger.Error(err, "Transact failed for select multi-condition")
		os.Exit(1)
	}

	// 3. Check transact results
	opErrsMulti, err := ovsdb.CheckOperationResults(replyMulti, selectOpMulti)
	if err != nil {
		logger.Error(err, "Error in transact results for select multi-condition", "op_errors", opErrsMulti)
		os.Exit(1)
	}

	// 4. Parse the select results
	err = ovs.GetSelectResults(replyMulti, &specificBridgesMultiCond)
	if err != nil {
		logger.Error(err, "Failed to parse select multi-condition results")
		os.Exit(1)
	}

	if len(specificBridgesMultiCond) == 0 {
		logger.Info("Bridge not found matching multiple conditions",
			"name", bridgeNameToFind, "external_key", extIDKeyToFind, "external_value", extIDValueToFind)
	} else {
		logger.Info("Found bridge(s) matching multiple conditions", "count", len(specificBridgesMultiCond))
		for _, br := range specificBridgesMultiCond {
			logger.Info("Multi-condition bridge details",
				"uuid", br.UUID,
				"name", br.Name,
				"ports", br.Ports,
				"external_ids", br.ExternalIDs,
				"other_config", br.OtherConfig,
				"fail_mode", br.FailMode)
		}
	}

	// --- Example 4: Select specific columns of a bridge ---
	logger.Info("--- Example 4: Selecting specific columns for bridge ---", "name", bridgeNameToFind)
	var partialBridges []Bridge

	// 1. Generate the Select operation using WhereAll and specifying columns in Select()
	//    Only "name" and "fail_mode" will be populated in the result. _uuid is always included.
	selectOpPartial, err := ovs.WhereAll(&Bridge{}, cond).Select("name", "fail_mode")
	if err != nil {
		logger.Error(err, "Failed to generate select partial column operation")
		os.Exit(1)
	}

	// 2. Execute the operation
	replyPartial, err := ovs.Transact(ctx, selectOpPartial...)
	if err != nil {
		logger.Error(err, "Transact failed for select partial column")
		os.Exit(1)
	}

	// 3. Check transact results
	opErrsPartial, err := ovsdb.CheckOperationResults(replyPartial, selectOpPartial)
	if err != nil {
		logger.Error(err, "Error in transact results for select partial column", "op_errors", opErrsPartial)
		os.Exit(1)
	}

	// 4. Parse the select results
	err = ovs.GetSelectResults(replyPartial, &partialBridges)
	if err != nil {
		logger.Error(err, "Failed to parse select partial column results")
		os.Exit(1)
	}

	if len(partialBridges) == 0 {
		logger.Info("Bridge not found for partial select", "name", bridgeNameToFind)
	} else {
		logger.Info("Found bridge(s) with partial columns", "name", bridgeNameToFind, "count", len(partialBridges))
		for _, br := range partialBridges {
			logger.Info("Partial bridge details",
				"uuid", br.UUID, // _uuid is always selected
				"name", br.Name, // was selected
				"fail_mode", br.FailMode, // was selected
				"ports (should be empty)", br.Ports, // not selected, so it will be the zero-value
				"external_ids (should be empty)", br.ExternalIDs) // not selected, so it will be the zero-value
		}
	}

	logger.Info("Example finished successfully!")
}
