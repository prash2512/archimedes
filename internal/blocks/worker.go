package blocks

import "math"

const (
	workerThreads       = 50
	workerReadHoldSec   = 0.0005 // 0.5ms — status check
	workerWriteHoldSec  = 0.005  // 5ms — job execution
)

type Worker struct{}

func (Worker) Kind() string { return "worker" }
func (Worker) Name() string { return "Worker" }

func (Worker) Profile() Profile {
	return Profile{
		CPUCores: 4,
		MemoryMB: 8192,
		DiskIOPS: SSDDiskIOPS,
		Read:     OpCost{CPUMs: 0.5, MemoryMB: 0.5},
		Write:    OpCost{CPUMs: 5.0, MemoryMB: 4.0, DiskIOs: 2},
		MaxConcurrency: workerThreads,
	}
}

func (Worker) InitState(state map[string]float64) {
	state["active_threads"] = 0
}

// Thread pool: only 50 threads, jobs hold them for 5ms each.
// Fills fast under load — the main bottleneck for workers.
func (Worker) Tick(ctx TickContext) TickEffect {
	readRPS := ctx.Reads / ctx.Dt
	writeRPS := ctx.Writes / ctx.Dt
	active := math.Min(readRPS*workerReadHoldSec+writeRPS*workerWriteHoldSec, workerThreads)
	ctx.State["active_threads"] = active

	poolUtil := active / workerThreads
	e := TickEffect{
		CapMultiplier: 1.0,
		Metrics:       map[string]float64{"thread_pool_util": poolUtil},
	}

	if poolUtil > 0.7 {
		t := (poolUtil - 0.7) / 0.3
		e.CapMultiplier = 1.0 - 0.4*t*t
		e.Latency = workerWriteHoldSec * 1000 * (1 + 2*t*t)
	}
	if poolUtil >= 0.99 {
		e.Saturated = true
	}
	return e
}

func init() { Types = append(Types, Worker{}) }
