package blocks

import "math"

const (
	goroutinePool   = 10000
	svcReadHoldSec  = 0.005 // 5ms wall-clock — network + processing
	svcWriteHoldSec = 0.020 // 20ms wall-clock — parse, validate, downstream calls
	svcMemMB        = 2048
	svcReadMemMB    = 2.0  // response objects, serialization buffers
	svcWriteMemMB   = 10.0 // request body, validation objects, query builders
)

type Service struct{}

func (Service) Kind() string { return "service" }
func (Service) Name() string { return "Service" }

func (Service) Profile() Profile {
	return Profile{
		CPUCores:       2,
		MemoryMB:       svcMemMB,
		Read:           OpCost{CPUMs: 0.3, MemoryMB: svcReadMemMB},
		Write:          OpCost{CPUMs: 2.0, MemoryMB: svcWriteMemMB},
		MaxConcurrency: goroutinePool,
	}
}

func (Service) InitState(state map[string]float64) {
	state["active_goroutines"] = 0
}

// Goroutine pool + memory: wall-clock hold times are much longer than CPU times
// because requests block on downstream calls (DB, cache).
func (Service) Tick(ctx TickContext) TickEffect {
	total := ctx.Reads + ctx.Writes
	readRPS := ctx.Reads / ctx.Dt
	writeRPS := ctx.Writes / ctx.Dt
	active := math.Min(readRPS*svcReadHoldSec+writeRPS*svcWriteHoldSec, goroutinePool)
	ctx.State["active_goroutines"] = active

	readRatio := ctx.Reads / math.Max(total, 1)
	memPerReq := svcReadMemMB*readRatio + svcWriteMemMB*(1-readRatio)
	memPressure := active * memPerReq / svcMemMB

	poolUtil := active / goroutinePool
	e := TickEffect{
		CapMultiplier: 1.0,
		Metrics: map[string]float64{
			"goroutine_util": poolUtil,
			"mem_pressure":   memPressure,
		},
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
