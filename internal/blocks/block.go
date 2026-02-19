package blocks

type Durability string

const (
	DurabilityNone     Durability = "none"
	DurabilityBatch    Durability = "batch"
	DurabilityPerWrite Durability = "per-write"
)

const (
	SSDDiskIOPS = 50000 // modern NVMe
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

// Ticker is an optional interface blocks can implement for custom per-tick
// simulation behavior (e.g. memory eviction, connection pool exhaustion).
type Ticker interface {
	InitState(state map[string]float64)
	Tick(ctx TickContext) TickEffect
}

type TickContext struct {
	Reads  float64            // read requests arriving this tick
	Writes float64            // write requests arriving this tick
	RawCap float64            // base capacity for this tick (capacity * dt)
	Dt     float64            // tick duration in seconds
	State  map[string]float64 // mutable per-block state (persists across ticks)
	Tick   int                // current tick number
}

type TickEffect struct {
	CapMultiplier float64
	AbsorbRatio   float64 // fraction of processed traffic not forwarded downstream
	Latency       float64
	Saturated     bool
	Metrics       map[string]float64
}

var Types []Block

func ByKind(kind string) (Block, bool) {
	for _, b := range Types {
		if b.Kind() == kind {
			return b, true
		}
	}
	return nil, false
}
