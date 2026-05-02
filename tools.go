//go:build tools
// +build tools

// Package tools pins development tool dependencies in go.mod so all contributors
// build the same tool versions. This file is excluded from regular builds via
// the `tools` build tag and is never compiled into the server binary.
//
// To install pinned tools locally, run `make tools-install` (which uses
// `go install` with explicit versions). After `go mod tidy -compat=1.25` (run
// with the `tools` tag enabled), these imports will appear in go.mod.
package tools

import (
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "golang.org/x/vuln/cmd/govulncheck"
	_ "honnef.co/go/tools/cmd/staticcheck"
)
