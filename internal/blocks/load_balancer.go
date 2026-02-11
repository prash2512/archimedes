package blocks

type LoadBalancer struct{}

func (LoadBalancer) Kind() string { return "load_balancer" }
func (LoadBalancer) Name() string { return "Load Balancer" }

func init() { Types = append(Types, LoadBalancer{}) }
