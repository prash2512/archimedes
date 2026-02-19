package search

import (
	"math"

	"github.com/prashanth/archimedes/internal/blocks"
)

const (
	invertedIndexReadIOs  = 2
	invertedIndexWriteIOs = 5
	bufferPool            = 0.50
	threadPool            = 1000
	segmentsPerWrite      = 0.01  // each write contributes to segment creation
	mergeRate             = 0.05  // background merge reclaims 5% of segments per tick
	maxSegments           = 100.0 // normalized segment count cap
)

type Elasticsearch struct{}

func (Elasticsearch) Kind() string { return "elasticsearch" }
func (Elasticsearch) Name() string { return "Elasticsearch" }

func (Elasticsearch) Profile() blocks.Profile {
	return blocks.Profile{
		CPUCores: 8,
		MemoryMB: 32768,
		DiskIOPS: blocks.SSDDiskIOPS,
		Read:     blocks.OpCost{CPUMs: 1.0, MemoryMB: 1.0, DiskIOs: invertedIndexReadIOs},
		Write:    blocks.OpCost{CPUMs: 2.0, MemoryMB: 1.0, DiskIOs: invertedIndexWriteIOs},
		MaxConcurrency:  threadPool,
		BufferPoolRatio: bufferPool,
		Durability:       blocks.DurabilityBatch,
		DefaultReadRatio: 0.8,
	}
}

func (Elasticsearch) InitState(state map[string]float64) {
	state["segment_count"] = 0
}

// Segment merge pressure: writes create segments, background merges steal
// CPU and I/O from queries. Many small segments degrade read performance.
func (Elasticsearch) Tick(ctx blocks.TickContext) blocks.TickEffect {
	segs := ctx.State["segment_count"]

	segs += ctx.Writes * segmentsPerWrite
	segs -= segs * mergeRate
	segs = math.Max(0, math.Min(segs, maxSegments))
	ctx.State["segment_count"] = segs

	pressure := segs / maxSegments

	capMult := 1.0
	latency := 1.0 // base query latency
	if pressure > 0.4 {
		// Merges compete with queries for CPU + disk I/O
		severity := (pressure - 0.4) / 0.6
		capMult = 1.0 - 0.4*severity
		latency = 1.0 * (1 + 4*severity)
	}

	return blocks.TickEffect{
		CapMultiplier: capMult,
		Latency:       latency,
		Saturated:     pressure > 0.9,
		Metrics:       map[string]float64{"merge_pressure": pressure},
	}
}

func init() { blocks.Types = append(blocks.Types, Elasticsearch{}) }
