package containerd

import (
	"context"
	"fmt"
	"github.com/containerd/containerd/api/events"
	taskAPI "github.com/containerd/containerd/api/runtime/task/v2"
	"github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/pkg/shutdown"
	ptypes "github.com/containerd/containerd/protobuf/types"
	"github.com/containerd/containerd/runtime"
	"github.com/containerd/containerd/runtime/v2/shim"
	"github.com/containerd/ttrpc"
	"github.com/creack/pty"
	"golang.org/x/sys/unix"
	"io"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sync"
	"syscall"
)

func NewTaskService(ctx context.Context, publisher shim.Publisher, sd shutdown.Service) (taskAPI.TaskService, error) {
	s := service{
		containers: make(map[string]*container),
		sd:         sd,
		events:     make(chan interface{}, 128),
	}

	go s.forward(ctx, publisher)
	return &s, nil
}

type service struct {
	mu         sync.Mutex
	containers map[string]*container
	events     chan interface{}
	sd         shutdown.Service
}

func (s *service) forward(ctx context.Context, publisher shim.Publisher) {
	ns, _ := namespaces.Namespace(ctx)
	ctx = namespaces.WithNamespace(context.Background(), ns)
	for e := range s.events {
		err := publisher.Publish(ctx, runtime.GetTopic(e), e)
		if err != nil {
			log.G(ctx).WithError(err).Error("post event")
		}
	}
	_ = publisher.Close()
}

func (s *service) getContainer(id string) (*container, error) {
	c := s.containers[id]
	if c == nil {
		return nil, errdefs.ToGRPCf(errdefs.ErrNotFound, "container not created")
	}
	return c, nil
}

func (s *service) getContainerL(id string) (*container, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.getContainer(id)
}

func (s *service) RegisterTTRPC(server *ttrpc.Server) error {
	taskAPI.RegisterTaskService(server, s)
	return nil
}

func (s *service) State(ctx context.Context, request *taskAPI.StateRequest) (*taskAPI.StateResponse, error) {
	log.G(ctx).WithField("request", request).Info("STATE")
	defer log.G(ctx).Info("STATE_DONE")

	if request.ExecID != "" {
		return nil, errdefs.ErrNotImplemented
	}

	c, err := s.getContainerL(request.ID)
	if err != nil {
		return nil, err
	}

	var pid int
	if c, err := s.getContainerL(request.ID); err == nil {
		if p := c.cmd.Process; p != nil {
			pid = p.Pid
		}
	}

	return &taskAPI.StateResponse{
		ID:       request.ID,
		Bundle:   c.bundlePath,
		Pid:      uint32(pid),
		Status:   c.status,
		Terminal: c.spec.Process.Terminal,
		ExecID:   request.ExecID, // TODO
	}, nil
}

func (s *service) Create(ctx context.Context, request *taskAPI.CreateTaskRequest) (_ *taskAPI.CreateTaskResponse, retErr error) {
	log.G(ctx).WithField("request", request).Info("CREATE")
	defer log.G(ctx).Info("CREATE_DONE")

	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	spec, err := oci.ReadSpec(path.Join(request.Bundle, oci.ConfigFilename))
	if err != nil {
		return nil, err
	}

	rootfs, err := mount.CanonicalizePath(spec.Root.Path)
	if err != nil {
		return nil, err
	}

	if err = os.MkdirAll(path.Join(rootfs, "var", "run"), 0o755); err != nil {
		return nil, err
	}

	// Workaround for 104-char limit of UNIX socket path
	shortenedRootfsPath, err := filepath.Rel(wd, path.Join(rootfs))
	if err != nil || len(shortenedRootfsPath) > len(rootfs) {
		shortenedRootfsPath = rootfs
	}
	dnsSocketPath := path.Join(shortenedRootfsPath, "var", "run", "mDNSResponder")

	s.mu.Lock()
	defer s.mu.Unlock()

	c := &container{
		spec:          spec,
		bundlePath:    request.Bundle,
		rootfs:        rootfs,
		dnsSocketPath: dnsSocketPath,
		waitblock:     make(chan struct{}),
		status:        task.Status_CREATED,
	}

	defer func() {
		if retErr != nil {
			if err := c.destroy(); err != nil {
				log.G(ctx).WithError(err).Warn("failed to cleanup container")
			}
		}
	}()

	c.io, err = setupIO(ctx, request.Stdin, request.Stdout, request.Stderr)
	if err != nil {
		return nil, err
	}

	if len(spec.Process.Args) <= 0 {
		// TODO: How to handle this properly?
		spec.Process.Args = []string{"/bin/sh"}
		// return nil, fmt.Errorf("args must not be empty")
	}

	c.cmd = exec.Command(c.spec.Process.Args[0])
	c.cmd.Args = c.spec.Process.Args
	c.cmd.Dir = c.spec.Process.Cwd
	c.cmd.Env = c.spec.Process.Env
	c.cmd.SysProcAttr = &syscall.SysProcAttr{
		Chroot: c.rootfs,
		Credential: &syscall.Credential{
			Uid: c.spec.Process.User.UID,
			Gid: c.spec.Process.User.GID,
		},
	}

	var mounts []mount.Mount
	for _, m := range request.Rootfs {
		mounts = append(mounts, mount.Mount{
			Type:    m.Type,
			Source:  m.Source,
			Target:  m.Target,
			Options: m.Options,
		})
	}

	if err = mount.All(mounts, c.rootfs); err != nil {
		return nil, fmt.Errorf("failed to mount rootfs component: %w", err)
	}

	s.containers[request.ID] = c

	s.events <- &events.TaskCreate{
		ContainerID: request.ID,
		Bundle:      c.bundlePath,
		Rootfs:      request.Rootfs,
		IO: &events.TaskIO{
			Stdin:    request.Stdin,
			Stdout:   request.Stdout,
			Stderr:   request.Stderr,
			Terminal: c.spec.Process.Terminal,
		},
		Checkpoint: request.Checkpoint,
	}

	return &taskAPI.CreateTaskResponse{}, nil
}

func unixSocketCopy(from, to *net.UnixConn) error {
	for {
		// TODO: How we determine buffer size that is guaranteed to be enough?
		b := make([]byte, 1024)
		oob := make([]byte, 1024)
		n, oobn, _, addr, err := from.ReadMsgUnix(b, oob)
		if err != nil {
			return err
		}
		_, _, err = to.WriteMsgUnix(b[:n], oob[:oobn], addr)
		if err != nil {
			return err
		}
	}
}

func (s *service) Start(ctx context.Context, request *taskAPI.StartRequest) (*taskAPI.StartResponse, error) {
	log.G(ctx).WithField("request", request).Info("START")
	defer log.G(ctx).Info("START_DONE")

	if request.ExecID != "" {
		return nil, errdefs.ErrNotImplemented
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	c, err := s.getContainer(request.ID)
	if err != nil {
		return nil, err
	}

	dnsSocket, err := net.ListenUnix("unix", &net.UnixAddr{Name: c.dnsSocketPath, Net: "unix"})
	if err != nil {
		return nil, err
	}

	// TODO: We should stop this somehow?
	go func() {
		for {
			con, err := dnsSocket.AcceptUnix()
			if err != nil {
				return
			}

			pipe, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: "/var/run/mDNSResponder", Net: "unix"})
			if err != nil {
				return
			}
			go unixSocketCopy(con, pipe)
			go unixSocketCopy(pipe, con)
		}
	}()

	if c.spec.Process.Terminal {
		// TODO: I'd like to use containerd/console package instead
		// But see https://github.com/containerd/console/issues/79
		var consoleSize *pty.Winsize
		if c.spec.Process.ConsoleSize != nil {
			consoleSize = &pty.Winsize{
				Cols: uint16(c.spec.Process.ConsoleSize.Width),
				Rows: uint16(c.spec.Process.ConsoleSize.Height),
			}
		}

		c.console, err = pty.StartWithSize(c.cmd, consoleSize)
		if err != nil {
			return nil, err
		}

		go io.Copy(c.console, c.io.stdin)
		go io.Copy(c.io.stdout, c.console)
	} else {
		c.cmd.SysProcAttr.Setpgid = true
		c.cmd.Stdin = c.io.stdin
		c.cmd.Stdout = c.io.stdout
		c.cmd.Stderr = c.io.stderr

		err = c.cmd.Start()
		if err != nil {
			return nil, err
		}
	}

	c.setStatusL(task.Status_RUNNING)

	s.events <- &events.TaskStart{
		ContainerID: request.ID,
		Pid:         uint32(c.cmd.Process.Pid),
	}

	go func() {
		w, _ := wait(c.cmd.Process)

		c.exitStatus = uint32(w.ExitCode())
		c.setStatusL(task.Status_STOPPED)
		_ = c.io.Close()
		s.events <- &events.TaskExit{
			ContainerID: request.ID,
			ID:          request.ID,
			Pid:         uint32(w.Pid()),
			ExitStatus:  c.exitStatus,
		}

		close(c.waitblock)
	}()

	return &taskAPI.StartResponse{
		Pid: uint32(c.cmd.Process.Pid),
	}, nil
}

func (s *service) Delete(ctx context.Context, request *taskAPI.DeleteRequest) (*taskAPI.DeleteResponse, error) {
	log.G(ctx).WithField("request", request).Info("DELETE")
	defer log.G(ctx).Info("DELETE_DONE")

	if request.ExecID != "" {
		return nil, errdefs.ErrNotImplemented
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	c, err := s.getContainer(request.ID)
	if err != nil {
		return nil, err
	}

	if err := c.destroy(); err != nil {
		log.G(ctx).WithError(err).Warn("failed to cleanup container")
	}

	delete(s.containers, request.ID)

	s.events <- &events.TaskDelete{
		ContainerID: request.ID, // TODO
	}

	var pid int
	if p := c.cmd.Process; p != nil {
		pid = p.Pid
	}

	return &taskAPI.DeleteResponse{
		Pid: uint32(pid), // TODO
	}, nil
}

func (s *service) Pids(ctx context.Context, request *taskAPI.PidsRequest) (*taskAPI.PidsResponse, error) {
	log.G(ctx).WithField("request", request).Info("PIDS")
	return nil, errdefs.ErrNotImplemented
}

func (s *service) Pause(ctx context.Context, request *taskAPI.PauseRequest) (*ptypes.Empty, error) {
	log.G(ctx).WithField("request", request).Info("PAUSE")
	return nil, errdefs.ErrNotImplemented
}

func (s *service) Resume(ctx context.Context, request *taskAPI.ResumeRequest) (*ptypes.Empty, error) {
	log.G(ctx).WithField("request", request).Info("RESUME")
	return nil, errdefs.ErrNotImplemented
}

func (s *service) Checkpoint(ctx context.Context, request *taskAPI.CheckpointTaskRequest) (*ptypes.Empty, error) {
	log.G(ctx).WithField("request", request).Info("CHECKPOINT")
	return nil, errdefs.ErrNotImplemented
}

func (s *service) Kill(ctx context.Context, request *taskAPI.KillRequest) (*ptypes.Empty, error) {
	log.G(ctx).WithField("request", request).Info("KILL")
	defer log.G(ctx).Info("KILL_DONE")

	if request.ExecID != "" {
		return nil, errdefs.ErrNotImplemented
	}

	c, err := s.getContainerL(request.ID)
	if err != nil {
		return nil, err
	}

	if p := c.cmd.Process; p != nil {
		_ = unix.Kill(-p.Pid, syscall.Signal(request.Signal))
	}

	return &ptypes.Empty{}, nil
}

func (s *service) Exec(ctx context.Context, request *taskAPI.ExecProcessRequest) (*ptypes.Empty, error) {
	log.G(ctx).WithField("request", request).Info("EXEC")
	return nil, errdefs.ErrNotImplemented
}

func (s *service) ResizePty(ctx context.Context, request *taskAPI.ResizePtyRequest) (*ptypes.Empty, error) {
	log.G(ctx).WithField("request", request).Info("RESIZEPTY")
	defer log.G(ctx).Info("RESIZEPTY_DONE")

	if request.ExecID != "" {
		return nil, errdefs.ErrNotImplemented
	}

	c, err := s.getContainerL(request.ID)
	if err != nil {
		return nil, err
	}

	con := c.getConsoleL()
	if con == nil {
		return &ptypes.Empty{}, nil
	}

	if err = pty.Setsize(con, &pty.Winsize{Cols: uint16(request.Width), Rows: uint16(request.Height)}); err != nil {
		return nil, err
	}

	return &ptypes.Empty{}, nil
}

func (s *service) CloseIO(ctx context.Context, request *taskAPI.CloseIORequest) (*ptypes.Empty, error) {
	log.G(ctx).WithField("request", request).Info("CLOSEIO")

	if request.ExecID != "" {
		return nil, errdefs.ErrNotImplemented
	}

	c, err := s.getContainerL(request.ID)
	if err != nil {
		return nil, err
	}

	if stdin := c.io.stdin; stdin != nil {
		stdin.Close()
	}

	return &ptypes.Empty{}, nil
}

func (s *service) Update(ctx context.Context, request *taskAPI.UpdateTaskRequest) (*ptypes.Empty, error) {
	log.G(ctx).WithField("request", request).Info("UPDATE")
	return nil, errdefs.ErrNotImplemented
}

func (s *service) Wait(ctx context.Context, request *taskAPI.WaitRequest) (*taskAPI.WaitResponse, error) {
	log.G(ctx).WithField("request", request).Info("WAIT")
	defer log.G(ctx).Info("WAIT_DONE")

	if request.ExecID != "" {
		return nil, errdefs.ErrNotImplemented
	}

	c, err := s.getContainerL(request.ID)
	if err != nil {
		return nil, err
	}

	<-c.waitblock

	return &taskAPI.WaitResponse{
		ExitStatus: c.exitStatus,
	}, nil
}

func (s *service) Stats(ctx context.Context, request *taskAPI.StatsRequest) (*taskAPI.StatsResponse, error) {
	log.G(ctx).WithField("request", request).Info("STATS")
	return nil, errdefs.ErrNotImplemented
}

func (s *service) Connect(ctx context.Context, request *taskAPI.ConnectRequest) (*taskAPI.ConnectResponse, error) {
	log.G(ctx).WithField("request", request).Info("CONNECT")
	defer log.G(ctx).Info("CONNECT_DONE")

	var pid int
	if c, err := s.getContainerL(request.ID); err == nil {
		if p := c.cmd.Process; p != nil {
			pid = p.Pid
		}
	}

	return &taskAPI.ConnectResponse{
		ShimPid: uint32(os.Getpid()),
		TaskPid: uint32(pid),
	}, nil
}

func (s *service) Shutdown(ctx context.Context, request *taskAPI.ShutdownRequest) (*ptypes.Empty, error) {
	log.G(ctx).WithField("request", request).Info("SHUTDOWN")
	defer log.G(ctx).Info("SHUTDOWN_DONE")

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.containers) > 0 {
		return &ptypes.Empty{}, nil
	}

	s.sd.Shutdown()

	return &ptypes.Empty{}, nil
}
