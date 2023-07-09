package containerd

import (
	"context"
	"fmt"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/runtime/v2/shim"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"
)

func NewManager(name string) shim.Manager {
	return manager{name: name}
}

type manager struct {
	name string
}

func (m manager) Name() string {
	return m.name
}

func (manager) Start(ctx context.Context, id string, opts shim.StartOpts) (_ string, retErr error) {
	cmd, err := newCommand(ctx, id, opts.Address, opts.Debug)
	if err != nil {
		return "", err
	}

	address, err := shim.SocketAddress(ctx, opts.Address, id)
	if err != nil {
		return "", err
	}

	socket, err := shim.NewSocket(address)
	if err != nil {
		if !shim.SocketEaddrinuse(err) {
			return "", fmt.Errorf("create new shim socket: %w", err)
		}
		if shim.CanConnect(address) {
			if err := shim.WriteAddress("address", address); err != nil {
				return "", fmt.Errorf("write existing socket for shim: %w", err)
			}
			return address, nil
		}
		if err := shim.RemoveSocket(address); err != nil {
			return "", fmt.Errorf("remove pre-existing socket: %w", err)
		}
		if socket, err = shim.NewSocket(address); err != nil {
			return "", fmt.Errorf("try create new shim socket 2x: %w", err)
		}
	}
	defer func() {
		if retErr != nil {
			_ = socket.Close()
			_ = shim.RemoveSocket(address)
		}
	}()

	if err := shim.WriteAddress("address", address); err != nil {
		return "", err
	}

	f, err := socket.File()
	if err != nil {
		return "", err
	}

	cmd.ExtraFiles = append(cmd.ExtraFiles, f)

	if err := cmd.Start(); err != nil {
		_ = f.Close()
		return "", err
	}

	defer func() {
		if retErr != nil {
			_ = cmd.Process.Kill()
		}
	}()
	go func() {
		_ = cmd.Wait()
	}()

	return address, nil
}

func (manager) Stop(ctx context.Context, id string) (shim.StopStatus, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return shim.StopStatus{}, err
	}

	bundlePath := filepath.Join(filepath.Dir(cwd), id)
	rootfs := path.Join(bundlePath, "rootfs")

	if err := mount.UnmountRecursive(rootfs, UnmountFlags); err != nil {
		log.G(ctx).WithError(err).Warn("failed to cleanup rootfs mount")
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
