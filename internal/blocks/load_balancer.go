package blocks

const (
	lbCPUPerReq = 0.01
	lbPool      = 100000
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

func init() { Types = append(Types, LoadBalancer{}) }
