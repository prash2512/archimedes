package storage

import (
	"math"

	"github.com/prashanth/archimedes/internal/blocks"
)

const (
	s3RateLimit     = 5000
	s3BandwidthMBps = 500.0  // ~4 Gbps throughput cap
	s3ReadMB        = 0.5    // avg object size for reads
	s3WriteMB       = 1.0    // avg object size for writes
	s3BaseLatencyMs = 50.0   // minimum latency floor (network + S3 overhead)
)

type S3 struct{}

func (S3) Kind() string { return "s3" }
func (S3) Name() string { return "Object Storage" }

func (S3) Profile() blocks.Profile {
	return blocks.Profile{
		CPUCores:       8, // managed service, effectively large
		MemoryMB:       65536,
		DiskIOPS:       50000, // effectively unlimited IOPS
		Read:           blocks.OpCost{CPUMs: 0.1, MemoryMB: s3ReadMB, DiskIOs: 1},
		Write:          blocks.OpCost{CPUMs: 0.2, MemoryMB: s3WriteMB, DiskIOs: 1},
		MaxConcurrency:   s3RateLimit,
		DefaultReadRatio: 0.6,
	}
}

func (S3) InitState(state map[string]float64) {
	state["bandwidth_util"] = 0
}

// Bandwidth saturation: throughput in MB/s against a bandwidth cap.
// High throughput → latency spikes on top of the base latency floor.
func (S3) Tick(ctx blocks.TickContext) blocks.TickEffect {
	throughputMB := ctx.Reads*s3ReadMB + ctx.Writes*s3WriteMB
	// Convert per-tick to per-second
	mbps := throughputMB / ctx.Dt
	bwUtil := math.Min(mbps/s3BandwidthMBps, 1.0)
	ctx.State["bandwidth_util"] = bwUtil

	capMult := 1.0
	latency := s3BaseLatencyMs
	if bwUtil > 0.7 {
		// Past 70% bandwidth, congestion kicks in
		pressure := (bwUtil - 0.7) / 0.3
		capMult = 1.0 - 0.4*pressure // down to 0.6x at full bandwidth
		latency = s3BaseLatencyMs * (1 + 3*pressure)
	}

	return blocks.TickEffect{
		CapMultiplier: capMult,
		Latency:       latency,
		Saturated:     bwUtil > 0.95,
		Metrics:       map[string]float64{"bandwidth_util": bwUtil},
	}
}

func init() { blocks.Types = append(blocks.Types, S3{}) }
