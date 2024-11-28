package containerd

import (
	"golang.org/x/sys/unix"
	"os"
	"syscall"
	"time"
)

// See https://jmmv.dev/2019/11/wait-for-process-group-darwin.html
// See https://github.com/bazelbuild/bazel/commit/b07b799cdf25661f1d59a8fd9d941c702886d3b8
// See https://chromium.googlesource.com/chromium/src/base/+/master/process/kill_mac.cc

func waitUntilZombie(process *os.Process) error {
	kq, err := unix.Kqueue()
	if err != nil {
		return err
	}
	defer unix.Close(kq)

	changes := []unix.Kevent_t{
		{
			Ident:  uint64(process.Pid),
			Filter: unix.EVFILT_PROC,
			Flags:  unix.EV_ADD,
			Fflags: unix.NOTE_EXIT,
		},
	}

	ke := []unix.Kevent_t{
		{},
	}

	_, err = unix.Kevent(kq, changes, ke, nil)
	return err
}

func waitForProcessGroup(process *os.Process) error {
	for {
		procs, err := unix.SysctlKinfoProcSlice("kern.proc.pgrp", process.Pid)
		if err != nil {
			return err
		}

		if len(procs) <= 1 {
			return nil
		}

		_ = syscall.Kill(-process.Pid, syscall.SIGTERM)

		time.Sleep(10 * time.Millisecond)
	}
}

func wait(process *os.Process) (*os.ProcessState, error) {
	if err := waitUntilZombie(process); err != nil {
		return nil, err
	}

	if err := waitForProcessGroup(process); err != nil {
		return nil, err
	}

	return process.Wait()
}
