package engine

import (
	"math"

	"github.com/prashanth/archimedes/internal/blocks"
)

type BlockResult struct {
	ID         string  `json:"id"`
	Kind       string  `json:"kind"`
	RPS        float64 `json:"rps"`
	CPUUtil    float64 `json:"cpu_util"`
	MemUtil    float64 `json:"mem_util"`
	DiskUtil   float64 `json:"disk_util"`
	Bottleneck float64 `json:"bottleneck"`
	Health     string  `json:"health"`
}

func Simulate(g *Graph, rps float64) ([]BlockResult, error) {
	order, err := g.TopoOrder()
	if err != nil {
		return nil, err
	}

	incoming := make(map[string]float64) // accumulated RPS per node
	for _, src := range g.Sources() {
		incoming[src.ID] = rps
	}

	results := make([]BlockResult, 0, len(order))
	for _, id := range order {
		node := g.nodes[id]
		nodeRPS := incoming[id]

		br := computeBlock(node, nodeRPS)
		results = append(results, br)

		for _, down := range node.outgoing {
			incoming[down] += nodeRPS
		}
	}
	return results, nil
}

func computeBlock(node *Node, rps float64) BlockResult {
	br := BlockResult{ID: node.ID, Kind: node.Kind, RPS: rps}

	b, ok := blocks.ByKind(node.Kind)
	if !ok || node.Kind == "user" {
		br.Health = "green"
		return br
	}

	p := b.Profile()
	op := p.Read // MVP: treat all traffic as reads

	if p.CPUCores > 0 {
		br.CPUUtil = rps * op.CPUMs / (float64(p.CPUCores) * 1000)
	}

	concurrent := math.Min(rps*(op.CPUMs/1000), float64(p.MaxConcurrency))
	if p.MemoryMB > 0 {
		br.MemUtil = concurrent * op.MemoryMB / float64(p.MemoryMB)
	}

	if p.DiskIOPS > 0 && op.DiskIOs > 0 {
		diskIOs := rps * op.DiskIOs * (1 - p.BufferPoolRatio)
		effIOPS := float64(p.DiskIOPS)
		if op.Sequential {
			effIOPS *= 10
		}
		br.DiskUtil = diskIOs / effIOPS
	}

	br.Bottleneck = max(br.CPUUtil, br.MemUtil, br.DiskUtil)

	switch {
	case br.Bottleneck < 0.6:
		br.Health = "green"
	case br.Bottleneck < 0.9:
		br.Health = "yellow"
	default:
		br.Health = "red"
	}
	return br
}
