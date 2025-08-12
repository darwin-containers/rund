package containerd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"

	"github.com/containerd/containerd/api/types"
	"github.com/containerd/containerd/v2/core/mount"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/containerd/v2/pkg/shim"
	"github.com/containerd/log"
)

func NewManager(name string) shim.Manager {
	return &manager{name: name}
}

type manager struct {
	name string
}

func (m *manager) Info(_ context.Context, _ io.Reader) (*types.RuntimeInfo, error) {
	info := &types.RuntimeInfo{
		Name: m.Name(),
		Version: &types.RuntimeVersion{
			Version: Version,
		},
	}
	return info, nil
}

func (m *manager) Name() string {
	return m.name
}

func (*manager) Start(ctx context.Context, id string, opts shim.StartOpts) (params shim.BootstrapParams, retErr error) {
	params.Version = 3
	params.Protocol = "ttrpc"

	cmd, err := newCommand(ctx, id, opts.Address, opts.Debug)
	if err != nil {
		return params, err
	}

	address, err := shim.SocketAddress(ctx, opts.Address, id, false)
	if err != nil {
		return params, err
	}

	socket, err := shim.NewSocket(address)
	if err != nil {
		if !shim.SocketEaddrinuse(err) {
			return params, fmt.Errorf("create new shim socket: %w", err)
		}
		if shim.CanConnect(address) {
			params.Address = address
			return params, nil
		}
		if err := shim.RemoveSocket(address); err != nil {
			return params, fmt.Errorf("remove pre-existing socket: %w", err)
		}
		if socket, err = shim.NewSocket(address); err != nil {
			return params, fmt.Errorf("try create new shim socket 2x: %w", err)
		}
	}
	defer func() {
		if retErr != nil {
			_ = socket.Close()
			_ = shim.RemoveSocket(address)
		}
	}()

	f, err := socket.File()
	if err != nil {
		return params, err
	}

	cmd.ExtraFiles = append(cmd.ExtraFiles, f)

	if err := cmd.Start(); err != nil {
		_ = f.Close()
		return params, err
	}

	defer func() {
		if retErr != nil {
			_ = cmd.Process.Kill()
		}
	}()
	go func() {
		_ = cmd.Wait()
	}()

	params.Address = address
	return params, nil
}

func (*manager) Stop(ctx context.Context, id string) (shim.StopStatus, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return shim.StopStatus{}, err
	}

	bundlePath := filepath.Join(filepath.Dir(cwd), id)

	spec, err := oci.ReadSpec(path.Join(bundlePath, oci.ConfigFilename))
	if err == nil {
		if err = mount.UnmountRecursive(spec.Root.Path, unmountFlags); err != nil {
			log.G(ctx).WithError(err).Warn("failed to cleanup rootfs mount")
		}
	}

	return shim.StopStatus{
		ExitedAt: time.Now(),
		// TODO
	}, nil
}

func newCommand(ctx context.Context, id, containerdAddress string, debug bool) (*exec.Cmd, error) {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}

	self, err := os.Executable()
	if err != nil {
		return nil, err
	}

	args := []string{
		"-namespace", ns,
		"-id", id,
		"-address", containerdAddress,
	}

	if debug {
		args = append(args, "-debug")
	}

	cmd := exec.Command(self, args...)

	return cmd, nil
}
