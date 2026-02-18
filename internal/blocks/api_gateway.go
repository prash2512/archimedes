package blocks

import "math"

const (
	gwCPUPerReq  = 0.05
	gwPool       = 50000
	gwRateLimit  = 40000.0 // requests per second before throttling
)

type APIGateway struct{}

func (APIGateway) Kind() string { return "api_gateway" }
func (APIGateway) Name() string { return "API Gateway" }

func (APIGateway) Profile() Profile {
	return Profile{
		CPUCores: 2,
		MemoryMB: 2048,
		Read:     OpCost{CPUMs: gwCPUPerReq, MemoryMB: 0.01},
		Write:    OpCost{CPUMs: gwCPUPerReq, MemoryMB: 0.01},
		MaxConcurrency: gwPool,
	}
}

func (APIGateway) InitState(state map[string]float64) {
	state["rate_util"] = 0
}

// Rate limiting: tracks request rate against a limit. Past the threshold,
// the gateway starts rejecting requests and adds auth/routing latency.
func (APIGateway) Tick(ctx TickContext) TickEffect {
	totalRPS := (ctx.Reads + ctx.Writes) / ctx.Dt
	rateUtil := math.Min(totalRPS/gwRateLimit, 1.0)
	ctx.State["rate_util"] = rateUtil

	capMult := 1.0
	latency := gwCPUPerReq * 1000
	if rateUtil > 0.8 {
		// Rate limiter kicks in, rejecting excess traffic
		pressure := (rateUtil - 0.8) / 0.2
		capMult = 1.0 - 0.5*pressure
		latency = gwCPUPerReq * 1000 * (1 + 2*pressure)
	}

	return TickEffect{
		CapMultiplier: capMult,
		Latency:       latency,
		Saturated:     rateUtil > 0.95,
		Metrics:       map[string]float64{"rate_util": rateUtil},
	}
}

func init() { Types = append(Types, APIGateway{}) }
