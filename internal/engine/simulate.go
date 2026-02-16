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
	QueueDepth float64 `json:"queue_depth"`
}

type BlockState struct {
	Queue float64
}

type SimState struct {
	Blocks map[string]*BlockState
}

func NewSimState(g *Graph) *SimState {
	s := &SimState{Blocks: make(map[string]*BlockState, len(g.nodes))}
	for id := range g.nodes {
		s.Blocks[id] = &BlockState{}
	}
	return s
}

func (s *SimState) AllDrained() bool {
	for _, bs := range s.Blocks {
		if bs.Queue > 0.5 {
			return false
		}
	}
	return true
}

const tickDt = 0.1 // seconds per tick

func SimulateTick(g *Graph, rps float64, readRatio float64, state *SimState) ([]BlockResult, error) {
	order, err := g.TopoOrder()
	if err != nil {
		return nil, err
	}

	arriving := make(map[string]float64)
	for _, src := range g.Sources() {
		arriving[src.ID] = rps * tickDt
	}

	results := make([]BlockResult, 0, len(order))
	for _, id := range order {
		node := g.nodes[id]
		bs := state.Blocks[id]

		total := bs.Queue + arriving[id]
		rawCap := nodeCapacity(node, readRatio) * tickDt
		// Contention: as utilization rises past 60%, effective throughput drops.
		// Models lock waits, context switches, GC pressure in real systems.
		util := math.Min(total/rawCap, 1.0)
		contention := 1.0
		if util > 0.6 {
			t := (util - 0.6) / 0.4
			contention = 1.0 - 0.5*t*t // quadratic — gentle at 70%, steep at 90%+
		}
		cap := rawCap * contention
		processed := math.Min(total, cap)
		bs.Queue = total - processed

		effectiveRPS := processed / tickDt
		br := computeBlock(node, effectiveRPS, readRatio)
		br.QueueDepth = bs.Queue
		results = append(results, br)

		for _, down := range node.outgoing {
			arriving[down] += processed
		}
	}
	return results, nil
}

func nodeCapacity(node *Node, readRatio float64) float64 {
	b, ok := blocks.ByKind(node.Kind)
	if !ok || node.Kind == "user" {
		return math.MaxFloat64
	}
	return BlockCapacity(b.Profile(), readRatio)
}

// BlockCapacity returns the max RPS a block can handle given a read/write mix.
// readRatio is 0.0 (all writes) to 1.0 (all reads).
func BlockCapacity(p blocks.Profile, readRatio float64) float64 {
	writeRatio := 1.0 - readRatio
	cap := math.MaxFloat64

	// CPU: weighted cost per request
	weightedCPUMs := p.Read.CPUMs*readRatio + p.Write.CPUMs*writeRatio
	if weightedCPUMs > 0 && p.CPUCores > 0 {
		cap = math.Min(cap, float64(p.CPUCores)*1000/weightedCPUMs)
	}

	// Disk: reads benefit from buffer pool, writes don't.
	// Sequential IO is 10x more efficient (counts as 1/10th of an IOPS).
	if p.DiskIOPS > 0 {
		var weightedDiskIOs float64

		readIOs := p.Read.DiskIOs * (1 - p.BufferPoolRatio) * readRatio
		if p.Read.Sequential {
			readIOs /= 10
		}
		weightedDiskIOs += readIOs

		writeIOs := p.Write.DiskIOs * writeRatio
		if p.Write.Sequential {
			writeIOs /= 10
		}
		weightedDiskIOs += writeIOs

		if weightedDiskIOs > 0 {
			cap = math.Min(cap, float64(p.DiskIOPS)/weightedDiskIOs)
		}
	}

	return cap
}

func Simulate(g *Graph, rps float64, readRatio float64) ([]BlockResult, error) {
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

		br := computeBlock(node, nodeRPS, readRatio)
		results = append(results, br)

		for _, down := range node.outgoing {
			incoming[down] += nodeRPS
		}
	}
	return results, nil
}

func computeBlock(node *Node, rps float64, readRatio float64) BlockResult {
	br := BlockResult{ID: node.ID, Kind: node.Kind, RPS: rps}

	b, ok := blocks.ByKind(node.Kind)
	if !ok || node.Kind == "user" {
		br.Health = "green"
		return br
	}

	p := b.Profile()
	writeRatio := 1.0 - readRatio
	readRPS := rps * readRatio
	writeRPS := rps * writeRatio

	// CPU utilization: weighted by read and write costs
	if p.CPUCores > 0 {
		cpuCap := float64(p.CPUCores) * 1000
		br.CPUUtil = (readRPS*p.Read.CPUMs + writeRPS*p.Write.CPUMs) / cpuCap
	}

	// Memory utilization: concurrent requests hold memory
	weightedCPUMs := p.Read.CPUMs*readRatio + p.Write.CPUMs*writeRatio
	weightedMemMB := p.Read.MemoryMB*readRatio + p.Write.MemoryMB*writeRatio
	concurrent := math.Min(rps*(weightedCPUMs/1000), float64(p.MaxConcurrency))
	if p.MemoryMB > 0 {
		br.MemUtil = concurrent * weightedMemMB / float64(p.MemoryMB)
	}

	// Disk utilization: reads benefit from buffer pool, writes don't.
	// Sequential IO is 10x more efficient.
	if p.DiskIOPS > 0 {
		var diskIOsPerSec float64

		if p.Read.DiskIOs > 0 {
			readIOs := readRPS * p.Read.DiskIOs * (1 - p.BufferPoolRatio)
			if p.Read.Sequential {
				readIOs /= 10
			}
			diskIOsPerSec += readIOs
		}

		if p.Write.DiskIOs > 0 {
			writeIOs := writeRPS * p.Write.DiskIOs
			if p.Write.Sequential {
				writeIOs /= 10
			}
			diskIOsPerSec += writeIOs
		}

		br.DiskUtil = diskIOsPerSec / float64(p.DiskIOPS)
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
