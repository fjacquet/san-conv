package main

import "github.com/fjacquet/san-conv/cmd"

// Overridden at build time via -ldflags "-X main.version=... -X main.commit=...".
var (
	version = "dev"
	commit  = "none"
)

func main() {
	cmd.Execute(version, commit)
}
