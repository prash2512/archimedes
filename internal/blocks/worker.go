package blocks

import "math"

const (
	workerThreads      = 50
	workerReadHoldSec  = 0.005 // 5ms — status check
	workerWriteHoldSec = 0.050 // 50ms — load payload, process, write results
	workerMemMB        = 8192
	workerReadMemMB    = 1.0  // status response
	workerWriteMemMB   = 10.0 // job payload in memory
)

type Worker struct{}

func (Worker) Kind() string { return "worker" }
func (Worker) Name() string { return "Worker" }

func (Worker) Profile() Profile {
	return Profile{
		CPUCores:       4,
		MemoryMB:       workerMemMB,
		DiskIOPS:       SSDDiskIOPS,
		Read:           OpCost{CPUMs: 0.5, MemoryMB: workerReadMemMB},
		Write:          OpCost{CPUMs: 5.0, MemoryMB: workerWriteMemMB, DiskIOs: 2},
		MaxConcurrency: workerThreads,
	}
}

func (Worker) InitState(state map[string]float64) {
	state["active_threads"] = 0
}

// Thread pool + memory: 50 threads, each job holds 50MB for 100ms.
// Pool saturates at 500 write RPS, memory fills fast.
func (Worker) Tick(ctx TickContext) TickEffect {
	total := ctx.Reads + ctx.Writes
	readRPS := ctx.Reads / ctx.Dt
	writeRPS := ctx.Writes / ctx.Dt
	active := math.Min(readRPS*workerReadHoldSec+writeRPS*workerWriteHoldSec, workerThreads)
	ctx.State["active_threads"] = active

	readRatio := ctx.Reads / math.Max(total, 1)
	memPerReq := workerReadMemMB*readRatio + workerWriteMemMB*(1-readRatio)
	memPressure := active * memPerReq / workerMemMB

	poolUtil := active / workerThreads
	e := TickEffect{
		CapMultiplier: 1.0,
		Metrics: map[string]float64{
			"thread_pool_util": poolUtil,
			"mem_pressure":     memPressure,
		},
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
