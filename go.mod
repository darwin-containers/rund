module github.com/darwin-containers/rund

go 1.25.0

require (
	github.com/containerd/containerd/api v1.9.0
	github.com/containerd/containerd/v2 v2.1.6
	github.com/containerd/errdefs v1.0.0
	github.com/containerd/errdefs/pkg v0.3.0
	github.com/containerd/fifo v1.1.0
	github.com/containerd/log v0.1.0
	github.com/containerd/plugin v1.1.0
	github.com/containerd/ttrpc v1.2.9
	github.com/containerd/typeurl/v2 v2.2.3
	github.com/creack/pty v1.1.24
	github.com/opencontainers/runtime-spec v1.3.0
	github.com/stretchr/testify v1.11.1
	golang.org/x/sys v0.47.0
)

require (
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/Microsoft/hcsshim v0.14.0-rc.1 // indirect
	github.com/containerd/cgroups/v3 v3.1.0 // indirect
	github.com/containerd/console v1.0.5 // indirect
	github.com/containerd/continuity v0.4.5 // indirect
	github.com/containerd/go-runc v1.1.0 // indirect
	github.com/containerd/platforms v1.0.0-rc.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/klauspost/compress v1.18.1 // indirect
	github.com/mdlayher/socket v0.5.1 // indirect
	github.com/mdlayher/vsock v1.2.1 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/moby/sys/user v0.4.0 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	go.opencensus.io v0.24.0 // indirect
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260226221140-a57be14db171 // indirect
	google.golang.org/grpc v1.81.1 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/containerd/containerd/v2 => github.com/darwin-containers/containerd/v2 v2.0.0-20260712142009-3a65cb8a2dde

replace github.com/containerd/containerd/api => github.com/darwin-containers/containerd/api v0.0.0-20260712142009-3a65cb8a2dde
