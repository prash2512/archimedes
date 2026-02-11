package blocks

type Block interface {
	Kind() string
	Name() string
}

var Types []Block
