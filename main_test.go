package main

import (
	"fmt"
	"runtime"
	"testing"
)

func TestBuildVersionOutputAddsVPrefixAndMetadata(t *testing.T) {
	got := buildVersionOutput(ProjectName, "1.2.3")
	want := fmt.Sprintf("%s version v1.2.3 (%s, %s/%s)", ProjectName, runtime.Version(), runtime.GOOS, runtime.GOARCH)

	if got != want {
		t.Fatalf("unexpected version output: got %q, want %q", got, want)
	}
}

func TestBuildVersionOutputPreservesExistingVPrefix(t *testing.T) {
	got := buildVersionOutput(ProjectName, "v1.2.3")
	want := fmt.Sprintf("%s version v1.2.3 (%s, %s/%s)", ProjectName, runtime.Version(), runtime.GOOS, runtime.GOARCH)

	if got != want {
		t.Fatalf("unexpected version output: got %q, want %q", got, want)
	}
}

func TestBuildVersionOutputNoVPrefixForDev(t *testing.T) {
	got := buildVersionOutput(ProjectName, "dev")
	want := fmt.Sprintf("%s version dev (%s, %s/%s)", ProjectName, runtime.Version(), runtime.GOOS, runtime.GOARCH)

	if got != want {
		t.Fatalf("unexpected version output: got %q, want %q", got, want)
	}
}

func TestBuildVersionOutputAddsVPrefixForPrerelease(t *testing.T) {
	got := buildVersionOutput(ProjectName, "1.2.3-alpha.1")
	want := fmt.Sprintf("%s version v1.2.3-alpha.1 (%s, %s/%s)", ProjectName, runtime.Version(), runtime.GOOS, runtime.GOARCH)

	if got != want {
		t.Fatalf("unexpected version output: got %q, want %q", got, want)
	}
}
