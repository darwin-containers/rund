package main

import (
	"context"
	"github.com/containerd/containerd/runtime/v2/shim"
	"github.com/macOScontainers/rund/containerd"
)

func withoutReaper(config *shim.Config) {
	config.NoReaper = true
}

func main() {
	shim.Run(context.Background(), containerd.NewManager("io.containerd.rund.v2"), withoutReaper)
}
