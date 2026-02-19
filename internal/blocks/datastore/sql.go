package datastore

import (
	"math"

	"github.com/prashanth/archimedes/internal/blocks"
)

const (
	bTreeReadIOs  = 2
	bTreeWriteIOs = 6
	bufferPool    = 0.85 // well-tuned shared_buffers
	maxConns      = 200

	readHoldSec  = 0.002 // 2ms — quick lookup, buffer pool hit
	writeHoldSec = 0.010 // 10ms — lock, WAL, fsync
)

type SQL struct{}

func (SQL) Kind() string { return "sql_datastore" }
func (SQL) Name() string { return "SQL Datastore" }

func (SQL) Profile() blocks.Profile {
	return blocks.Profile{
		CPUCores: 8,
		MemoryMB: 32768,
		DiskIOPS: blocks.SSDDiskIOPS,
		Read:     blocks.OpCost{CPUMs: 0.5, MemoryMB: 0.5, DiskIOs: bTreeReadIOs},
		Write:    blocks.OpCost{CPUMs: 1.0, MemoryMB: 0.5, DiskIOs: bTreeWriteIOs},
		MaxConcurrency:  maxConns,
		BufferPoolRatio: bufferPool,
		Durability:       blocks.DurabilityPerWrite,
		DefaultReadRatio: 0.7,
	}
}

func (SQL) InitState(state map[string]float64) {
	state["active_conns"] = 0
}

// Connection pool: reads and writes hold connections for different durations.
// Write-heavy loads fill the pool much faster (12ms vs 2ms hold).
func (SQL) Tick(ctx blocks.TickContext) blocks.TickEffect {
	readRPS := ctx.Reads / ctx.Dt
	writeRPS := ctx.Writes / ctx.Dt
	readConns := readRPS * readHoldSec
	writeConns := writeRPS * writeHoldSec
	active := math.Min(readConns+writeConns, maxConns)
	ctx.State["active_conns"] = active

	poolUtil := active / maxConns
	e := blocks.TickEffect{
		CapMultiplier: 1.0,
		Metrics:       map[string]float64{"conn_pool_util": poolUtil},
	}

	if poolUtil > 0.7 {
		t := (poolUtil - 0.7) / 0.3
		e.CapMultiplier = 1.0 - 0.4*t*t
		e.Latency = writeHoldSec * 1000 * (1 + 2*t*t)
	}
	if poolUtil >= 0.99 {
		e.Saturated = true
	}
	return e
}

func init() { blocks.Types = append(blocks.Types, SQL{}) }
