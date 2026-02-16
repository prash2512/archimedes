package engine

import (
	"math"
	"testing"

	"github.com/prashanth/archimedes/internal/blocks"
	_ "github.com/prashanth/archimedes/internal/blocks/cache"
	_ "github.com/prashanth/archimedes/internal/blocks/datastore"
	_ "github.com/prashanth/archimedes/internal/blocks/queue"
	_ "github.com/prashanth/archimedes/internal/blocks/search"
)

func approx(a, b float64) bool {
	return math.Abs(a-b) < 0.001
}

func TestSimulateServiceCPU(t *testing.T) {
	// Service: 2 cores, 0.5ms per op → saturates at 4000 RPS
	g := mustGraph(t, Topology{
		Blocks: []TopoBlock{{ID: "s", Kind: "service"}},
	})
	results, err := Simulate(g, 1000, 1.0)
	if err != nil {
		t.Fatal(err)
	}
	r := results[0]
	if !approx(r.CPUUtil, 0.25) {
		t.Errorf("cpu_util: want 0.25, got %f", r.CPUUtil)
	}
	if r.DiskUtil != 0 {
		t.Errorf("disk_util: want 0, got %f", r.DiskUtil)
	}
	if r.Health != "green" {
		t.Errorf("health: want green, got %s", r.Health)
	}
}

func TestSimulateSQLDisk(t *testing.T) {
	// SQL: 5000 IOPS, 2 read I/Os, 0.75 buffer pool → effective 0.5 I/Os per read
	// At 8000 RPS: disk = 8000 * 0.5 / 5000 = 0.8, cpu = 8000 * 0.5 / 4000 = 1.0
	g := mustGraph(t, Topology{
		Blocks: []TopoBlock{{ID: "db", Kind: "sql_datastore"}},
	})
	results, err := Simulate(g, 8000, 1.0)
	if err != nil {
		t.Fatal(err)
	}
	r := results[0]
	if !approx(r.DiskUtil, 0.8) {
		t.Errorf("disk_util: want 0.8, got %f", r.DiskUtil)
	}
	if !approx(r.CPUUtil, 1.0) {
		t.Errorf("cpu_util: want 1.0, got %f", r.CPUUtil)
	}
	if r.Health != "red" {
		t.Errorf("health: want red, got %s", r.Health)
	}
}

func TestSimulateChainPropagation(t *testing.T) {
	// User → Service → SQL: RPS flows through the chain
	g := mustGraph(t, Topology{
		Blocks: []TopoBlock{
			{ID: "u", Kind: "user"},
			{ID: "s", Kind: "service"},
			{ID: "db", Kind: "sql_datastore"},
		},
		Edges: []TopoEdge{
			{From: "u", To: "s"},
			{From: "s", To: "db"},
		},
	})
	results, err := Simulate(g, 500, 1.0)
	if err != nil {
		t.Fatal(err)
	}

	byID := map[string]BlockResult{}
	for _, r := range results {
		byID[r.ID] = r
	}

	if byID["u"].RPS != 500 {
		t.Errorf("user rps: want 500, got %f", byID["u"].RPS)
	}
	if byID["s"].RPS != 500 {
		t.Errorf("service rps: want 500, got %f", byID["s"].RPS)
	}
	if byID["db"].RPS != 500 {
		t.Errorf("db rps: want 500, got %f", byID["db"].RPS)
	}
}

func TestSimulateRedisHighRPS(t *testing.T) {
	// Redis: 1 core, 0.01ms per op → saturates at 100k RPS
	// At 80000 RPS: cpu = 80000 * 0.01 / 1000 = 0.8
	g := mustGraph(t, Topology{
		Blocks: []TopoBlock{{ID: "r", Kind: "redis"}},
	})
	results, err := Simulate(g, 80000, 1.0)
	if err != nil {
		t.Fatal(err)
	}
	r := results[0]
	if !approx(r.CPUUtil, 0.8) {
		t.Errorf("cpu_util: want 0.8, got %f", r.CPUUtil)
	}
	if r.Health != "yellow" {
		t.Errorf("health: want yellow, got %s", r.Health)
	}
}

func TestBlockCapacity(t *testing.T) {
	tests := []struct {
		kind string
		want float64
	}{
		{"service", 4000},
		{"load_balancer", 100000},
		{"api_gateway", 40000},
		{"redis", 100000},
		{"sql_datastore", 8000},  // read-limited by CPU: 4*1000/0.5
		{"kafka", 50000},         // sequential disk: 5000*10/1
		{"elasticsearch", 5000},  // disk: 5000/(2*0.5)
	}
	for _, tt := range tests {
		b, ok := blocks.ByKind(tt.kind)
		if !ok {
			t.Fatalf("unknown kind %s", tt.kind)
		}
		got := BlockCapacity(b.Profile(), 1.0)
		if !approx(got, tt.want) {
			t.Errorf("%s: want %g, got %g", tt.kind, tt.want, got)
		}
	}
}

func TestSimulateTickQueueBuilds(t *testing.T) {
	// Service capacity = 4000 RPS. Send 8000 → queue should grow each tick.
	g := mustGraph(t, Topology{
		Blocks: []TopoBlock{{ID: "s", Kind: "service"}},
	})
	state := NewSimState(g)

	var prev float64
	for i := 0; i < 5; i++ {
		SimulateTick(g, 8000, 1.0, state)
		q := state.Blocks["s"].Queue
		if q <= prev {
			t.Fatalf("tick %d: queue should grow, got %g (prev %g)", i+1, q, prev)
		}
		prev = q
	}
	if prev < 1000 {
		t.Errorf("queue after 5 overloaded ticks should be large, got %g", prev)
	}
}

func TestSimulateTickDrains(t *testing.T) {
	// Overload for 5 ticks, then drain with RPS=0
	g := mustGraph(t, Topology{
		Blocks: []TopoBlock{{ID: "s", Kind: "service"}},
	})
	state := NewSimState(g)

	for i := 0; i < 5; i++ {
		SimulateTick(g, 8000, 1.0, state)
	}
	peak := state.Blocks["s"].Queue
	// Drain: run enough ticks for queue to empty
	for i := 0; i < 20; i++ {
		SimulateTick(g, 0, 1.0, state)
	}
	q := state.Blocks["s"].Queue
	if q > 0.5 {
		t.Errorf("queue after drain: want ~0, got %g (peak was %g)", q, peak)
	}
}

func TestSimulateTickQueueAtHighUtil(t *testing.T) {
	// Service at 90% raw load (3600 RPS, capacity 4000) should queue
	// due to contention effects even though raw capacity isn't exceeded
	g := mustGraph(t, Topology{
		Blocks: []TopoBlock{{ID: "s", Kind: "service"}},
	})
	state := NewSimState(g)

	for i := 0; i < 10; i++ {
		SimulateTick(g, 3600, 1.0, state)
	}
	q := state.Blocks["s"].Queue
	if q < 1 {
		t.Errorf("expected queue buildup at 90%% util, got %g", q)
	}
}

func TestSimulateTickChainThroughput(t *testing.T) {
	// User → Service → SQL at 500 RPS (within capacity for both)
	// Service capacity=4000, SQL read capacity=8000
	// All requests should flow through, no queue buildup
	g := mustGraph(t, Topology{
		Blocks: []TopoBlock{
			{ID: "u", Kind: "user"},
			{ID: "s", Kind: "service"},
			{ID: "db", Kind: "sql_datastore"},
		},
		Edges: []TopoEdge{
			{From: "u", To: "s"},
			{From: "s", To: "db"},
		},
	})
	state := NewSimState(g)

	results, _ := SimulateTick(g, 500, 1.0, state)
	byID := map[string]BlockResult{}
	for _, r := range results {
		byID[r.ID] = r
	}

	if !approx(byID["s"].RPS, 500) {
		t.Errorf("service rps: want 500, got %g", byID["s"].RPS)
	}
	if !approx(byID["db"].RPS, 500) {
		t.Errorf("db rps: want 500, got %g", byID["db"].RPS)
	}
	if state.Blocks["s"].Queue > 0.5 {
		t.Errorf("service queue should be 0, got %g", state.Blocks["s"].Queue)
	}
}

func TestSQLWriteHeavyCapacity(t *testing.T) {
	// SQL at 100% reads: capacity = 8000 RPS (CPU-limited: 4*1000/0.5)
	// SQL at 100% writes: capacity = 500 RPS (disk-limited: 5000/10)
	// SQL at 70/30 read/write: disk becomes bottleneck from write amplification
	b, _ := blocks.ByKind("sql_datastore")
	p := b.Profile()

	allRead := BlockCapacity(p, 1.0)
	if !approx(allRead, 8000) {
		t.Errorf("all-read capacity: want 8000, got %g", allRead)
	}

	allWrite := BlockCapacity(p, 0.0)
	if !approx(allWrite, 500) {
		t.Errorf("all-write capacity: want 500, got %g", allWrite)
	}

	// 70/30: disk = reads: 2*0.25*0.7=0.35 + writes: 10*0.3=3.0 = 3.35 IOs
	// disk cap = 5000/3.35 ≈ 1492.5
	// cpu = 4000/(0.5*0.7+1.0*0.3) = 4000/0.65 ≈ 6153.8
	// bottleneck is disk at ~1493
	mixed := BlockCapacity(p, 0.7)
	if mixed > 1600 || mixed < 1400 {
		t.Errorf("70/30 capacity: want ~1493, got %g", mixed)
	}
}

func TestSQLWriteHeavyHealth(t *testing.T) {
	// SQL at 1000 RPS all-reads is green (capacity 8000)
	// SQL at 1000 RPS all-writes is red (capacity 500)
	g := mustGraph(t, Topology{
		Blocks: []TopoBlock{{ID: "db", Kind: "sql_datastore"}},
	})
	readResults, _ := Simulate(g, 1000, 1.0)
	writeResults, _ := Simulate(g, 1000, 0.0)

	if readResults[0].Health != "green" {
		t.Errorf("all-read at 1000 RPS: want green, got %s", readResults[0].Health)
	}
	if writeResults[0].Health != "red" {
		t.Errorf("all-write at 1000 RPS: want red, got %s", writeResults[0].Health)
	}
}

func TestKafkaSymmetric(t *testing.T) {
	// Kafka reads and writes have the same cost — capacity shouldn't change
	b, _ := blocks.ByKind("kafka")
	p := b.Profile()

	allRead := BlockCapacity(p, 1.0)
	allWrite := BlockCapacity(p, 0.0)
	mixed := BlockCapacity(p, 0.5)

	if !approx(allRead, allWrite) {
		t.Errorf("kafka should be symmetric: read=%g write=%g", allRead, allWrite)
	}
	if !approx(allRead, mixed) {
		t.Errorf("kafka should be symmetric: read=%g mixed=%g", allRead, mixed)
	}
}

func TestRedisSymmetric(t *testing.T) {
	// Redis reads and writes have the same cost — capacity shouldn't change
	b, _ := blocks.ByKind("redis")
	p := b.Profile()

	allRead := BlockCapacity(p, 1.0)
	allWrite := BlockCapacity(p, 0.0)

	if !approx(allRead, allWrite) {
		t.Errorf("redis should be symmetric: read=%g write=%g", allRead, allWrite)
	}
}

func TestSimulateTickWriteHeavy(t *testing.T) {
	// SQL at 1000 RPS all-writes should queue (capacity ~500)
	g := mustGraph(t, Topology{
		Blocks: []TopoBlock{{ID: "db", Kind: "sql_datastore"}},
	})
	state := NewSimState(g)

	for range 5 {
		SimulateTick(g, 1000, 0.0, state)
	}
	if state.Blocks["db"].Queue < 100 {
		t.Errorf("SQL at 1000 write RPS should queue, got %g", state.Blocks["db"].Queue)
	}
}

func TestSQLConnPoolWriteHeavy(t *testing.T) {
	// Write-heavy load fills the conn pool faster (12ms hold vs 2ms for reads).
	// At 5000 write RPS: write_conns = 5000*0.012 = 60, pool_util = 60%
	// At 5000 read RPS:  read_conns  = 5000*0.002 = 10, pool_util = 10%
	g := mustGraph(t, Topology{
		Blocks: []TopoBlock{{ID: "db", Kind: "sql_datastore"}},
	})

	writeState := NewSimState(g)
	readState := NewSimState(g)

	for range 5 {
		SimulateTick(g, 5000, 0.0, writeState)
		SimulateTick(g, 5000, 1.0, readState)
	}

	wPool := writeState.Blocks["db"].Extra["active_conns"]
	rPool := readState.Blocks["db"].Extra["active_conns"]
	if wPool <= rPool {
		t.Errorf("write-heavy should use more conns: write=%g read=%g", wPool, rPool)
	}
}

func TestSQLConnPoolSaturation(t *testing.T) {
	// At very high write RPS the pool should saturate (active_conns → maxConns=100).
	// 100/0.012 ≈ 8333 write RPS to fill pool.
	g := mustGraph(t, Topology{
		Blocks: []TopoBlock{{ID: "db", Kind: "sql_datastore"}},
	})
	state := NewSimState(g)

	for range 5 {
		results, _ := SimulateTick(g, 10000, 0.0, state)
		_ = results
	}

	active := state.Blocks["db"].Extra["active_conns"]
	if active < 95 {
		t.Errorf("conn pool should be near-full at 10k write RPS, got %g active", active)
	}
}

func mustGraph(t *testing.T, topo Topology) *Graph {
	t.Helper()
	g, err := BuildGraph(topo)
	if err != nil {
		t.Fatal(err)
	}
	return g
}
