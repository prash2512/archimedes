package blocks

type Durability string

const (
	DurabilityNone     Durability = "none"
	DurabilityBatch    Durability = "batch"
	DurabilityPerWrite Durability = "per-write"
)

const (
	SSDDiskIOPS = 5000
	HDDDiskIOPS = 200
)

type OpCost struct {
	CPUMs      float64
	MemoryMB   float64
	DiskIOs    float64
	Sequential bool
}

type Profile struct {
	CPUCores        int
	MemoryMB        int
	DiskIOPS        int
	Read            OpCost
	Write           OpCost
	MaxConcurrency  int
	BufferPoolRatio float64
	Durability      Durability
}

type Block interface {
	Kind() string
	Name() string
	Profile() Profile
}

var Types []Block
