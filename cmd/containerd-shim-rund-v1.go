package main

import (
	"context"
	"github.com/containerd/containerd/runtime/v2/shim"
	"github.com/macOScontainers/rund/containerd"
	_ "github.com/macOScontainers/rund/containerd/plugin"
)

func main() {
	shim.Run(context.Background(), containerd.NewManager("io.containerd.rund.v2"))
}
