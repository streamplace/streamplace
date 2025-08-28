package config

import (
	"flag"

	"github.com/livepeer/go-livepeer/cmd/livepeer/starter"
)

type LivepeerFlagsStruct struct {
	SnakeToCamel map[string]string
	CamelToSnake map[string]string
}

var LivepeerFlags = LivepeerFlagsStruct{
	SnakeToCamel: make(map[string]string),
	CamelToSnake: make(map[string]string),
}

func init() {
	lpFlags := flag.NewFlagSet("livepeer", flag.ContinueOnError)
	_ = starter.NewLivepeerConfig(lpFlags)
	lpFlags.VisitAll(func(f *flag.Flag) {
		snake := ToSnakeCase(f.Name)
		LivepeerFlags.SnakeToCamel[snake] = f.Name
		LivepeerFlags.CamelToSnake[f.Name] = snake
	})
}
