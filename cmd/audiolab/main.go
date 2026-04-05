package main

import (
	"fmt"
	"os"

	"voidnet/internal/audiolab"
)

func main() {
	if err := audiolab.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
