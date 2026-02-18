package docstore

import (
	"math"

	"github.com/prashanth/archimedes/internal/blocks"
)

const (
	mongoConnPool     = 500
	mongoTotalMemMB   = 16384
	mongoCacheMemMB   = 11468.8 // 70% of total for WiredTiger cache
	mongoCompactRate  = 0.02    // compaction reclaims 2% per tick
	mongoWriteAmpIOs  = 4       // write amplification from journaling + oplog
)

type MongoDB struct{}

func (MongoDB) Kind() string { return "docstore" }
func (MongoDB) Name() string { return "Document DB" }

func (MongoDB) Profile() blocks.Profile {
	return blocks.Profile{
		CPUCores:        4,
		MemoryMB:        mongoTotalMemMB,
		DiskIOPS:        blocks.SSDDiskIOPS,
		Read:            blocks.OpCost{CPUMs: 0.3, MemoryMB: 0.5, DiskIOs: 2},
		Write:           blocks.OpCost{CPUMs: 1.0, MemoryMB: 0.5, DiskIOs: mongoWriteAmpIOs},
		MaxConcurrency:  mongoConnPool,
		BufferPoolRatio: 0.70,
	}
}

func (MongoDB) InitState(state map[string]float64) {
	state["cache_used_mb"] = 0
}

// WiredTiger cache pressure + compaction: writes fill the cache,
// compaction runs in the background stealing CPU from queries.
func (MongoDB) Tick(ctx blocks.TickContext) blocks.TickEffect {
	used := ctx.State["cache_used_mb"]

	// Writes add to cache, compaction reclaims
	used += ctx.Writes * 0.5 // MemoryMB per write
	used -= used * mongoCompactRate
	used = math.Max(0, math.Min(used, mongoCacheMemMB))
	ctx.State["cache_used_mb"] = used

	pressure := used / mongoCacheMemMB

	capMult := 1.0
	latency := 0.3 // base read latency
	if pressure > 0.6 {
		// Compaction fights queries for CPU + disk
		severity := (pressure - 0.6) / 0.4
		capMult = 1.0 - 0.4*severity  // down to 0.6x at full cache
		latency = 0.3 * (1 + 5*severity)
	}

	return blocks.TickEffect{
		CapMultiplier: capMult,
		Latency:       latency,
		Saturated:     pressure > 0.95,
		Metrics:       map[string]float64{"cache_pressure": pressure},
	}
}

func init() { blocks.Types = append(blocks.Types, MongoDB{}) }
