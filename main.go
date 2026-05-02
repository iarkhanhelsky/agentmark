package main

import (
	"errors"
	"log"
	"os"

	"agentmark/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		if errors.Is(err, cli.ErrAlreadyReported) {
			os.Exit(1)
		}
		log.Fatal(err)
	}
}
