package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/ovn-kubernetes/libovsdb/cache"
	"github.com/ovn-kubernetes/libovsdb/client"
	"github.com/ovn-kubernetes/libovsdb/example/vswitchd"
	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
)

// Silly game that detects creation of Bridge named "stop" and exits
// Just a demonstration of how an app can use libovsdb library to configure and manage OVS
const (
	bridgeTable = "Bridge"
	ovsTable    = "Open_vSwitch"
)

var quit chan bool
var update chan model.Model

var rootUUID string
var connection = flag.String("ovsdb", "unix:/var/run/openvswitch/db.sock", "OVSDB connection string")

func play(ovs client.Client) {
	go processInput(ovs)
	for model := range update {
		bridge := model.(*vswitchd.Bridge)
		if bridge.Name == "stop" {
			fmt.Printf("Bridge stop detected: %+v\n", *bridge)
			ovs.Disconnect()
			quit <- true
		} else {
			fmt.Printf("Current list of bridges:\n")
			var bridges []vswitchd.Bridge
			if err := ovs.List(context.Background(), &bridges); err != nil {
				log.Fatal(err)
			}
			for _, b := range bridges {
				fmt.Printf("UUID: %s  Name: %s\n", b.UUID, b.Name)
			}
		}
	}
}

func createBridge(ovs client.Client, bridgeName string) {
	bridge := vswitchd.Bridge{
		UUID: "gopher",
		Name: bridgeName,
	}
	insertOp, err := ovs.Create(&bridge)
	if err != nil {
		log.Fatal(err)
	}

	ovsRow := vswitchd.OpenvSwitch{
		UUID: rootUUID,
	}
	mutateOps, err := ovs.Where(&ovsRow).Mutate(&ovsRow, model.Mutation{
		Field:   &ovsRow.Bridges,
		Mutator: "insert",
		Value:   []string{bridge.UUID},
	})
	if err != nil {
		log.Fatal(err)
	}

	operations := append(insertOp, mutateOps...)
	reply, err := ovs.Transact(context.TODO(), operations...)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := ovsdb.CheckOperationResults(reply, operations); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Bridge Addition Successful : ", reply[0].UUID.GoUUID)
}

func processInput(ovs client.Client) {
	for {
		fmt.Printf("\n Enter a Bridge Name : ")
		var bridgeName string
		_, err := fmt.Scanf("%s", &bridgeName)
		if err != nil {
			log.Fatal("Error reading input: ", err)
		}
		if bridgeName == "" {
			continue
		}
		createBridge(ovs, bridgeName)
	}
}

func main() {
	flag.Parse()
	quit = make(chan bool)
	update = make(chan model.Model)

	clientDBModel, err := model.NewClientDBModel("Open_vSwitch",
		map[string]model.Model{bridgeTable: &vswitchd.Bridge{}, ovsTable: &vswitchd.OpenvSwitch{}})
	if err != nil {
		log.Fatal("Unable to create DB model ", err)
	}

	ovs, err := client.NewOVSDBClient(clientDBModel, client.WithEndpoint(*connection))
	if err != nil {
		log.Fatal(err)
	}
	err = ovs.Connect(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	defer ovs.Disconnect()

	ovs.Cache().AddEventHandler(&cache.EventHandlerFuncs{
		AddFunc: func(table string, model model.Model) {
			if table == bridgeTable {
				update <- model
			}
		},
	})
	_, err = ovs.Monitor(
		context.TODO(),
		ovs.NewMonitor(
			client.WithTable(&vswitchd.OpenvSwitch{}),
			client.WithTable(&vswitchd.Bridge{}),
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Get root UUID
	for uuid := range ovs.Cache().Table("Open_vSwitch").Rows() {
		rootUUID = uuid
	}

	fmt.Println(`Silly game of stopping this app when a Bridge with name "stop" is monitored !`)
	go play(ovs)
	<-quit
}
