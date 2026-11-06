package main

import (
	"errors"
	"strings"
	"testing"
)

func TestProgress(t *testing.T) {
	var out, errOut strings.Builder
	p := newProgress(&out, &errOut)
	p.tty = true // a strings.Builder is no terminal
	p.start("", 2)
	p.at("lever:a")
	p.step()
	p.fail(errors.New("lever:b: down"))
	p.Write([]byte("result\n"))
	p.close()
	p.step() // after close, draws nothing
	const cl = "\r\x1b[K"
	bar := func(n int, label string) string {
		return cl + "[" + strings.Repeat("=", n) + strings.Repeat(" ", barWidth-n) + "] " + label
	}
	want := bar(0, "0/2 ") + bar(0, "0/2 lever:a") + bar(barWidth/2, "1/2 lever:a") + // start, at, step
		cl + "hirectl: lever:b: down\n" + bar(barWidth/2, "1/2 lever:a") + // fail
		cl + bar(barWidth/2, "1/2 lever:a") + // Write: result on out
		cl // close
	if out.String() != "result\n" || errOut.String() != want {
		t.Fatalf("out %q\nerrOut %q\nwant   %q", out.String(), errOut.String(), want)
	}
}
