package containerd

import (
	"errors"
	"golang.org/x/sys/unix"
	"os"
)

func wait(process *os.Process) (*os.ProcessState, error) {
	_, err := unix.Wait4(-process.Pid, nil, 0, nil)

	if err != nil && !errors.Is(err, unix.ECHILD) {
		return nil, err
	}

	return process.Wait()
}
