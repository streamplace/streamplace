package main

import (
	"fmt"

	v0 "stream.place/streamplace/pkg/schema/v0"
)

func main() {
	err := Main()
	if err != nil {
		panic(err)
	}
}

// Exports the generated EIP-712 schema for use elsewhere
func Main() error {
	schema, err := v0.MakeV0Schema()
	if err != nil {
		return err
	}
	eipSchema, err := schema.EIP712()
	if err != nil {
		return err
	}
	bs, err := eipSchema.JSON()
	if err != nil {
		return err
	}
	fmt.Println(string(bs))
	return nil
}
