package blocks

import "math"

const (
	goroutinePool    = 10000
	svcReadHoldSec   = 0.0003 // 0.3ms — validate, serialize, respond
	svcWriteHoldSec  = 0.002  // 2ms — parse, hash, build queries
)

type Service struct{}

func (Service) Kind() string { return "service" }
func (Service) Name() string { return "Service" }

func (Service) Profile() Profile {
	return Profile{
		CPUCores: 2,
		MemoryMB: 4096,
		Read:     OpCost{CPUMs: 0.3, MemoryMB: 0.2},
		Write:    OpCost{CPUMs: 2.0, MemoryMB: 1.0},
		MaxConcurrency: goroutinePool,
	}
}

func (Service) InitState(state map[string]float64) {
	state["active_goroutines"] = 0
}

// Goroutine pool: writes hold goroutines ~7x longer than reads.
func (Service) Tick(ctx TickContext) TickEffect {
	readRPS := ctx.Reads / ctx.Dt
	writeRPS := ctx.Writes / ctx.Dt
	active := math.Min(readRPS*svcReadHoldSec+writeRPS*svcWriteHoldSec, goroutinePool)
	ctx.State["active_goroutines"] = active

	poolUtil := active / goroutinePool
	e := TickEffect{
		CapMultiplier: 1.0,
		Metrics:       map[string]float64{"goroutine_util": poolUtil},
	}

	if poolUtil > 0.7 {
		t := (poolUtil - 0.7) / 0.3
		e.CapMultiplier = 1.0 - 0.4*t*t
		e.Latency = svcWriteHoldSec * 1000 * (1 + 2*t*t)
	}
	if poolUtil >= 0.99 {
		e.Saturated = true
	}
	return e
}

func init() { Types = append(Types, Service{}) }
