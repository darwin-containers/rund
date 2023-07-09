package containerd

import (
	"context"
	"encoding/json"
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
	"golang.org/x/sys/unix"
	"os"
	"os/exec"
	"path"
	"sync"
	"syscall"
)

// UnmountFlags Flags to use when unmounting filesystems. Workaround against mds.
const UnmountFlags = unix.MNT_FORCE

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

	return &taskAPI.StateResponse{
		Status: c.status,
		// TODO
	}, nil
}

func (s *service) Create(ctx context.Context, request *taskAPI.CreateTaskRequest) (_ *taskAPI.CreateTaskResponse, retErr error) {
	log.G(ctx).WithField("request", request).Info("CREATE")
	defer log.G(ctx).Info("CREATE_DONE")

	s.mu.Lock()
	defer s.mu.Unlock()

	spec, err := readSpec(path.Join(request.Bundle, "config.json"))
	if err != nil {
		return nil, err
	}

	pio, err := setupIO(ctx, request.Stdin, request.Stdout, request.Stderr)
	defer func() {
		if err != nil {
			_ = pio.Close()
		}
	}()

	c := &container{
		spec:   spec,
		status: task.Status_CREATED,
		rootfs: path.Join(request.Bundle, "rootfs"),
		io:     pio,
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

	defer func() {
		if retErr != nil {
			if err := mount.UnmountMounts(mounts, c.rootfs, 0); err != nil {
				log.G(ctx).WithError(err).Warn("failed to cleanup rootfs mount")
			}
		}
	}()

	if err := mount.All(mounts, c.rootfs); err != nil {
		return nil, fmt.Errorf("failed to mount rootfs component: %w", err)
	}

	c.cmd = exec.Command("chroot", c.rootfs)
	c.cmd.Args = append(c.cmd.Args, c.spec.Process.Args...)
	c.cmd.Env = c.spec.Process.Env
	c.cmd.Stdin = pio.stdin
	c.cmd.Stdout = pio.stdout
	c.cmd.Stderr = pio.stderr
	c.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	s.containers[request.ID] = c

	s.events <- &events.TaskCreate{
		ContainerID: request.ID,
		Bundle:      request.Bundle,
		Rootfs:      request.Rootfs,
		IO: &events.TaskIO{
			Stdin:  request.Stdin,
			Stdout: request.Stdout,
			Stderr: request.Stderr,
		},
		Checkpoint: request.Checkpoint,
	}

	return &taskAPI.CreateTaskResponse{}, nil
}

func readSpec(path string) (*oci.Spec, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	var spec oci.Spec
	if err := json.NewDecoder(f).Decode(&spec); err != nil {
		return nil, err
	}

	return &spec, nil
}

func (s *service) Start(ctx context.Context, request *taskAPI.StartRequest) (*taskAPI.StartResponse, error) {
	log.G(ctx).WithField("request", request).Info("START")
	defer log.G(ctx).Info("START_DONE")

	s.mu.Lock()
	defer s.mu.Unlock()

	if request.ExecID != "" {
		return nil, errdefs.ErrNotImplemented
	}

	c, err := s.getContainer(request.ID)
	if err != nil {
		return nil, err
	}

	varRun := path.Join(c.rootfs, "var", "run")
	if err := os.MkdirAll(varRun, 775); err != nil {
		return nil, err
	}

	// TODO: Can't do it this way, cross-device link
	// if err := os.Link("/var/run/mDNSResponder", path.Join(varRun, "mDNSResponder")); err != nil {
	//	return nil, err
	// }

	if err := c.cmd.Start(); err != nil {
		return nil, err
	}

	c.setStatusL(task.Status_RUNNING)

	s.events <- &events.TaskStart{
		ContainerID: request.ID,
		Pid:         uint32(c.cmd.Process.Pid),
	}

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

	_ = c.io.Close()

	if err := mount.UnmountRecursive(c.rootfs, UnmountFlags); err != nil {
		log.G(ctx).WithError(err).Warn("failed to cleanup rootfs mount")
	}

	delete(s.containers, request.ID)

	s.events <- &events.TaskDelete{
		ContainerID: request.ID,
		// TODO
	}

	var pid int
	if p := c.cmd.Process; p != nil {
		pid = p.Pid
	}

	return &taskAPI.DeleteResponse{
		Pid: uint32(pid),
		// TODO
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
		_ = unix.Kill(p.Pid, syscall.Signal(request.Signal))
	}

	return &ptypes.Empty{}, nil
}

func (s *service) Exec(ctx context.Context, request *taskAPI.ExecProcessRequest) (*ptypes.Empty, error) {
	log.G(ctx).WithField("request", request).Info("EXEC")
	return nil, errdefs.ErrNotImplemented
}

func (s *service) ResizePty(ctx context.Context, request *taskAPI.ResizePtyRequest) (*ptypes.Empty, error) {
	log.G(ctx).WithField("request", request).Info("RESIZEPTY")
	return nil, errdefs.ErrNotImplemented
}

func (s *service) CloseIO(ctx context.Context, request *taskAPI.CloseIORequest) (*ptypes.Empty, error) {
	log.G(ctx).WithField("request", request).Info("CLOSEIO")
	return nil, errdefs.ErrNotImplemented
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

	c.setStatusL(task.Status_STOPPED)

	defer func(io stdio) {
		_ = io.Close()
	}(c.io)

	if c.cmd.Process == nil {
		return nil, errdefs.ErrFailedPrecondition
	}

	// TODO: handle error?
	wait, _ := c.cmd.Process.Wait()

	if c.io.stdin != nil {
		_ = c.io.stdin.Close()
	}

	s.events <- &events.TaskExit{
		ContainerID: request.ID,
		Pid:         uint32(wait.Pid()),
		ExitStatus:  uint32(wait.ExitCode()),
		// TODO
	}

	return &taskAPI.WaitResponse{
		ExitStatus: uint32(wait.ExitCode()),
		// TODO
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
