package queue

import "github.com/prashanth/archimedes/internal/blocks"

const (
	appendLogIOs = 1
	cpuPerOp     = 0.02
	brokerConns  = 10000
)

type Kafka struct{}

func (Kafka) Kind() string { return "kafka" }
func (Kafka) Name() string { return "Kafka" }

func (Kafka) Profile() blocks.Profile {
	return blocks.Profile{
		CPUCores: 4,
		MemoryMB: 32768,
		DiskIOPS: blocks.SSDDiskIOPS,
		Read:     blocks.OpCost{CPUMs: cpuPerOp, MemoryMB: 0.01, DiskIOs: appendLogIOs, Sequential: true},
		Write:    blocks.OpCost{CPUMs: cpuPerOp, MemoryMB: 0.01, DiskIOs: appendLogIOs, Sequential: true},
		MaxConcurrency: brokerConns,
		Durability:     blocks.DurabilityBatch,
	}
}

func init() { blocks.Types = append(blocks.Types, Kafka{}) }
