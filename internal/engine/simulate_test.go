package engine

import (
	"math"
	"testing"

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

func mustGraph(t *testing.T, topo Topology) *Graph {
	t.Helper()
	g, err := BuildGraph(topo)
	if err != nil {
		t.Fatal(err)
	}
	return g
}
