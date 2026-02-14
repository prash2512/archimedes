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
	results, err := Simulate(g, 1000)
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
	results, err := Simulate(g, 8000)
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
	results, err := Simulate(g, 500)
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
	results, err := Simulate(g, 80000)
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
		got := BlockCapacity(b.Profile())
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
		SimulateTick(g, 8000, state)
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
		SimulateTick(g, 8000, state)
	}
	peak := state.Blocks["s"].Queue
	// Drain: run enough ticks for queue to empty
	for i := 0; i < 20; i++ {
		SimulateTick(g, 0, state)
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
		SimulateTick(g, 3600, state)
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

	results, _ := SimulateTick(g, 500, state)
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

func mustGraph(t *testing.T, topo Topology) *Graph {
	t.Helper()
	g, err := BuildGraph(topo)
	if err != nil {
		t.Fatal(err)
	}
	return g
}
