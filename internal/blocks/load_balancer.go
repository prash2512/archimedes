package blocks

import "math"

const (
	lbCPUPerReq   = 0.01
	lbPool        = 100000
	lbMaxConnTable = 80000.0 // connection table size before pressure
)

type LoadBalancer struct{}

func (LoadBalancer) Kind() string { return "load_balancer" }
func (LoadBalancer) Name() string { return "Load Balancer" }

func (LoadBalancer) Profile() Profile {
	return Profile{
		CPUCores: 1,
		MemoryMB: 1024,
		Read:     OpCost{CPUMs: lbCPUPerReq, MemoryMB: 0.001},
		Write:    OpCost{CPUMs: lbCPUPerReq, MemoryMB: 0.001},
		MaxConcurrency: lbPool,
	}
}

func (LoadBalancer) InitState(state map[string]float64) {
	state["conn_track_util"] = 0
}

// Connection tracking: active connections consume entries in the conntrack
// table. When the table fills, new connections stall.
func (LoadBalancer) Tick(ctx TickContext) TickEffect {
	totalRPS := (ctx.Reads + ctx.Writes) / ctx.Dt
	connUtil := math.Min(totalRPS/lbMaxConnTable, 1.0)
	ctx.State["conn_track_util"] = connUtil

	capMult := 1.0
	latency := lbCPUPerReq * 1000
	if connUtil > 0.75 {
		pressure := (connUtil - 0.75) / 0.25
		capMult = 1.0 - 0.3*pressure
		latency = lbCPUPerReq * 1000 * (1 + 2*pressure)
	}

	return TickEffect{
		CapMultiplier: capMult,
		Latency:       latency,
		Saturated:     connUtil > 0.95,
		Metrics:       map[string]float64{"conn_track_util": connUtil},
	}
}

func init() { Types = append(Types, LoadBalancer{}) }
