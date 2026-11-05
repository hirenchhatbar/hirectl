package main

import (
	"strings"
	"testing"
)

func TestNewFlagSetUsageDoubleDash(t *testing.T) {
	fs := newFlagSet("run")
	fs.String("feeds", "", "feed IDs\ne.g. --feeds go-remote")
	var b strings.Builder
	fs.SetOutput(&b)
	fs.Usage()
	if got := b.String(); !strings.HasPrefix(got, "Usage of hirectl run:\n  --feeds string\n") || strings.Contains(got, "\n  -feeds") {
		t.Errorf("usage = %q", got)
	}
}
