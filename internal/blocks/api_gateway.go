package blocks

type APIGateway struct{}

func (APIGateway) Kind() string { return "api_gateway" }
func (APIGateway) Name() string { return "API Gateway" }

func init() { Types = append(Types, APIGateway{}) }
