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
		MaxConcurrency:   cdnEdgePool,
		DefaultReadRatio: 0.95,
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

	capMult := 1.0 + ratio*4.0
	readFrac := ctx.Reads / math.Max(total, 1)
	absorbed := ratio * readFrac

	return TickEffect{
		CapMultiplier: capMult,
		AbsorbRatio:   absorbed,
		Latency:       cdnCPUPerReq * 1000 * (1 - 0.8*ratio),
		Metrics:       map[string]float64{"hit_ratio": ratio},
	}
}

func init() { Types = append(Types, CDN{}) }
