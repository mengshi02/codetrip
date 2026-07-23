package main

import (
	"strings"
	"testing"
)

func TestRootCommandsUseSingleWords(t *testing.T) {
	root := newRootCmd(newCLIFlags())
	want := map[string]bool{
		"check": false, "context": false, "delete": false, "diff": false, "embed": false, "export": false, "hybrid": false,
		"impact": false, "index": false, "list": false, "mcp": false, "path": false, "search": false,
		"rename": false, "source": false, "traverse": false, "version": false,
	}
	for _, command := range root.Commands() {
		name := command.Name()
		if strings.Contains(name, "-") {
			t.Errorf("root command %q is not a single word", name)
		}
		if _, ok := want[name]; ok {
			want[name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("root command %q is missing", name)
		}
	}
}
