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
