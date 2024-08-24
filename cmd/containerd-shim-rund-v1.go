package main

import (
	"context"
	"github.com/containerd/containerd/v2/pkg/shim"
	"github.com/darwin-containers/rund/containerd"
)

func withoutReaper(config *shim.Config) {
	config.NoReaper = true
}

func main() {
	shim.Run(context.Background(), containerd.NewManager("io.containerd.rund.v2"), withoutReaper)
}
