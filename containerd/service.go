package containerd

import (
	"context"
	"fmt"
	"github.com/containerd/containerd/api/events"
	taskAPI "github.com/containerd/containerd/api/runtime/task/v3"
	"github.com/containerd/containerd/api/types"
	"github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/v2/core/mount"
	"github.com/containerd/containerd/v2/core/runtime"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/containerd/v2/pkg/protobuf"
	ptypes "github.com/containerd/containerd/v2/pkg/protobuf/types"
	"github.com/containerd/containerd/v2/pkg/shim"
	"github.com/containerd/containerd/v2/pkg/shutdown"
	"github.com/containerd/errdefs"
	"github.com/containerd/errdefs/pkg/errgrpc"
	"github.com/containerd/log"
	"github.com/containerd/ttrpc"
	"github.com/containerd/typeurl/v2"
	"github.com/creack/pty"
	"github.com/opencontainers/runtime-spec/specs-go"
	"net"
	"os"
	"path"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

func NewTaskService(ctx context.Context, publisher shim.Publisher, sd shutdown.Service) (taskAPI.TTRPCTaskService, error) {
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
		return nil, errgrpc.ToGRPCf(errdefs.ErrNotFound, "container not created")
	}
	return c, nil
}

func (s *service) getContainerL(id string) (*container, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.getContainer(id)
}

func (s *service) RegisterTTRPC(server *ttrpc.Server) error {
	taskAPI.RegisterTTRPCTaskService(server, s)
	return nil
}

func (s *service) State(ctx context.Context, request *taskAPI.StateRequest) (*taskAPI.StateResponse, error) {
	log.G(ctx).WithField("request", request).Info("STATE")
	defer log.G(ctx).Info("STATE_DONE")

	c, err := s.getContainerL(request.ID)
	if err != nil {
		return nil, err
	}

	p, err := c.getProcessL(request.ExecID)
	if err != nil {
		return nil, err
	}

	var pid int
	if process := p.cmd.Process; process != nil {
		pid = process.Pid
	}

	return &taskAPI.StateResponse{
		ID:         request.ID,
		Bundle:     c.bundlePath,
		Pid:        uint32(pid),
		Status:     p.status,
		Stdin:      p.io.stdinPath,
		Stdout:     p.io.stdoutPath,
		Stderr:     p.io.stderrPath,
		Terminal:   c.spec.Process.Terminal,
		ExitedAt:   protobuf.ToTimestamp(p.exitedAt),
		ExitStatus: p.exitStatus,
		ExecID:     request.ExecID,
	}, nil
}

func (s *service) Create(ctx context.Context, request *taskAPI.CreateTaskRequest) (_ *taskAPI.CreateTaskResponse, retErr error) {
	log.G(ctx).WithField("request", request).Info("CREATE")
	defer log.G(ctx).Info("CREATE_DONE")

	spec, err := oci.ReadSpec(path.Join(request.Bundle, oci.ConfigFilename))
	if err != nil {
		return nil, err
	}

	rootfs, err := mount.CanonicalizePath(spec.Root.Path)
	if err != nil {
		return nil, err
	}

	// Workaround for 104-char limit of UNIX socket path
	shortenedRootfsPath, err := shortenPath(rootfs)
	if err != nil {
		return nil, err
	}

	dnsSocketPath := path.Join(shortenedRootfsPath, "var", "run", "mDNSResponder")

	s.mu.Lock()
	defer s.mu.Unlock()

	c := &container{
		spec:          spec,
		bundlePath:    request.Bundle,
		rootfs:        rootfs,
		dnsSocketPath: dnsSocketPath,
		primary: managedProcess{
			spec:      spec.Process,
			waitblock: make(chan struct{}),
			status:    task.Status_CREATED,
		},
		auxiliary: make(map[string]*managedProcess),
	}

	defer func() {
		if retErr != nil {
			if err := c.destroy(); err != nil {
				log.G(ctx).WithError(err).Warn("failed to cleanup container")
			}
		}
	}()

	if err = c.primary.setup(ctx, c.rootfs, request.Stdin, request.Stdout, request.Stderr); err != nil {
		return nil, err
	}

	mounts, err := processMounts(c.rootfs, request.Rootfs, spec.Mounts)
	if err != nil {
		return nil, err
	}

	if err = mount.All(mounts, c.rootfs); err != nil {
		return nil, fmt.Errorf("failed to mount rootfs component: %w", err)
	}

	// TODO: Check if container already exists?
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

func shortenPath(p string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	shortened, err := filepath.Rel(wd, path.Join(p))
	if err != nil || len(shortened) > len(p) {
		return p, nil
	}

	return shortened, nil
}

func processMounts(targetRoot string, rootfs []*types.Mount, specMounts []specs.Mount) ([]mount.Mount, error) {
	var mounts []mount.Mount
	for _, m := range rootfs {
		mm, err := processMount(targetRoot, m.Type, m.Source, m.Target, m.Options)
		if err != nil {
			return nil, err
		}

		if mm != nil {
			mounts = append(mounts, *mm)
		}
	}

	for _, m := range specMounts {
		mm, err := processMount(targetRoot, m.Type, m.Source, m.Destination, m.Options)
		if err != nil {
			return nil, err
		}

		if mm != nil {
			mounts = append(mounts, *mm)
		}
	}

	return mounts, nil
}

func processMount(rootfs, mtype, source, target string, options []string) (*mount.Mount, error) {
	m := &mount.Mount{
		Type:    mtype,
		Source:  source,
		Target:  target,
		Options: options,
	}

	switch mtype {
	case "bind":
		stat, err := os.Stat(source)
		if err != nil {
			return nil, err
		}

		if stat.IsDir() {
			fullPath := filepath.Join(rootfs, target)
			if err = os.MkdirAll(fullPath, 0o755); err != nil {
				return nil, err
			}

			return m, nil
		} else {
			// skip, only dirs are supported by bindfs
		}
	case "devfs":
		return m, nil
	}

	log.L.Warn("skipping mount: ", m)
	return nil, nil
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

	s.mu.Lock()
	defer s.mu.Unlock()

	c, err := s.getContainer(request.ID)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if request.ExecID == "" {
		if err = os.MkdirAll(path.Dir(c.dnsSocketPath), 0o755); err != nil {
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
	}

	p, err := c.getProcess(request.ExecID)
	if err != nil {
		return nil, err
	}

	if err = p.start(); err != nil {
		return nil, err
	}

	go func() {
		var w *os.ProcessState

		if request.ExecID == "" {
			w, _ = wait(p.cmd.Process)
		} else {
			w, _ = p.cmd.Process.Wait()
		}

		p.exitedAt = time.Now()
		p.exitStatus = uint32(w.ExitCode())
		p.status = task.Status_STOPPED

		_ = p.io.Close()

		// Madness...
		id := request.ID
		if request.ExecID != "" {
			id = request.ExecID
		}

		s.events <- &events.TaskExit{
			ContainerID: request.ID,
			ID:          id,
			Pid:         uint32(w.Pid()),
			ExitedAt:    protobuf.ToTimestamp(p.exitedAt),
			ExitStatus:  p.exitStatus,
		}

		close(p.waitblock)
	}()

	if request.ExecID == "" {
		s.events <- &events.TaskStart{
			ContainerID: request.ID,
			Pid:         uint32(p.cmd.Process.Pid),
		}
	} else {
		s.events <- &events.TaskExecStarted{
			ContainerID: request.ID,
			ExecID:      request.ExecID,
			Pid:         uint32(p.cmd.Process.Pid),
		}
	}

	return &taskAPI.StartResponse{
		Pid: uint32(p.cmd.Process.Pid),
	}, nil
}

func (s *service) Delete(ctx context.Context, request *taskAPI.DeleteRequest) (*taskAPI.DeleteResponse, error) {
	log.G(ctx).WithField("request", request).Info("DELETE")
	defer log.G(ctx).Info("DELETE_DONE")

	s.mu.Lock()
	defer s.mu.Unlock()

	c, err := s.getContainer(request.ID)
	if err != nil {
		return nil, err
	}

	if request.ExecID != "" {
		c.mu.Lock()
		defer c.mu.Unlock()

		p, err := c.getProcess(request.ExecID)
		if err != nil {
			return nil, err
		}

		if err := p.destroy(); err != nil {
			log.G(ctx).WithError(err).Warn("failed to destroy exec")
		}
		delete(c.auxiliary, request.ExecID)

		return &taskAPI.DeleteResponse{
			ExitedAt:   protobuf.ToTimestamp(p.exitedAt),
			ExitStatus: p.exitStatus,
		}, nil
	}

	if err := c.destroy(); err != nil {
		log.G(ctx).WithError(err).Warn("failed to cleanup container")
	}

	delete(s.containers, request.ID)

	var pid uint32
	if p := c.primary.cmd.Process; p != nil {
		pid = uint32(p.Pid)
	}

	s.events <- &events.TaskDelete{
		ContainerID: request.ID,
		ExitedAt:    protobuf.ToTimestamp(c.primary.exitedAt),
		ExitStatus:  c.primary.exitStatus,
		ID:          request.ID,
		Pid:         pid,
	}

	return &taskAPI.DeleteResponse{
		ExitedAt:   protobuf.ToTimestamp(c.primary.exitedAt),
		ExitStatus: c.primary.exitStatus,
		Pid:        pid,
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

	c, err := s.getContainerL(request.ID)
	if err != nil {
		return nil, err
	}

	p, err := c.getProcessL(request.ExecID)
	if err != nil {
		return nil, err
	}

	// TODO: Do we care about error here?
	_ = p.kill(syscall.Signal(request.Signal))

	return &ptypes.Empty{}, nil
}

func (s *service) Exec(ctx context.Context, request *taskAPI.ExecProcessRequest) (_ *ptypes.Empty, retErr error) {
	log.G(ctx).WithField("request", request).Info("EXEC")

	specAny, err := typeurl.UnmarshalAny(request.Spec)
	if err != nil {
		log.G(ctx).WithError(err).Error("failed to unmarshal spec")
		return nil, errdefs.ErrInvalidArgument
	}

	spec, ok := specAny.(*specs.Process)
	if !ok {
		log.G(ctx).Error("mismatched type for spec")
		return nil, errdefs.ErrInvalidArgument
	}

	c, err := s.getContainerL(request.ID)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	aux := &managedProcess{
		spec:      spec,
		waitblock: make(chan struct{}),
		status:    task.Status_CREATED,
	}

	defer func() {
		if retErr != nil {
			if err := aux.destroy(); err != nil {
				log.G(ctx).WithError(err).Warn("failed to cleanup aux")
			}
		}
	}()

	if err = aux.setup(ctx, c.rootfs, request.Stdin, request.Stdout, request.Stderr); err != nil {
		return nil, err
	}

	// TODO: Check if aux already exists?
	c.auxiliary[request.ExecID] = aux

	s.events <- &events.TaskExecAdded{
		ContainerID: request.ID,
		ExecID:      request.ExecID,
	}

	return &ptypes.Empty{}, nil
}

func (s *service) ResizePty(ctx context.Context, request *taskAPI.ResizePtyRequest) (*ptypes.Empty, error) {
	log.G(ctx).WithField("request", request).Info("RESIZEPTY")
	defer log.G(ctx).Info("RESIZEPTY_DONE")

	c, err := s.getContainerL(request.ID)
	if err != nil {
		return nil, err
	}

	p, err := c.getProcessL(request.ExecID)
	if err != nil {
		return nil, err
	}

	if con := p.getConsoleL(); con != nil {
		if err = pty.Setsize(con, &pty.Winsize{Cols: uint16(request.Width), Rows: uint16(request.Height)}); err != nil {
			return nil, err
		}
	}

	return &ptypes.Empty{}, nil
}

func (s *service) CloseIO(ctx context.Context, request *taskAPI.CloseIORequest) (*ptypes.Empty, error) {
	log.G(ctx).WithField("request", request).Info("CLOSEIO")

	c, err := s.getContainerL(request.ID)
	if err != nil {
		return nil, err
	}

	p, err := c.getProcessL(request.ExecID)
	if err != nil {
		return nil, err
	}

	if stdin := p.io.stdin; stdin != nil {
		_ = stdin.Close()
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

	c, err := s.getContainerL(request.ID)
	if err != nil {
		return nil, err
	}

	p, err := c.getProcessL(request.ExecID)
	if err != nil {
		return nil, err
	}

	<-p.waitblock

	return &taskAPI.WaitResponse{
		ExitedAt:   protobuf.ToTimestamp(p.exitedAt),
		ExitStatus: p.exitStatus,
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
		if p := c.primary.cmd.Process; p != nil {
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
