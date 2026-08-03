package main

import (
	"os"

	"github.com/PayCal-Technologies/clyde/internal/clyde"
)

func main() {
	os.Exit(clyde.MainWithInput(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
