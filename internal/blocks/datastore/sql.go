package datastore

import "github.com/prashanth/archimedes/internal/blocks"

const (
	bTreeReadIOs  = 2
	bTreeWriteIOs = 10
	bufferPool    = 0.75
	maxConns      = 100
)

type SQL struct{}

func (SQL) Kind() string { return "sql_datastore" }
func (SQL) Name() string { return "SQL Datastore" }

func (SQL) Profile() blocks.Profile {
	return blocks.Profile{
		CPUCores: 4,
		MemoryMB: 16384,
		DiskIOPS: blocks.SSDDiskIOPS,
		Read:     blocks.OpCost{CPUMs: 0.5, MemoryMB: 0.5, DiskIOs: bTreeReadIOs},
		Write:    blocks.OpCost{CPUMs: 1.0, MemoryMB: 0.5, DiskIOs: bTreeWriteIOs},
		MaxConcurrency:  maxConns,
		BufferPoolRatio: bufferPool,
		Durability:      blocks.DurabilityPerWrite,
	}
}

func init() { blocks.Types = append(blocks.Types, SQL{}) }
