package queue

import (
	"math"

	"github.com/prashanth/archimedes/internal/blocks"
)

const (
	appendLogIOs    = 1
	cpuPerOp        = 0.02
	brokerConns     = 10000
	pageCacheMemMB  = 32768.0 // total page cache available
	pageCacheFillMB = 0.01    // MB per write filling page cache
	pageCacheDecay  = 0.02    // 2% natural eviction per tick
)

type Kafka struct{}

func (Kafka) Kind() string { return "kafka" }
func (Kafka) Name() string { return "Kafka" }

func (Kafka) Profile() blocks.Profile {
	return blocks.Profile{
		CPUCores: 4,
		MemoryMB: 32768,
		DiskIOPS: blocks.SSDDiskIOPS,
		Read:     blocks.OpCost{CPUMs: cpuPerOp, MemoryMB: 0.01, DiskIOs: appendLogIOs, Sequential: true},
		Write:    blocks.OpCost{CPUMs: cpuPerOp, MemoryMB: 0.01, DiskIOs: appendLogIOs, Sequential: true},
		MaxConcurrency: brokerConns,
		Durability:     blocks.DurabilityBatch,
	}
}

func (Kafka) InitState(state map[string]float64) {
	state["page_cache_used"] = 0
}

// Page cache pressure: heavy writes fill OS page cache. Consumers reading
// recent data hit cache (fast); when cache is full, reads fall to disk.
func (Kafka) Tick(ctx blocks.TickContext) blocks.TickEffect {
	used := ctx.State["page_cache_used"]

	used += ctx.Writes * pageCacheFillMB
	used -= used * pageCacheDecay
	used = math.Max(0, math.Min(used, pageCacheMemMB))
	ctx.State["page_cache_used"] = used

	cacheUtil := used / pageCacheMemMB

	capMult := 1.0
	latency := cpuPerOp * 1000 // base latency
	if cacheUtil > 0.7 {
		// Reads miss page cache, fall to disk — much slower
		pressure := (cacheUtil - 0.7) / 0.3
		capMult = 1.0 - 0.3*pressure
		latency = cpuPerOp * 1000 * (1 + 5*pressure)
	}

	return blocks.TickEffect{
		CapMultiplier: capMult,
		Latency:       latency,
		Saturated:     cacheUtil > 0.95,
		Metrics:       map[string]float64{"page_cache_util": cacheUtil},
	}
}

func init() { blocks.Types = append(blocks.Types, Kafka{}) }
