package main

import (
	"runtime/debug"
	"testing"
)

func buildInfo(version string, ok bool) func() (*debug.BuildInfo, bool) {
	return func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: version}}, ok
	}
}

func TestResolveVersion(t *testing.T) {
	cases := []struct {
		label  string
		linked string
		info   func() (*debug.BuildInfo, bool)
		want   string
	}{
		{"a release build", "0.1.1", buildInfo("v9.9.9", true), "0.1.1"},
		{"go install of a tag", "dev", buildInfo("v0.1.1", true), "0.1.1"},
		{"a local build", "dev", buildInfo("(devel)", true), "dev"},
		{"no build info", "dev", buildInfo("", false), "dev"},
	}
	for _, tc := range cases {
		if got := resolveVersion(tc.linked, tc.info); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.label, got, tc.want)
		}
	}
}
