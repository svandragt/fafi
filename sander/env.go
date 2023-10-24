package sander

import (
	"log"
	"os"
)

var ParsedArgs map[string]string

func GetArgFromEnvWithDefault(envKey, defaultValue string) string {
	// Commandline arguments (inline) are prioritised over environment variables.
	if ParsedArgs == nil {
		ParsedArgs = ParseArguments()
	}
	argName := ConvertToCmdLineKey(envKey)
	argValue, present := ParsedArgs[argName]
	if present {
		if Debug {
			log.Printf("ARG\t--%s=%s\n", argName, argValue)
		}
		return argValue
	}

	// Environment variables are prioritised over default values.
	envValue := os.Getenv(envKey)
	if envValue != "" {
		if Debug {
			log.Printf("ENV\t%s=%s\n", envKey, envValue)
		}
		return envValue
	}

	if Debug {
		log.Printf("DEF\t%s=%s\n", envKey, defaultValue)
	}
	return defaultValue
}
