package main

import (
	"context"
	"github.com/containerd/containerd/runtime/v2/shim"
	"github.com/slonopotamus/rund/containerd"
	_ "github.com/slonopotamus/rund/containerd/plugin"
)

func main() {
	shim.Run(context.Background(), containerd.NewManager("io.containerd.rund.v2"))
}
