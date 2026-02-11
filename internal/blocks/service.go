package blocks

type Service struct{}

func (Service) Kind() string { return "service" }
func (Service) Name() string { return "Service" }

func init() { Types = append(Types, Service{}) }
