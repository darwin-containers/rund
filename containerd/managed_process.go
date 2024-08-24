package containerd

import (
	"context"
	"github.com/containerd/containerd/api/types/task"
	"github.com/creack/pty"
	"github.com/hashicorp/go-multierror"
	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type managedProcess struct {
	spec       *specs.Process
	io         stdio
	console    *os.File
	mu         sync.Mutex
	cmd        *exec.Cmd
	waitblock  chan struct{}
	status     task.Status
	exitStatus uint32
	exitedAt   time.Time
}

func (p *managedProcess) getConsoleL() *os.File {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.console
}

func (p *managedProcess) destroy() (retErr error) {
	// TODO: Do we care about error?
	_ = p.kill(syscall.SIGKILL)

	if err := p.io.Close(); err != nil {
		retErr = multierror.Append(retErr, err)
	}

	if p.console != nil {
		if err := p.console.Close(); err != nil {
			retErr = multierror.Append(retErr, err)
		}
	}

	if p.status != task.Status_STOPPED {
		p.status = task.Status_STOPPED
		p.exitedAt = time.Now()
		p.exitStatus = uint32(syscall.SIGKILL)
	}

	return
}

func (p *managedProcess) kill(signal syscall.Signal) error {
	if p.cmd != nil {
		if process := p.cmd.Process; p != nil {
			return unix.Kill(-process.Pid, signal)
		}
	}

	return nil
}

func (p *managedProcess) setup(ctx context.Context, rootfs string, stdin string, stdout string, stderr string) error {
	var err error

	p.io, err = setupIO(ctx, stdin, stdout, stderr)
	if err != nil {
		return err
	}

	if len(p.spec.Args) <= 0 {
		// TODO: How to handle this properly?
		p.spec.Args = []string{"/bin/sh"}
		// return fmt.Errorf("args must not be empty")
	}

	p.cmd = exec.Command(p.spec.Args[0])
	p.cmd.Args = p.spec.Args
	p.cmd.Dir = p.spec.Cwd
	p.cmd.Env = p.spec.Env
	p.cmd.SysProcAttr = &syscall.SysProcAttr{
		Chroot: rootfs,
		Credential: &syscall.Credential{
			Uid: p.spec.User.UID,
			Gid: p.spec.User.GID,
		},
	}

	return nil
}

func (p *managedProcess) start() (err error) {
	if p.spec.Terminal {
		// TODO: I'd like to use containerd/console package instead
		// But see https://github.com/containerd/console/issues/79
		var consoleSize *pty.Winsize
		if p.spec.ConsoleSize != nil {
			consoleSize = &pty.Winsize{
				Cols: uint16(p.spec.ConsoleSize.Width),
				Rows: uint16(p.spec.ConsoleSize.Height),
			}
		}

		p.console, err = pty.StartWithSize(p.cmd, consoleSize)
		if err != nil {
			return err
		}

		go io.Copy(p.console, p.io.stdin)
		go io.Copy(p.io.stdout, p.console)
	} else {
		p.cmd.SysProcAttr.Setpgid = true
		p.cmd.Stdin = p.io.stdin
		p.cmd.Stdout = p.io.stdout
		p.cmd.Stderr = p.io.stderr

		err = p.cmd.Start()
		if err != nil {
			return err
		}
	}

	p.status = task.Status_RUNNING

	return nil
}
