package main

import (
	"strings"
	"testing"
)

func TestPrintCrossCheck(t *testing.T) {
	stored := []Job{{Board: "lever:a"}, {Board: "lever:a"}, {Board: "lever:old"}, {Board: "greenhouse:x"}}
	online := []string{"ashby:p", "greenhouse:x", "lever:a", "lever:b", "lever:c", "workable:q"}
	var out strings.Builder
	printCrossCheck(&out, []string{"ashby", "greenhouse", "lever", "workable"}, stored, online)
	want := "ashby\t0 stored\t1 online\t1 not stored\ngreenhouse\t1 stored\t1 online\t0 not stored\nlever\t2 stored\t3 online\t2 not stored\nworkable\t0 stored\t1 online\t1 not stored\n" +
		"\nhirectl discover --sources lever\nhirectl discover --sources ashby\nhirectl discover --sources workable\n"
	if out.String() != want {
		t.Errorf("got %q, want %q", out.String(), want)
	}
	out.Reset()
	printCrossCheck(&out, []string{"greenhouse"}, stored, online)
	if want := "greenhouse\t1 stored\t1 online\t0 not stored\n"; out.String() != want {
		t.Errorf("none missing: got %q, want %q", out.String(), want)
	}
}
