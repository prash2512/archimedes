package cache

import (
	"math"

	"github.com/prashanth/archimedes/internal/blocks"
)

const (
	singleThread = 1
	cpuPerOp     = 0.01
	totalMemMB   = 16384
	ttlDecayRate = 0.01  // 1% of used memory expires per tick
	evictThresh  = 0.80  // LRU eviction kicks in at 80% memory
	memPerWrite  = 0.001 // MB per write op (from OpCost)
)

type Redis struct{}

func (Redis) Kind() string { return "redis" }
func (Redis) Name() string { return "Redis" }

func (Redis) Profile() blocks.Profile {
	return blocks.Profile{
		CPUCores: 1,
		MemoryMB: totalMemMB,
		Read:     blocks.OpCost{CPUMs: cpuPerOp, MemoryMB: memPerWrite},
		Write:    blocks.OpCost{CPUMs: cpuPerOp, MemoryMB: memPerWrite},
		MaxConcurrency: singleThread,
		Durability:     blocks.DurabilityNone,
	}
}

func (Redis) InitState(state map[string]float64) {
	state["memory_used_mb"] = 0
}

// Memory pressure: writes grow memory, TTLs decay it. When memory exceeds 80%,
// LRU eviction runs on the single thread, stealing CPU from request processing.
func (Redis) Tick(ctx blocks.TickContext) blocks.TickEffect {
	used := ctx.State["memory_used_mb"]

	// Writes add memory, TTLs naturally reclaim some.
	used += ctx.Writes * memPerWrite
	used -= used * ttlDecayRate
	used = math.Max(used, 0)
	used = math.Min(used, totalMemMB)
	ctx.State["memory_used_mb"] = used

	memPct := used / totalMemMB
	evicting := 0.0
	e := blocks.TickEffect{
		CapMultiplier: 1.0,
	}

	if memPct > evictThresh {
		evicting = 1.0
		pressure := (memPct - evictThresh) / (1 - evictThresh)
		e.CapMultiplier = 1.0 - 0.5*pressure
		e.Latency = cpuPerOp * 1000 * (1 + 3*pressure)
	}

	total := ctx.Reads + ctx.Writes
	if total > 0 {
		hitRate := math.Min(memPct/evictThresh, 0.95)
		readFrac := ctx.Reads / total
		e.AbsorbRatio = hitRate * readFrac
	}

	e.Metrics = map[string]float64{"memory_pct": memPct, "evicting": evicting}
	return e
}

func init() { blocks.Types = append(blocks.Types, Redis{}) }
