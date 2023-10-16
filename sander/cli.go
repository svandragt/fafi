package sander

import (
	"fmt"
	"os"
	"strings"
)

func ParseArguments() map[string]string {
	arguments := make(map[string]string)

	// Iterate through the command-line arguments
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if strings.HasPrefix(arg, "--") {
			// Long-named argument
			parts := strings.SplitN(arg[2:], "=", 2)
			if len(parts) == 2 {
				arguments[parts[0]] = parts[1]
			} else {
				fmt.Printf("Invalid argument format: %s\n", arg)
			}
		}
	}

	return arguments
}
