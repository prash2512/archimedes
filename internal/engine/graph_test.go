package engine

import "testing"

func TestBuildGraph(t *testing.T) {
	topo := Topology{
		Blocks: []TopoBlock{
			{ID: "b1", Kind: "user"},
			{ID: "b2", Kind: "service"},
			{ID: "b3", Kind: "sql_datastore"},
		},
		Edges: []TopoEdge{
			{From: "b1", To: "b2"},
			{From: "b2", To: "b3"},
		},
	}

	g, err := BuildGraph(topo)
	if err != nil {
		t.Fatal(err)
	}

	srcs := g.Sources()
	if len(srcs) != 1 || srcs[0].Kind != "user" {
		t.Fatalf("expected single user source, got %v", srcs)
	}

	ds := g.Downstream("b1")
	if len(ds) != 1 || ds[0].ID != "b2" {
		t.Fatalf("expected b2 downstream of b1, got %v", ds)
	}

	ds = g.Downstream("b2")
	if len(ds) != 1 || ds[0].ID != "b3" {
		t.Fatalf("expected b3 downstream of b2, got %v", ds)
	}

	ds = g.Downstream("b3")
	if len(ds) != 0 {
		t.Fatalf("expected no downstream of b3, got %v", ds)
	}
}

func TestTopoOrder(t *testing.T) {
	topo := Topology{
		Blocks: []TopoBlock{
			{ID: "b1", Kind: "user"},
			{ID: "b2", Kind: "service"},
			{ID: "b3", Kind: "sql_datastore"},
		},
		Edges: []TopoEdge{
			{From: "b1", To: "b2"},
			{From: "b2", To: "b3"},
		},
	}
	g, err := BuildGraph(topo)
	if err != nil {
		t.Fatal(err)
	}

	order, err := g.TopoOrder()
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(order))
	}

	pos := map[string]int{}
	for i, id := range order {
		pos[id] = i
	}
	if pos["b1"] > pos["b2"] || pos["b2"] > pos["b3"] {
		t.Fatalf("bad order: %v", order)
	}
}

func TestTopoOrderDiamond(t *testing.T) {
	// b1 -> b2 -> b4
	// b1 -> b3 -> b4
	topo := Topology{
		Blocks: []TopoBlock{
			{ID: "b1", Kind: "user"},
			{ID: "b2", Kind: "service"},
			{ID: "b3", Kind: "redis"},
			{ID: "b4", Kind: "sql_datastore"},
		},
		Edges: []TopoEdge{
			{From: "b1", To: "b2"},
			{From: "b1", To: "b3"},
			{From: "b2", To: "b4"},
			{From: "b3", To: "b4"},
		},
	}
	g, err := BuildGraph(topo)
	if err != nil {
		t.Fatal(err)
	}

	order, err := g.TopoOrder()
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(order))
	}

	pos := map[string]int{}
	for i, id := range order {
		pos[id] = i
	}
	if pos["b1"] > pos["b2"] || pos["b1"] > pos["b3"] || pos["b2"] > pos["b4"] || pos["b3"] > pos["b4"] {
		t.Fatalf("bad order: %v", order)
	}
}

func TestTopoOrderCycle(t *testing.T) {
	topo := Topology{
		Blocks: []TopoBlock{
			{ID: "b1", Kind: "service"},
			{ID: "b2", Kind: "service"},
		},
		Edges: []TopoEdge{
			{From: "b1", To: "b2"},
			{From: "b2", To: "b1"},
		},
	}
	g, err := BuildGraph(topo)
	if err != nil {
		t.Fatal(err)
	}

	_, err = g.TopoOrder()
	if err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestBuildGraphBadEdge(t *testing.T) {
	topo := Topology{
		Blocks: []TopoBlock{{ID: "b1", Kind: "user"}},
		Edges:  []TopoEdge{{From: "b1", To: "b99"}},
	}

	_, err := BuildGraph(topo)
	if err == nil {
		t.Fatal("expected error for unknown block in edge")
	}
}
