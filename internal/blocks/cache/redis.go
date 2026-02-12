package cache

import "github.com/prashanth/archimedes/internal/blocks"

const (
	singleThread = 1
	cpuPerOp     = 0.01
)

type Redis struct{}

func (Redis) Kind() string { return "redis" }
func (Redis) Name() string { return "Redis" }

func (Redis) Profile() blocks.Profile {
	return blocks.Profile{
		CPUCores: 1,
		MemoryMB: 16384,
		Read:     blocks.OpCost{CPUMs: cpuPerOp, MemoryMB: 0.001},
		Write:    blocks.OpCost{CPUMs: cpuPerOp, MemoryMB: 0.001},
		MaxConcurrency: singleThread,
		Durability:     blocks.DurabilityNone,
	}
}

func init() { blocks.Types = append(blocks.Types, Redis{}) }
