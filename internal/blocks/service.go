package blocks

const (
	serviceCPUPerReq = 0.5
	serviceMemPerReq = 0.5
	goroutinePool    = 10000
)

type Service struct{}

func (Service) Kind() string { return "service" }
func (Service) Name() string { return "Service" }

func (Service) Profile() Profile {
	return Profile{
		CPUCores: 2,
		MemoryMB: 4096,
		Read:     OpCost{CPUMs: serviceCPUPerReq, MemoryMB: serviceMemPerReq},
		Write:    OpCost{CPUMs: serviceCPUPerReq, MemoryMB: serviceMemPerReq},
		MaxConcurrency: goroutinePool,
	}
}

func init() { Types = append(Types, Service{}) }
