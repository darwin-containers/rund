module github.com/darwin-containers/rund

go 1.22

require (
	github.com/containerd/containerd/v2 v2.0.0-beta.2
	github.com/containerd/errdefs v0.1.0
	github.com/containerd/fifo v1.1.0
	github.com/containerd/log v0.1.0
	github.com/containerd/plugin v0.1.0
	github.com/containerd/ttrpc v1.2.3
	github.com/containerd/typeurl/v2 v2.1.1
	github.com/creack/pty v1.1.21
	github.com/hashicorp/go-multierror v1.1.1
	github.com/opencontainers/runtime-spec v1.2.0
	github.com/stretchr/testify v1.9.0
	golang.org/x/sys v0.19.0
)

require (
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/Microsoft/hcsshim v0.12.0-rc.2 // indirect
	github.com/containerd/cgroups/v3 v3.0.3 // indirect
	github.com/containerd/console v1.0.3 // indirect
	github.com/containerd/containerd v1.7.13 // indirect
	github.com/containerd/continuity v0.4.3 // indirect
	github.com/containerd/go-runc v1.1.0 // indirect
	github.com/containerd/platforms v0.1.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/klauspost/compress v1.17.4 // indirect
	github.com/moby/sys/mountinfo v0.7.1 // indirect
	github.com/moby/sys/sequential v0.5.0 // indirect
	github.com/moby/sys/user v0.1.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0-rc5 // indirect
	github.com/opencontainers/runtime-tools v0.9.1-0.20221107090550-2e043c6bd626 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635 // indirect
	go.opencensus.io v0.24.0 // indirect
	golang.org/x/mod v0.14.0 // indirect
	golang.org/x/net v0.19.0 // indirect
	golang.org/x/sync v0.6.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/tools v0.16.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231212172506-995d672761c0 // indirect
	google.golang.org/grpc v1.60.1 // indirect
	google.golang.org/protobuf v1.32.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
	tags.cncf.io/container-device-interface v0.6.2 // indirect
	tags.cncf.io/container-device-interface/specs-go v0.6.0 // indirect
)

replace github.com/containerd/containerd/v2 => github.com/darwin-containers/containerd/v2 v2.0.0-20240218142517-4e59b5b68c3e
