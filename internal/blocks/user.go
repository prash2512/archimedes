package blocks

type User struct{}

func (User) Kind() string { return "user" }
func (User) Name() string { return "User" }

func init() { Types = append(Types, User{}) }
