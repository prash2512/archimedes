package blocks

import "math"

const (
	cdnCPUPerReq   = 0.005
	cdnEdgePool    = 200000
	cdnCacheMemMB  = 8192
	cdnWarmupRate  = 0.02  // cache warms 2% per tick under load
	cdnDecayRate   = 0.005 // cache cools 0.5% per tick without traffic
)

type CDN struct{}

func (CDN) Kind() string { return "cdn" }
func (CDN) Name() string { return "CDN" }

func (CDN) Profile() Profile {
	return Profile{
		CPUCores:       1,
		MemoryMB:       cdnCacheMemMB,
		Read:           OpCost{CPUMs: cdnCPUPerReq, MemoryMB: 0.001},
		Write:          OpCost{CPUMs: cdnCPUPerReq, MemoryMB: 0.001},
		MaxConcurrency: cdnEdgePool,
	}
}

func (CDN) InitState(state map[string]float64) {
	state["hit_ratio"] = 0
}

// Cache hit ratio: reads warm the cache, idle time cools it.
// High hit ratio means most traffic is served at the edge.
func (CDN) Tick(ctx TickContext) TickEffect {
	ratio := ctx.State["hit_ratio"]

	total := ctx.Reads + ctx.Writes
	if total > 0 {
		ratio += cdnWarmupRate * (1 - ratio)
	} else {
		ratio -= cdnDecayRate * ratio
	}
	ratio = math.Max(0, math.Min(1, ratio))
	ctx.State["hit_ratio"] = ratio

	// High hit ratio reduces effective load (cache absorbs it).
	// Model as capacity multiplier: more hits = more headroom.
	capMult := 1.0 + ratio*4.0 // up to 5x effective capacity at 100% hit

	return TickEffect{
		CapMultiplier: capMult,
		Latency:       cdnCPUPerReq * 1000 * (1 - 0.8*ratio), // cache hits are faster
		Metrics:       map[string]float64{"hit_ratio": ratio},
	}
}

func init() { Types = append(Types, CDN{}) }
