package blocks

type Block struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

var Catalog = []Block{
	{Kind: "user", Name: "User"},
	{Kind: "load_balancer", Name: "Load Balancer"},
	{Kind: "api_gateway", Name: "API Gateway"},
	{Kind: "service", Name: "Service"},
	{Kind: "sql_datastore", Name: "SQL Datastore"},
}
