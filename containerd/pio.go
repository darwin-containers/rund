package containerd

import (
	"context"
	"github.com/containerd/fifo"
	"io"
	"os"
	"syscall"
)

type stdio struct {
	stdin  io.ReadWriteCloser
	stdout io.ReadWriteCloser
	stderr io.ReadWriteCloser
}

func setupIO(ctx context.Context, stdin, stdout, stderr string) (stdio, error) {
	s := stdio{}
	if _, err := os.Stat(stdin); err == nil {
		s.stdin, err = fifo.OpenFifo(ctx, stdin, syscall.O_RDONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			return s, err
		}
	}
	if _, err := os.Stat(stdout); err == nil {
		s.stdout, err = fifo.OpenFifo(ctx, stdout, syscall.O_WRONLY, 0)
		if err != nil {
			return s, err
		}
	}
	if _, err := os.Stat(stderr); err == nil {
		s.stderr, err = fifo.OpenFifo(ctx, stderr, syscall.O_WRONLY, 0)
		if err != nil {
			return s, err
		}
	}
	return s, nil
}

func (s stdio) Close() error {
	if s.stdin != nil {
		_ = s.stdin.Close()
	}
	if s.stdout != nil {
		_ = s.stdout.Close()
	}
	if s.stderr != nil {
		_ = s.stderr.Close()
	}
	return nil
}
