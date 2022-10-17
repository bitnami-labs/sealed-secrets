package main

import (
	"bytes"
	goflag "flag"
	"testing"

	flag "github.com/spf13/pflag"
)

func TestVersion(t *testing.T) {
	buf := bytes.NewBufferString("")
	testVersionFlags := flag.NewFlagSet("testVersionFlags", flag.ExitOnError)
	nopFlags := goflag.NewFlagSet("nop", goflag.ExitOnError)
	err := mainE(buf, testVersionFlags, nopFlags, []string{"--version"})
	if err != nil {
		t.Fatal(err)
	}

	if got, want := buf.String(), "kubeseal version: UNKNOWN\n"; got != want {
		t.Errorf("got: %q, want: %q", got, want)
	}
}
