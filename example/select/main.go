package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-logr/stdr"
	"github.com/ovn-org/libovsdb/client"
	"github.com/ovn-org/libovsdb/model"
	"github.com/ovn-org/libovsdb/ovsdb"
	"github.com/ovn-org/libovsdb/ovsdb/serverdb"
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

	// =================================================================
	// Part 1: Select API - Direct query for raw row data
	// =================================================================
	logger.Info("=== Select API Examples - Returns raw OVSDB rows ===")

	// --- Example 1: Select all bridges and get their names and ports ---
	logger.Info("--- Example 1: Selecting all bridges (name, ports) ---")
	whereAll := []ovsdb.Condition{}            // Empty condition means select all rows
	columnsBridge := []string{"name", "ports"} // Specify columns to query

	bridgeRows, err := ovs.Select(ctx, dbName, tableName, whereAll, columnsBridge)
	if err != nil {
		logger.Error(err, "Select query failed")
		os.Exit(1)
	}

	logger.Info("Query results", "bridge_count", len(bridgeRows))
	for i, row := range bridgeRows {
		bridgeName, ok := row["name"].(string)
		if !ok {
			logger.Info("Skipping row without valid name", "row_index", i)
			continue
		}

		// Manually parse port data (OvsSet type)
		portsData, ok := row["ports"]
		if !ok {
			logger.Info("Bridge has no ports field", "bridge", bridgeName)
			continue
		}
		portsSet, ok := portsData.(ovsdb.OvsSet)
		if !ok {
			logger.Info("Ports field is not an OvsSet type", "bridge", bridgeName, "type", fmt.Sprintf("%T", portsData))
			continue
		}

		// Extract UUIDs from the set
		portUUIDs := make([]string, 0, len(portsSet.GoSet))
		for _, item := range portsSet.GoSet {
			if uuid, ok := item.(ovsdb.UUID); ok {
				portUUIDs = append(portUUIDs, uuid.GoUUID)
			} else {
				logger.Info("Non-UUID item found in ports set", "bridge", bridgeName, "item", item)
			}
		}
		logger.Info("Bridge details", "bridge", bridgeName, "ports", portUUIDs)
	}

	// --- Example 2: Select a specific bridge by name ---
	bridgeNameToFind := "br-int" // Change to a bridge name that exists in your environment
	logger.Info("--- Example 2: Selecting bridge by name ---", "name", bridgeNameToFind)
	whereSpecific := []ovsdb.Condition{
		{
			Column:   "name",
			Function: ovsdb.ConditionEqual,
			Value:    bridgeNameToFind,
		},
	}

	// Select all columns when columns is nil
	specificBridgeRows, err := ovs.Select(ctx, dbName, tableName, whereSpecific, nil)
	if err != nil {
		logger.Error(err, "Select query failed")
		os.Exit(1)
	}

	if len(specificBridgeRows) == 0 {
		logger.Info("Bridge not found", "name", bridgeNameToFind)
	} else {
		logger.Info("Found specific bridge", "name", bridgeNameToFind, "count", len(specificBridgeRows))
		for i, row := range specificBridgeRows {
			// Print all columns/fields in the row
			logger.Info("Bridge raw data", "row_index", i, "data", row)
		}
	}

	// --- Example 2.1: Select a specific bridge with multiple conditions (e.g., name AND fail_mode) ---
	// Note: This requires fail_mode to be set on the bridge in your environment.
	// You might need to run: ovs-vsctl set bridge br-int fail_mode=secure
	failModeToFind := "secure"
	logger.Info("--- Example 2.1: Selecting bridge by name AND fail_mode ---", "name", bridgeNameToFind, "fail_mode", failModeToFind)
	whereMulti := []ovsdb.Condition{
		{
			Column:   "name",
			Function: ovsdb.ConditionEqual,
			Value:    bridgeNameToFind,
		},
		{
			Column:   "fail_mode",
			Function: ovsdb.ConditionEqual,
			Value:    failModeToFind,
		},
	}

	multiCondBridgeRows, err := ovs.Select(ctx, dbName, tableName, whereMulti, nil)
	if err != nil {
		logger.Error(err, "Select query with multiple conditions failed")
		os.Exit(1)
	}

	if len(multiCondBridgeRows) == 0 {
		logger.Info("Bridge not found with specified name and fail_mode", "name", bridgeNameToFind, "fail_mode", failModeToFind)
	} else {
		logger.Info("Found bridge(s) matching multiple conditions", "count", len(multiCondBridgeRows))
		for i, row := range multiCondBridgeRows {
			logger.Info("Multi-condition bridge raw data", "row_index", i, "data", row)
		}
	}

	// =================================================================
	// Part 2: SelectModels API - Direct query to Go structs
	// =================================================================
	logger.Info("\n=== SelectModels API Examples - Automatic mapping to Go structs ===")

	// --- Example 3: Select all bridges (auto-mapped to structs) ---
	logger.Info("--- Example 3: Selecting all bridges (auto-mapped to structs) ---")
	var allBridges []Bridge
	err = ovs.SelectModels(ctx, &Bridge{}, &allBridges, []model.Condition{}...)
	if err != nil {
		logger.Error(err, "SelectModels query for all bridges failed")
		os.Exit(1)
	}
	logger.Info("Found all bridges", "count", len(allBridges))
	for _, br := range allBridges {
		logger.Info("Bridge details", "uuid", br.UUID, "name", br.Name, "port_count", len(br.Ports), "fail_mode", br.FailMode)
	}

	// --- Example 4: Select a specific bridge by name (using model condition) ---
	logger.Info("--- Example 4: Selecting bridge by name (using model condition) ---", "name", bridgeNameToFind)
	var specificBridges []Bridge

	// Create condition using model field pointer
	cond := model.Condition{
		Field:    &(&Bridge{}).Name, // Pointer to field in a zero-value struct
		Function: ovsdb.ConditionEqual,
		Value:    bridgeNameToFind,
	}

	err = ovs.SelectModels(ctx, &specificBridges, &Bridge{}, cond)
	if err != nil {
		logger.Error(err, "SelectModels query for specific bridge failed", "name", bridgeNameToFind)
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

	// --- Example 4.1: Select bridges with multiple conditions (name AND external_id value) ---
	// Note: Assumes a bridge with name bridgeNameToFind exists and has external_ids["key"] = "value"
	extIDKeyToFind := "some_key"     // Change to a key that exists
	extIDValueToFind := "some_value" // Change to the corresponding value
	logger.Info("--- Example 4.1: Selecting bridge by name AND external_id ---",
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

	err = ovs.SelectModels(ctx, &specificBridgesMultiCond, &Bridge{}, condMulti1, condMulti2)
	if err != nil {
		logger.Error(err, "SelectModels query with multiple conditions failed")
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
				"external_ids", br.ExternalIDs)
		}
	}

	// --- Example 5: Select bridges with a specific external ID key ---
	externalIDKey := "bridge-id" // Change as needed
	logger.Info("--- Example 5: Selecting bridges by external_ids key ---", "key", externalIDKey)
	var bridgesWithExtID []Bridge

	// Create map includes condition
	condExtID := model.Condition{
		Field:    &(&Bridge{}).ExternalIDs, // Pointer to map field
		Function: ovsdb.ConditionIncludes,
		// Value for includes on a map must be a map[string]string containing key(s) to check
		Value: map[string]string{externalIDKey: ""}, // Value associated with key doesn't matter for 'includes' key check
	}

	err = ovs.SelectModels(ctx, &bridgesWithExtID, &Bridge{}, condExtID)
	if err != nil {
		logger.Error(err, "SelectModels query for external_id key failed", "key", externalIDKey)
		os.Exit(1)
	}

	logger.Info("Found bridges with external ID key", "key", externalIDKey, "count", len(bridgesWithExtID))
	for _, br := range bridgesWithExtID {
		logger.Info("Bridge details (extID)", "uuid", br.UUID, "name", br.Name, "external_ids", br.ExternalIDs)
	}

	logger.Info("Example program complete")
}
