package engine

import (
	"errors"
	"fmt"
)

type OutEdge struct {
	To     string
	Weight float64 // fraction of traffic (0.0–1.0), default 1.0
}

type Node struct {
	ID       string
	Kind     string
	Replicas int
	Shards   int
	CPUCores int
	outgoing []OutEdge
}

type Graph struct {
	nodes    map[string]*Node
	incoming map[string]int
}

type TopoBlock struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Replicas int    `json:"replicas,omitempty"`
	Shards   int    `json:"shards,omitempty"`
	CPUCores int    `json:"cpu_cores,omitempty"`
}

type TopoEdge struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Weight float64 `json:"weight,omitempty"` // 0.0–1.0, default 1.0
}

type Topology struct {
	Blocks    []TopoBlock `json:"blocks"`
	Edges     []TopoEdge  `json:"edges"`
	RPS       float64     `json:"rps"`
	ReadRatio float64     `json:"read_ratio"`
}

func BuildGraph(topo Topology) (*Graph, error) {
	g := &Graph{
		nodes:    make(map[string]*Node),
		incoming: make(map[string]int),
	}

	for _, b := range topo.Blocks {
		replicas := b.Replicas
		if replicas < 1 {
			replicas = 1
		}
		shards := b.Shards
		if shards < 1 {
			shards = 1
		}
		g.nodes[b.ID] = &Node{
			ID:       b.ID,
			Kind:     b.Kind,
			Replicas: replicas,
			Shards:   shards,
			CPUCores: b.CPUCores,
		}
		g.incoming[b.ID] = 0
	}

	for _, e := range topo.Edges {
		from, ok := g.nodes[e.From]
		if !ok {
			return nil, fmt.Errorf("unknown block %q in edge", e.From)
		}
		if _, ok := g.nodes[e.To]; !ok {
			return nil, fmt.Errorf("unknown block %q in edge", e.To)
		}
		w := e.Weight
		if w <= 0 {
			w = 1.0
		}
		from.outgoing = append(from.outgoing, OutEdge{To: e.To, Weight: w})
		g.incoming[e.To]++
	}

	return g, nil
}

func (g *Graph) Node(id string) *Node {
	return g.nodes[id]
}

func (g *Graph) Downstream(id string) []*Node {
	node := g.nodes[id]
	if node == nil {
		return nil
	}
	out := make([]*Node, len(node.outgoing))
	for i, oe := range node.outgoing {
		out[i] = g.nodes[oe.To]
	}
	return out
}

func (g *Graph) Sources() []*Node {
	var srcs []*Node
	for id, count := range g.incoming {
		if count == 0 {
			srcs = append(srcs, g.nodes[id])
		}
	}
	return srcs
}

// TopoOrder returns node IDs in topological order (Kahn's algorithm).
// Returns an error if the graph contains a cycle.
func (g *Graph) TopoOrder() ([]string, error) {
	deg := make(map[string]int, len(g.incoming))
	for id, d := range g.incoming {
		deg[id] = d
	}

	var queue []string
	for id, d := range deg {
		if d == 0 {
			queue = append(queue, id)
		}
	}

	var order []string
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		order = append(order, id)
		for _, oe := range g.nodes[id].outgoing {
			deg[oe.To]--
			if deg[oe.To] == 0 {
				queue = append(queue, oe.To)
			}
		}
	}

	if len(order) != len(g.nodes) {
		return nil, errors.New("cycle detected in topology")
	}
	return order, nil
}
