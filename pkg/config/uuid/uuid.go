package main

// package to generate a uuidv7 at build time for the packaged Expo app

import (
	"flag"
	"fmt"
	"os"

	"github.com/google/uuid"
)

func main() {
	u, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}

	output := flag.String("o", "", "file to output to")

	flag.Parse()
	if *output != "" {
		if err := os.WriteFile(*output, []byte(u.String()), 0644); err != nil {
			panic(err)
		}
	} else {
		fmt.Printf("%s", u)
	}
}
