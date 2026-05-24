package sander

import (
	"os"
	"testing"
)

func TestParseArgumentsValueless(t *testing.T) {
	prev := os.Args
	t.Cleanup(func() { os.Args = prev })
	os.Args = []string{"fafi2", "--debug", "--port=8080"}

	args := ParseArguments()
	if v, ok := args["debug"]; !ok || v != "1" {
		t.Errorf("debug = (%q, %v), want (\"1\", true)", v, ok)
	}
	if v, ok := args["port"]; !ok || v != "8080" {
		t.Errorf("port = (%q, %v), want (\"8080\", true)", v, ok)
	}
}
