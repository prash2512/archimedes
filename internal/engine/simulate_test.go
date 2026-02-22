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
	// Service: 4 cores, 0.2ms per read → saturates at 20000 RPS
	// At 1000 read RPS: cpu = 1000 * 0.2 / 4000 = 0.05
	g := mustGraph(t, Topology{
		Blocks: []TopoBlock{{ID: "s", Kind: "service"}},
	})
	results, err := Simulate(g, 1000, 1.0)
	if err != nil {
		t.Fatal(err)
	}
	r := results[0]
	if !approx(r.CPUUtil, 0.05) {
		t.Errorf("cpu_util: want 0.05, got %f", r.CPUUtil)
	}
	if r.DiskUtil != 0 {
		t.Errorf("disk_util: want 0, got %f", r.DiskUtil)
	}
	if r.Health != "green" {
		t.Errorf("health: want green, got %s", r.Health)
	}
}

func TestSimulateSQLDisk(t *testing.T) {
	// SQL: 8 cores, 50000 IOPS (NVMe), 2 read I/Os, 0.85 buffer pool → 0.3 I/Os per read
	// At 16000 RPS: disk = 16000 * 0.3 / 50000 = 0.096, cpu = 16000 * 0.5 / 8000 = 1.0
	// CPU is the bottleneck, not disk
	g := mustGraph(t, Topology{
		Blocks: []TopoBlock{{ID: "db", Kind: "sql_datastore"}},
	})
	results, err := Simulate(g, 16000, 1.0)
	if err != nil {
		t.Fatal(err)
	}
	r := results[0]
	if r.DiskUtil > 0.15 {
		t.Errorf("disk_util: want <0.15, got %f", r.DiskUtil)
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
		{"service", 20000},       // read: 4*1000/0.2
		{"worker", 8000},         // read: 4*1000/0.5
		{"analytics", 800},       // read: 8*1000/10.0
		{"load_balancer", 100000},
		{"api_gateway", 40000},
		{"redis", 100000},
		{"sql_datastore", 16000}, // read-limited by CPU: 8*1000/0.5
		{"kafka", 200000},        // CPU: 4*1000/0.02
		{"elasticsearch", 8000},  // CPU: 8*1000/1.0
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
	// Service capacity = 20000 RPS reads. Send 25000 → queue should grow.
	g := mustGraph(t, Topology{
		Blocks: []TopoBlock{{ID: "s", Kind: "service"}},
	})
	state := NewSimState(g)

	for i := 0; i < 3; i++ {
		SimulateTick(g, 25000, 1.0, state)
	}
	q := state.Blocks["s"].Queue
	if q < 100 {
		t.Errorf("queue after 3 overloaded ticks should be growing, got %g", q)
	}
}

func TestSimulateTickDrains(t *testing.T) {
	// Overload for 5 ticks, then drain with RPS=0
	g := mustGraph(t, Topology{
		Blocks: []TopoBlock{{ID: "s", Kind: "service"}},
	})
	state := NewSimState(g)

	for i := 0; i < 5; i++ {
		SimulateTick(g, 30000, 1.0, state)
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
	// Service at 90% raw load (18000 RPS, capacity 20000) should queue
	// due to contention effects even though raw capacity isn't exceeded
	g := mustGraph(t, Topology{
		Blocks: []TopoBlock{{ID: "s", Kind: "service"}},
	})
	state := NewSimState(g)

	for i := 0; i < 10; i++ {
		SimulateTick(g, 18000, 1.0, state)
	}
	q := state.Blocks["s"].Queue
	if q < 1 {
		t.Errorf("expected queue buildup at 90%% util, got %g", q)
	}
}

func TestSimulateTickChainThroughput(t *testing.T) {
	// User → Service → SQL at 500 RPS (within capacity for both)
	// Service capacity=20000, SQL read capacity=16000
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
	// SQL: 8 cores, 0.5ms read CPU, 1.0ms write CPU, 50k NVMe IOPS
	// 100% reads: CPU = 8*1000/0.5 = 16000 RPS
	// 100% writes: CPU = 8*1000/1.0 = 8000 RPS
	// 70/30: CPU = 8000/(0.5*0.7+1.0*0.3) = 8000/0.65 ≈ 12308
	b, _ := blocks.ByKind("sql_datastore")
	p := b.Profile()

	allRead := BlockCapacity(p, 1.0)
	if !approx(allRead, 16000) {
		t.Errorf("all-read capacity: want 16000, got %g", allRead)
	}

	allWrite := BlockCapacity(p, 0.0)
	if !approx(allWrite, 8000) {
		t.Errorf("all-write capacity: want 8000, got %g", allWrite)
	}

	mixed := BlockCapacity(p, 0.7)
	if mixed > 12500 || mixed < 12100 {
		t.Errorf("70/30 capacity: want ~12308, got %g", mixed)
	}
}

func TestSQLWriteHeavyHealth(t *testing.T) {
	// SQL: 8 cores, capacity = 16k reads, 8k writes
	// 1000 RPS reads → green, 1000 RPS writes → green
	// 10000 RPS all-writes → red (exceeds 8k capacity)
	g := mustGraph(t, Topology{
		Blocks: []TopoBlock{{ID: "db", Kind: "sql_datastore"}},
	})
	readResults, _ := Simulate(g, 1000, 1.0)
	writeResults, _ := Simulate(g, 1000, 0.0)
	heavyWriteResults, _ := Simulate(g, 10000, 0.0)

	if readResults[0].Health != "green" {
		t.Errorf("all-read at 1000 RPS: want green, got %s", readResults[0].Health)
	}
	if writeResults[0].Health != "green" {
		t.Errorf("all-write at 1000 RPS: want green, got %s", writeResults[0].Health)
	}
	if heavyWriteResults[0].Health != "red" {
		t.Errorf("all-write at 10000 RPS: want red, got %s", heavyWriteResults[0].Health)
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
	// SQL uses default 70/30 ratio internally; at 15k RPS it should queue
	g := mustGraph(t, Topology{
		Blocks: []TopoBlock{{ID: "db", Kind: "sql_datastore"}},
	})
	state := NewSimState(g)

	for range 5 {
		SimulateTick(g, 15000, 0.0, state)
	}
	if state.Blocks["db"].Queue < 100 {
		t.Errorf("SQL at 15000 RPS should queue, got %g", state.Blocks["db"].Queue)
	}
}

func TestRedisMemoryGrowsWithWrites(t *testing.T) {
	g := mustGraph(t, Topology{
		Blocks: []TopoBlock{{ID: "r", Kind: "redis"}},
	})
	state := NewSimState(g)

	for range 20 {
		SimulateTick(g, 80000, 0.0, state) // all writes
	}

	used := state.Blocks["r"].Extra["memory_used_mb"]
	if used < 1 {
		t.Errorf("redis memory should grow with writes, got %g MB", used)
	}
}

func TestRedisEvictionReducesCapacity(t *testing.T) {
	g := mustGraph(t, Topology{
		Blocks: []TopoBlock{{ID: "r", Kind: "redis"}},
	})
	state := NewSimState(g)

	// Pre-fill memory past eviction threshold (80% of 16384 = ~13107 MB)
	state.Blocks["r"].Extra["memory_used_mb"] = 15000

	results, _ := SimulateTick(g, 50000, 0.0, state)
	for _, r := range results {
		if r.ID == "r" {
			if r.Metrics["evicting"] != 1 {
				t.Error("redis should be evicting above 80% memory")
			}
			if r.Metrics["memory_pct"] < 0.8 {
				t.Errorf("memory_pct should be above 0.8, got %g", r.Metrics["memory_pct"])
			}
		}
	}
}

func TestSQLConnPoolAtDefaultRatio(t *testing.T) {
	// SQL default ratio is 70/30. At 5000 RPS:
	// read_conns = 3500*0.002=7, write_conns = 1500*0.012=18, total ~25
	g := mustGraph(t, Topology{
		Blocks: []TopoBlock{{ID: "db", Kind: "sql_datastore"}},
	})
	state := NewSimState(g)

	for range 5 {
		SimulateTick(g, 5000, 0.7, state)
	}

	active := state.Blocks["db"].Extra["active_conns"]
	if active < 10 || active > 100 {
		t.Errorf("expected moderate conn pool usage at 5k RPS, got %g", active)
	}
}

func TestSQLConnPoolSaturation(t *testing.T) {
	// At very high write RPS the pool should saturate (active_conns → maxConns=200).
	// 200/0.010 = 20000 write RPS to fill pool.
	g := mustGraph(t, Topology{
		Blocks: []TopoBlock{{ID: "db", Kind: "sql_datastore"}},
	})
	state := NewSimState(g)

	for range 5 {
		results, _ := SimulateTick(g, 25000, 0.0, state)
		_ = results
	}

	active := state.Blocks["db"].Extra["active_conns"]
	if active < 190 {
		t.Errorf("conn pool should be near-full at 25k write RPS, got %g active", active)
	}
}

func TestEdgeWeightSplitsTraffic(t *testing.T) {
	g := mustGraph(t, Topology{
		Blocks: []TopoBlock{
			{ID: "u", Kind: "user"},
			{ID: "a", Kind: "service"},
			{ID: "b", Kind: "service"},
		},
		Edges: []TopoEdge{
			{From: "u", To: "a", Weight: 0.3},
			{From: "u", To: "b", Weight: 0.7},
		},
	})
	results, _ := Simulate(g, 10000, 0.5)
	var rpsA, rpsB float64
	for _, r := range results {
		if r.ID == "a" {
			rpsA = r.RPS
		}
		if r.ID == "b" {
			rpsB = r.RPS
		}
	}
	if math.Abs(rpsA-3000) > 100 {
		t.Errorf("service A should get ~3000 RPS, got %g", rpsA)
	}
	if math.Abs(rpsB-7000) > 100 {
		t.Errorf("service B should get ~7000 RPS, got %g", rpsB)
	}
}

func TestCDNAbsorbsDownstreamTraffic(t *testing.T) {
	g := mustGraph(t, Topology{
		Blocks: []TopoBlock{
			{ID: "u", Kind: "user"},
			{ID: "cdn", Kind: "cdn"},
			{ID: "svc", Kind: "service"},
		},
		Edges: []TopoEdge{
			{From: "u", To: "cdn"},
			{From: "cdn", To: "svc"},
		},
	})
	state := NewSimState(g)

	// Warm up CDN cache
	for range 50 {
		SimulateTick(g, 5000, 0.9, state)
	}
	results, _ := SimulateTick(g, 5000, 0.9, state)
	var svcRPS float64
	for _, r := range results {
		if r.ID == "svc" {
			svcRPS = r.RPS
		}
	}
	// CDN should absorb most read traffic, so service sees much less than 5000
	if svcRPS > 3000 {
		t.Errorf("CDN should absorb traffic, but service sees %g RPS", svcRPS)
	}
}

func TestSimulateReturnsName(t *testing.T) {
	g := mustGraph(t, Topology{
		Blocks: []TopoBlock{
			{ID: "u", Kind: "user"},
			{ID: "s", Kind: "service", Name: "Video API"},
		},
		Edges: []TopoEdge{{From: "u", To: "s"}},
	})
	results, err := Simulate(g, 1000, 1.0)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.ID == "s" && r.Name != "Video API" {
			t.Errorf("want name 'Video API', got %q", r.Name)
		}
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
