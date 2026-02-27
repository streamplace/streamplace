package main

import (
	"fmt"
	"go/format"
	"os"
	"strings"
)

// If you need to gen any CBOR stubs, put the type names here and in gen.go.
var types = []string{
	"RichtextFacet_Emoji",
}

func main() {
	var sb strings.Builder
	sb.WriteString("package streamplace\n\nimport \"io\"\n\n")
	for _, name := range types {
		fmt.Fprintf(&sb, "func (t *%s) MarshalCBOR(w io.Writer) error   { return nil }\n", name)
		fmt.Fprintf(&sb, "func (t *%s) UnmarshalCBOR(r io.Reader) error { return nil }\n\n", name)
	}

	src, err := format.Source([]byte(sb.String()))
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("pkg/streamplace/cbor_gen.go", src, 0644); err != nil {
		panic(err)
	}
	fmt.Printf("wrote CBOR stubs for %d types\n", len(types))
}
