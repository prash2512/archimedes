package search

import "github.com/prashanth/archimedes/internal/blocks"

const (
	invertedIndexReadIOs  = 2
	invertedIndexWriteIOs = 5
	bufferPool            = 0.50
	threadPool            = 1000
)

type Elasticsearch struct{}

func (Elasticsearch) Kind() string { return "elasticsearch" }
func (Elasticsearch) Name() string { return "Elasticsearch" }

func (Elasticsearch) Profile() blocks.Profile {
	return blocks.Profile{
		CPUCores: 8,
		MemoryMB: 32768,
		DiskIOPS: blocks.SSDDiskIOPS,
		Read:     blocks.OpCost{CPUMs: 1.0, MemoryMB: 1.0, DiskIOs: invertedIndexReadIOs},
		Write:    blocks.OpCost{CPUMs: 2.0, MemoryMB: 1.0, DiskIOs: invertedIndexWriteIOs},
		MaxConcurrency:  threadPool,
		BufferPoolRatio: bufferPool,
		Durability:      blocks.DurabilityBatch,
	}
}

func init() { blocks.Types = append(blocks.Types, Elasticsearch{}) }
