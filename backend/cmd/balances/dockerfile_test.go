package main

import (
	"os"
	"regexp"
	"testing"
)

// The image's build stage compiles with the Go that go.mod's toolchain line
// names, so a release binary is the one CI tested and govulncheck scanned.
// Dependabot bumps the two separately; this fails the PR that splits them.
func TestDockerfileGoMatchesToolchain(t *testing.T) {
	gomod, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	dockerfile, err := os.ReadFile("../../../Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	toolchain := regexp.MustCompile(`(?m)^toolchain go(\S+)$`).FindSubmatch(gomod)
	if toolchain == nil {
		t.Fatal("go.mod has no toolchain line")
	}
	image := regexp.MustCompile(`(?m)^FROM\b.*\bgolang:([0-9.]+)[@-]`).FindSubmatch(dockerfile)
	if image == nil {
		t.Fatal("Dockerfile has no golang:<version> build stage")
	}
	if string(image[1]) != string(toolchain[1]) {
		t.Errorf("Dockerfile builds with golang:%s, go.mod's toolchain is go%s", image[1], toolchain[1])
	}
}
