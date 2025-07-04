package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/ovn-kubernetes/libovsdb/cache"
	"github.com/ovn-kubernetes/libovsdb/client"
	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
)

// ORMBridge is the simplified ORM model of the Bridge table
type bridgeType struct {
	UUID        string            `ovsdb:"_uuid"`
	Name        string            `ovsdb:"name"`
	OtherConfig map[string]string `ovsdb:"other_config"`
	ExternalIDs map[string]string `ovsdb:"external_ids"`
	Ports       []string          `ovsdb:"ports"`
	Status      map[string]string `ovsdb:"status"`
}

// ORMovs is the simplified ORM model of the Bridge table
type ovsType struct {
	UUID    string   `ovsdb:"_uuid"`
	Bridges []string `ovsdb:"bridges"`
}

var (
	cpuprofile    = flag.String("cpuprofile", "", "write cpu profile to this file")
	memprofile    = flag.String("memoryprofile", "", "write memory profile to this file")
	nins          = flag.Int("inserts", 100, "the number of insertions to make to the database (per client)")
	nclients      = flag.Int("clients", 1, "the number of clients to use")
	parallel      = flag.Bool("parallel", false, "run clients in parallel")
	verbose       = flag.Bool("verbose", false, "Be verbose")
	connection    = flag.String("ovsdb", "unix:/var/run/openvswitch/db.sock", "OVSDB connection string")
	clientDBModel model.ClientDBModel
)

type result struct {
	insertions   int
	deletions    int
	transactTime []time.Duration
	cacheTime    []time.Duration
}

func cleanup(ctx context.Context) {
	ovs, err := client.NewOVSDBClient(clientDBModel, client.WithEndpoint(*connection))
	if err != nil {
		log.Fatal(err)
	}
	err = ovs.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer ovs.Disconnect()

	if _, err := ovs.MonitorAll(ctx); err != nil {
		log.Fatal(err)
	}

	var rootUUID string
	// Get root UUID
	for uuid := range ovs.Cache().Table("Open_vSwitch").Rows() {
		rootUUID = uuid
		log.Printf("rootUUID is %v", rootUUID)
	}

	// Remove all existing bridges
	var bridges []bridgeType
	if err := ovs.List(context.Background(), &bridges); err == nil {
		log.Printf("%d existing bridges found", len(bridges))
		for _, bridge := range bridges {
			deleteBridge(ctx, ovs, rootUUID, &bridge)
		}
	} else {
		if err != client.ErrNotFound {
			log.Fatal(err)
		}
	}
}

func run(ctx context.Context, resultsChan chan result, wg *sync.WaitGroup) {
	defer wg.Done()

	result := result{}
	ready := false
	var rootUUID string

	ovs, err := client.NewOVSDBClient(clientDBModel, client.WithEndpoint(*connection))
	if err != nil {
		log.Fatal(err)
	}
	err = ovs.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer ovs.Disconnect()

	var bridges []bridgeType
	bridgeCh := make(map[string]chan bool)
	for i := 0; i < *nins; i++ {
		br := newBridge()
		bridges = append(bridges, br)
		bridgeCh[br.Name] = make(chan bool)
	}

	ovs.Cache().AddEventHandler(
		&cache.EventHandlerFuncs{
			AddFunc: func(table string, model model.Model) {
				if ready && table == "Bridge" {
					br := model.(*bridgeType)
					var ch chan bool
					var ok bool
					if ch, ok = bridgeCh[br.Name]; !ok {
						return
					}
					close(ch)
					result.insertions++
				}
			},
			DeleteFunc: func(table string, _ model.Model) {
				if table == "Bridge" {
					result.deletions++
				}
			},
		},
	)

	if _, err := ovs.MonitorAll(ctx); err != nil {
		log.Fatal(err)
	}

	// Get root UUID
	for uuid := range ovs.Cache().Table("Open_vSwitch").Rows() {
		rootUUID = uuid
		if *verbose {
			fmt.Printf("rootUUID is %v\n", rootUUID)
		}
	}

	ready = true
	cacheWg := sync.WaitGroup{}
	for i := 0; i < *nins; i++ {
		br := bridges[i]
		ch := bridgeCh[br.Name]
		log.Printf("create bridge: %s", br.Name)
		cacheWg.Add(1)
		go func(_ context.Context, ch chan bool) {
			defer cacheWg.Done()
			<-ch
		}(ctx, ch)
		createBridge(ctx, ovs, rootUUID, br)
	}
	cacheWg.Wait()
	resultsChan <- result
}

func transact(ctx context.Context, ovs client.Client, operations []ovsdb.Operation) (bool, string) {
	reply, err := ovs.Transact(ctx, operations...)
	if err != nil {
		return false, ""
	}
	if _, err := ovsdb.CheckOperationResults(reply, operations); err != nil {
		return false, ""
	}
	return true, reply[0].UUID.GoUUID
}

func deleteBridge(ctx context.Context, ovs client.Client, rootUUID string, bridge *bridgeType) {
	log.Printf("deleting bridge %s", bridge.Name)
	deleteOp, err := ovs.Where(bridge).Delete()
	if err != nil {
		log.Fatal(err)
	}
	ovsRow := ovsType{
		UUID: rootUUID,
	}
	mutateOp, err := ovs.Where(&ovsRow).Mutate(&ovsRow, model.Mutation{
		Field:   &ovsRow.Bridges,
		Mutator: ovsdb.MutateOperationDelete,
		Value:   []string{bridge.UUID},
	})
	if err != nil {
		log.Fatal(err)
	}
	operations := append(deleteOp, mutateOp...)
	_, _ = transact(ctx, ovs, operations)
}

func newBridge() bridgeType {
	return bridgeType{
		UUID: "gopher",
		Name: fmt.Sprintf("br-%s", uuid.NewString()),
		OtherConfig: map[string]string{
			"foo":  "bar",
			"fake": "config",
		},
		ExternalIDs: map[string]string{
			"key1": "val1",
			"key2": "val2",
		},
	}
}

func createBridge(ctx context.Context, ovs client.Client, rootUUID string, bridge bridgeType) {
	insertOp, err := ovs.Create(&bridge)
	if err != nil {
		log.Fatal(err)
	}
	ovsRow := ovsType{}
	mutateOp, err := ovs.Where(&ovsType{UUID: rootUUID}).Mutate(&ovsRow, model.Mutation{
		Field:   &ovsRow.Bridges,
		Mutator: ovsdb.MutateOperationInsert,
		Value:   []string{bridge.UUID},
	})
	if err != nil {
		log.Fatal(err)
	}

	operations := append(insertOp, mutateOp...)
	_, _ = transact(ctx, ovs, operations)
}
func main() {
	flag.Parse()
	ctx := context.Background()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal(err)
		}
		defer pprof.StopCPUProfile()
	}
	if !*verbose {
		log.SetOutput(io.Discard)
	}

	var err error
	clientDBModel, err = model.NewClientDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &ovsType{}, "Bridge": &bridgeType{}})
	if err != nil {
		log.Fatal(err)
	}

	cleanup(ctx)

	var wg sync.WaitGroup
	resultChan := make(chan result)
	results := make([]result, *nclients)
	go func() {
		for result := range resultChan {
			results = append(results, result)
		}
	}()

	for i := 0; i < *nclients; i++ {
		wg.Add(1)
		go run(ctx, resultChan, &wg)
		if !*parallel {
			wg.Wait()
		}
	}
	log.Print("waiting for clients to complete")
	// wait for all clients
	wg.Wait()
	// close the result channel to avoid leaking a goroutine
	close(resultChan)

	result := result{}
	for _, r := range results {
		result.insertions += r.insertions
		result.deletions += r.deletions
		result.transactTime = append(result.transactTime, r.transactTime...)
		result.cacheTime = append(result.cacheTime, r.cacheTime...)
	}

	fmt.Printf("\n\n\n")
	fmt.Printf("Summary:\n")
	fmt.Printf("\tTotal Insertions: %d\n", result.insertions)
	fmt.Printf("\tTotal Deletions: %d\n", result.deletions)

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		runtime.GC()
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal("could not write memory profile: ", err)
		}
	}
}
