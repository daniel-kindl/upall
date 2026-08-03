package main

import (
	"os"

	"github.com/daniel-kindl/upall/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
