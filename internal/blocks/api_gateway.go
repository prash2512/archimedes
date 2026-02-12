package blocks

const (
	gwCPUPerReq = 0.05
	gwPool      = 50000
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

func init() { Types = append(Types, APIGateway{}) }
