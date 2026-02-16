package blocks

import "math"

const (
	queryThreads         = 100
	analyticsReadHoldSec  = 0.010 // 10ms — aggregation query
	analyticsWriteHoldSec = 0.0005 // 0.5ms — log ingestion
)

type Analytics struct{}

func (Analytics) Kind() string { return "analytics" }
func (Analytics) Name() string { return "Analytics" }

func (Analytics) Profile() Profile {
	return Profile{
		CPUCores: 8,
		MemoryMB: 32768,
		Read:     OpCost{CPUMs: 10.0, MemoryMB: 8.0},
		Write:    OpCost{CPUMs: 0.5, MemoryMB: 0.2},
		MaxConcurrency: queryThreads,
	}
}

func (Analytics) InitState(state map[string]float64) {
	state["active_queries"] = 0
}

// Query thread pool: reads (aggregations) hold threads for 10ms each.
// Opposite of most blocks — reads fill the pool, not writes.
func (Analytics) Tick(ctx TickContext) TickEffect {
	readRPS := ctx.Reads / ctx.Dt
	writeRPS := ctx.Writes / ctx.Dt
	active := math.Min(readRPS*analyticsReadHoldSec+writeRPS*analyticsWriteHoldSec, queryThreads)
	ctx.State["active_queries"] = active

	poolUtil := active / queryThreads
	e := TickEffect{
		CapMultiplier: 1.0,
		Metrics:       map[string]float64{"query_pool_util": poolUtil},
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
