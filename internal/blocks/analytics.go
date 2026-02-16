package blocks

import "math"

const (
	queryThreads          = 100
	analyticsReadHoldSec  = 0.100 // 100ms — full aggregation (scan + sort + group)
	analyticsWriteHoldSec = 0.005 // 5ms — log ingestion
	analyticsMemMB        = 32768
	analyticsReadMemMB    = 200.0 // intermediate aggregation buffers
	analyticsWriteMemMB   = 0.2   // log entry, tiny
)

type Analytics struct{}

func (Analytics) Kind() string { return "analytics" }
func (Analytics) Name() string { return "Analytics" }

func (Analytics) Profile() Profile {
	return Profile{
		CPUCores:       8,
		MemoryMB:       analyticsMemMB,
		Read:           OpCost{CPUMs: 10.0, MemoryMB: analyticsReadMemMB},
		Write:          OpCost{CPUMs: 0.5, MemoryMB: analyticsWriteMemMB},
		MaxConcurrency: queryThreads,
	}
}

func (Analytics) InitState(state map[string]float64) {
	state["active_queries"] = 0
}

// Query thread pool + memory: aggregation queries hold 200MB each for 100ms.
// Opposite of most blocks — reads fill the pool and memory, not writes.
func (Analytics) Tick(ctx TickContext) TickEffect {
	total := ctx.Reads + ctx.Writes
	readRPS := ctx.Reads / ctx.Dt
	writeRPS := ctx.Writes / ctx.Dt
	active := math.Min(readRPS*analyticsReadHoldSec+writeRPS*analyticsWriteHoldSec, queryThreads)
	ctx.State["active_queries"] = active

	readRatio := ctx.Reads / math.Max(total, 1)
	memPerReq := analyticsReadMemMB*readRatio + analyticsWriteMemMB*(1-readRatio)
	memPressure := active * memPerReq / analyticsMemMB

	poolUtil := active / queryThreads
	e := TickEffect{
		CapMultiplier: 1.0,
		Metrics: map[string]float64{
			"query_pool_util": poolUtil,
			"mem_pressure":    memPressure,
		},
	}

	if poolUtil > 0.7 {
		t := (poolUtil - 0.7) / 0.3
		e.CapMultiplier = 1.0 - 0.4*t*t
		e.Latency = analyticsReadHoldSec * 1000 * (1 + 2*t*t)
	}
	if poolUtil >= 0.99 {
		e.Saturated = true
	}
	return e
}

func init() { Types = append(Types, Analytics{}) }
