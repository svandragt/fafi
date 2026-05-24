package sander

import (
	"os"
	"strings"
)

func ParseArguments() map[string]string {
	arguments := make(map[string]string)

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if !strings.HasPrefix(arg, "--") {
			continue
		}
		parts := strings.SplitN(arg[2:], "=", 2)
		if len(parts) == 2 {
			arguments[parts[0]] = parts[1]
		} else {
			// Valueless flag (e.g. --debug) is treated as truthy "1" so it
			// matches the same convention as FAFI_FOO=1 env vars.
			arguments[parts[0]] = "1"
		}
	}

	return arguments
}
