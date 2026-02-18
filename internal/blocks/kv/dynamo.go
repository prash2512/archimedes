package kv

import (
	"math"

	"github.com/prashanth/archimedes/internal/blocks"
)

const (
	kvConnPool       = 10000
	kvTotalMemMB     = 16384
	kvHotspotBuildup = 0.03 // pressure builds 3% per tick under heavy writes
	kvHotspotDecay   = 0.01 // pressure decays 1% per tick naturally
)

type KVStore struct{}

func (KVStore) Kind() string { return "kv_store" }
func (KVStore) Name() string { return "KV Store" }

func (KVStore) Profile() blocks.Profile {
	return blocks.Profile{
		CPUCores:        4,
		MemoryMB:        kvTotalMemMB,
		DiskIOPS:        blocks.SSDDiskIOPS,
		Read:            blocks.OpCost{CPUMs: 0.1, MemoryMB: 0.01, DiskIOs: 1},
		Write:           blocks.OpCost{CPUMs: 0.3, MemoryMB: 0.01, DiskIOs: 2},
		MaxConcurrency:  kvConnPool,
		BufferPoolRatio: 0.90,
	}
}

func (KVStore) InitState(state map[string]float64) {
	state["hotspot_pressure"] = 0
}

// Partition hotspot: heavy writes concentrate on hot partitions,
// degrading throughput. Pressure builds under write load and decays at rest.
func (KVStore) Tick(ctx blocks.TickContext) blocks.TickEffect {
	pressure := ctx.State["hotspot_pressure"]

	if ctx.Writes > 0 {
		// Write intensity relative to capacity drives hotspot buildup
		writeIntensity := math.Min(ctx.Writes/ctx.RawCap, 1.0)
		pressure += kvHotspotBuildup * writeIntensity
	}
	pressure -= kvHotspotDecay * pressure
	pressure = math.Max(0, math.Min(1, pressure))
	ctx.State["hotspot_pressure"] = pressure

	capMult := 1.0
	latency := 0.1 // base ~0.1ms single-digit latency
	if pressure > 0.3 {
		// Hot partitions throttle writes, spill to reads
		severity := (pressure - 0.3) / 0.7
		capMult = 1.0 - 0.5*severity
		latency = 0.1 * (1 + 10*severity)
	}

	return blocks.TickEffect{
		CapMultiplier: capMult,
		Latency:       latency,
		Saturated:     pressure > 0.9,
		Metrics:       map[string]float64{"hotspot_pressure": pressure},
	}
}

func init() { blocks.Types = append(blocks.Types, KVStore{}) }
