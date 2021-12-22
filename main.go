// Copyright The RAI Inc.
// The RAI Authors
package main

import (
	"embed"

	"github.com/retail-ai-inc/bean/cmd"
)

// Diretory of template files.
//go:embed internal/*
var internalFS embed.FS

func main() {
	cmd.Execute(internalFS)
}
