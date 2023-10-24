package sander

import "strings"

func Pluralize(text string, num int) string {
	if num == 1 {
		return text
	}
	return text + "s"
}

// ConvertToCmdLineKey Takes `FAFI_EXAMPLE_THING` and returns `example-thing`
func ConvertToCmdLineKey(environmentKey string) string {
	// Replace underscores with spaces and split into words
	index := strings.Index(environmentKey, "_")
	if index != -1 {
		environmentKey = environmentKey[index+1:]
	}
	words := strings.Split(strings.ReplaceAll(environmentKey, "_", " "), " ")

	// Join the words with hyphens and convert to lowercase
	result := strings.Join(words, "-")
	result = strings.ToLower(result)

	return result
}
