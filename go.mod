module github.com/darwin-containers/rund

go 1.23.0

require (
	github.com/containerd/containerd/api v1.8.0
	github.com/containerd/containerd/v2 v2.0.1
	github.com/containerd/errdefs v1.0.0
	github.com/containerd/errdefs/pkg v0.3.0
	github.com/containerd/fifo v1.1.0
	github.com/containerd/log v0.1.0
	github.com/containerd/plugin v1.0.0
	github.com/containerd/ttrpc v1.2.7
	github.com/containerd/typeurl/v2 v2.2.3
	github.com/creack/pty v1.1.24
	github.com/hashicorp/go-multierror v1.1.1
	github.com/opencontainers/runtime-spec v1.2.1
	github.com/stretchr/testify v1.10.0
	golang.org/x/sys v0.33.0
)

require (
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/Microsoft/hcsshim v0.12.9 // indirect
	github.com/containerd/cgroups/v3 v3.0.4 // indirect
	github.com/containerd/console v1.0.4 // indirect
	github.com/containerd/continuity v0.4.5 // indirect
	github.com/containerd/go-runc v1.1.0 // indirect
	github.com/containerd/platforms v1.0.0-rc.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/mdlayher/socket v0.5.1 // indirect
	github.com/mdlayher/vsock v1.2.1 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/moby/sys/user v0.3.0 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0 // indirect
	github.com/opencontainers/runtime-tools v0.9.1-0.20221107090550-2e043c6bd626 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635 // indirect
	go.opencensus.io v0.24.0 // indirect
	golang.org/x/mod v0.22.0 // indirect
	golang.org/x/net v0.32.0 // indirect
	golang.org/x/sync v0.10.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241209162323-e6fa225c2576 // indirect
	google.golang.org/grpc v1.69.0 // indirect
	google.golang.org/protobuf v1.35.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
	tags.cncf.io/container-device-interface v0.8.0 // indirect
	tags.cncf.io/container-device-interface/specs-go v0.8.0 // indirect
)

replace github.com/containerd/containerd/v2 => github.com/darwin-containers/containerd/v2 v2.0.0-20241214113315-4850a67a9a04

replace github.com/containerd/containerd/api => github.com/darwin-containers/containerd/api v0.0.0-20241214113315-4850a67a9a04
