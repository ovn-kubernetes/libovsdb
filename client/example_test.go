package client_test

import (
	"context"
	"fmt"
	"strings"

	"github.com/ovn-kubernetes/libovsdb/client"
	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
)

type logicalSwitch struct {
	UUID        string            `ovsdb:"_uuid"`
	Name        string            `ovsdb:"name"`
	Ports       []string          `ovsdb:"ports"`
	ExternalIDs map[string]string `ovsdb:"external_ids"`
}

// Connect to a server, monitor every table of the model and read from
// the cache that the monitor populates.
func ExampleNewOVSDBClient() {
	dbModel, err := model.NewClientDBModel("OVN_Northbound", map[string]model.Model{
		"Logical_Switch": &logicalSwitch{},
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	ovs, err := client.NewOVSDBClient(dbModel, client.WithEndpoint("tcp:127.0.0.1:6641"))
	if err != nil {
		fmt.Println(err)
		return
	}
	ctx := context.Background()
	if err := ovs.Connect(ctx); err != nil {
		fmt.Println(err)
		return
	}
	defer ovs.Disconnect()

	// The cache based API (List, Get and the Where* functions) only sees the
	// tables that are being monitored.
	if _, err := ovs.MonitorAll(ctx); err != nil {
		fmt.Println(err)
		return
	}

	var switches []*logicalSwitch
	err = ovs.WhereCache(func(ls *logicalSwitch) bool {
		return strings.HasPrefix(ls.Name, "ext_")
	}).List(ctx, &switches)
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, ls := range switches {
		fmt.Println(ls.Name)
	}
}

// Create a row and mutate another in a single transaction.
func ExampleClient_Transact() {
	var ovs client.Client // connected and monitoring, see ExampleNewOVSDBClient
	ctx := context.Background()

	create, err := ovs.Create(&logicalSwitch{Name: "foo"})
	if err != nil {
		fmt.Println(err)
		return
	}

	ls := &logicalSwitch{Name: "bar"}
	mutate, err := ovs.Where(ls).Mutate(ls, model.Mutation{
		Field:   &ls.ExternalIDs,
		Mutator: ovsdb.MutateOperationInsert,
		Value:   map[string]string{"owner": "me"},
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	ops := append(create, mutate...)
	results, err := ovs.Transact(ctx, ops...)
	if err != nil {
		fmt.Println(err)
		return
	}
	if _, err := ovsdb.CheckOperationResults(results, ops); err != nil {
		fmt.Println(err)
	}
}
