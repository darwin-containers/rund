package containerd

import (
	"context"
	"encoding/json"
	"github.com/containerd/containerd/api/events"
	taskAPI "github.com/containerd/containerd/api/runtime/task/v2"
	"github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/pkg/shutdown"
	"github.com/containerd/containerd/runtime"
	"github.com/containerd/containerd/runtime/v2/shim"
	"github.com/containerd/ttrpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"os"
	"os/exec"
	"path"
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

func (s *service) RegisterTTRPC(server *ttrpc.Server) error {
	taskAPI.RegisterTaskService(server, s)
	return nil
}

func (s *service) State(ctx context.Context, request *taskAPI.StateRequest) (*taskAPI.StateResponse, error) {
	log.G(ctx).WithField("request", request).Warn("STATE")

	if request.ExecID != "" {
		return nil, errdefs.ErrNotImplemented
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	c, err := s.getContainer(request.ID)
	if err != nil {
		return nil, err
	}

	// TODO

	if c.process != nil {
		return &taskAPI.StateResponse{
			Status: task.Status_CREATED,
			// TODO
		}, nil
	} else {
		return &taskAPI.StateResponse{
			Status: task.Status_STOPPED,
			// TODO
		}, nil
	}
}

func (s *service) Create(ctx context.Context, request *taskAPI.CreateTaskRequest) (*taskAPI.CreateTaskResponse, error) {
	log.G(ctx).WithField("request", request).Warn("CREATE")

	s.mu.Lock()
	defer s.mu.Unlock()

	c := &container{
		request: request,
	}

	if _, err := os.Stat(request.Stdin); err == nil {
		c.stdin, err = os.OpenFile(request.Stdin, syscall.O_RDONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			return nil, err
		}
	}

	if _, err := os.Stat(request.Stdout); err == nil {
		c.stdout, err = os.OpenFile(request.Stdout, syscall.O_WRONLY, 0)
		if err != nil {
			return nil, err
		}
	}

	if _, err := os.Stat(request.Stderr); err == nil {
		c.stderr, err = os.OpenFile(request.Stderr, syscall.O_WRONLY, 0)
		if err != nil {
			return nil, err
		}
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
		Pid:        uint32(c.getPid()),
	}

	return &taskAPI.CreateTaskResponse{
		Pid: uint32(c.getPid()),
	}, nil
}

func readSpec(path string) (*oci.Spec, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var spec oci.Spec
	if err := json.NewDecoder(f).Decode(&spec); err != nil {
		return nil, err
	}

	return &spec, nil
}

func (s *service) Start(ctx context.Context, request *taskAPI.StartRequest) (*taskAPI.StartResponse, error) {
	log.G(ctx).WithField("request", request).Warn("START")

	s.mu.Lock()
	defer s.mu.Unlock()

	if request.ExecID != "" {
		return nil, errdefs.ErrNotImplemented
	}

	c, err := s.getContainer(request.ID)
	if err != nil {
		return nil, err
	}

	spec, err := readSpec(path.Join(c.request.Bundle, "config.json"))
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("/usr/sbin/chroot", c.request.Rootfs[0].Source)
	cmd.Args = append(cmd.Args, spec.Process.Args...)
	cmd.Stdin = c.stdin
	cmd.Stdout = c.stdout
	cmd.Stderr = c.stderr

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	c.process = cmd.Process
	if err != nil {
		return nil, err
	}

	s.events <- &events.TaskStart{
		ContainerID: request.ID,
		Pid:         uint32(c.getPid()),
	}

	return &taskAPI.StartResponse{
		Pid: uint32(c.getPid()),
	}, nil
}

func (s *service) Delete(ctx context.Context, request *taskAPI.DeleteRequest) (*taskAPI.DeleteResponse, error) {
	log.G(ctx).WithField("request", request).Warn("DELETE")

	if request.ExecID != "" {
		return nil, errdefs.ErrNotImplemented
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	c, err := s.getContainer(request.ID)
	if err != nil {
		return nil, err
	}

	delete(s.containers, request.ID)
	s.events <- &events.TaskDelete{
		ContainerID: request.ID,
		// TODO
	}
	return &taskAPI.DeleteResponse{
		Pid: uint32(c.getPid()),
		// TODO
	}, nil
}

func (s *service) Pids(ctx context.Context, request *taskAPI.PidsRequest) (*taskAPI.PidsResponse, error) {
	log.G(ctx).WithField("request", request).Warn("PIDS")
	return nil, errdefs.ErrNotImplemented
}

func (s *service) Pause(ctx context.Context, request *taskAPI.PauseRequest) (*emptypb.Empty, error) {
	log.G(ctx).WithField("request", request).Warn("PAUSE")
	return nil, errdefs.ErrNotImplemented
}

func (s *service) Resume(ctx context.Context, request *taskAPI.ResumeRequest) (*emptypb.Empty, error) {
	log.G(ctx).WithField("request", request).Warn("RESUME")
	return nil, errdefs.ErrNotImplemented
}

func (s *service) Checkpoint(ctx context.Context, request *taskAPI.CheckpointTaskRequest) (*emptypb.Empty, error) {
	log.G(ctx).WithField("request", request).Warn("CHECKPOINT")
	return nil, errdefs.ErrNotImplemented
}

func (s *service) Kill(ctx context.Context, request *taskAPI.KillRequest) (*emptypb.Empty, error) {
	log.G(ctx).WithField("request", request).Warn("KILL")

	if request.ExecID != "" {
		return nil, errdefs.ErrNotImplemented
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	c, err := s.getContainer(request.ID)
	if err != nil {
		return nil, err
	}

	if c.process != nil {
		_ = c.process.Kill()
	}

	return &emptypb.Empty{}, nil
}

func (s *service) Exec(ctx context.Context, request *taskAPI.ExecProcessRequest) (*emptypb.Empty, error) {
	log.G(ctx).WithField("request", request).Warn("EXEC")
	return nil, errdefs.ErrNotImplemented
}

func (s *service) ResizePty(ctx context.Context, request *taskAPI.ResizePtyRequest) (*emptypb.Empty, error) {
	log.G(ctx).WithField("request", request).Warn("RESIZEPTY")
	return nil, errdefs.ErrNotImplemented
}

func (s *service) CloseIO(ctx context.Context, request *taskAPI.CloseIORequest) (*emptypb.Empty, error) {
	log.G(ctx).WithField("request", request).Warn("CLOSEIO")
	return nil, errdefs.ErrNotImplemented
}

func (s *service) Update(ctx context.Context, request *taskAPI.UpdateTaskRequest) (*emptypb.Empty, error) {
	log.G(ctx).WithField("request", request).Warn("UPDATE")
	return nil, errdefs.ErrNotImplemented
}

func (s *service) Wait(ctx context.Context, request *taskAPI.WaitRequest) (*taskAPI.WaitResponse, error) {
	log.G(ctx).WithField("request", request).Warn("WAIT")

	if request.ExecID != "" {
		return nil, errdefs.ErrNotImplemented
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	c, err := s.getContainer(request.ID)
	if err != nil {
		return nil, err
	}

	if c.process == nil {
		return nil, errdefs.ErrFailedPrecondition
	}

	wait, _ := c.process.Wait()

	if c.stdin != nil {
		_ = c.stdin.Close()
	}

	if c.stdout != nil {
		_ = c.stdout.Close()
	}

	if c.stderr != nil {
		_ = c.stderr.Close()
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
	log.G(ctx).WithField("request", request).Warn("STATS")
	return nil, errdefs.ErrNotImplemented
}

func (s *service) Connect(ctx context.Context, request *taskAPI.ConnectRequest) (*taskAPI.ConnectResponse, error) {
	log.G(ctx).WithField("request", request).Warn("CONNECT")

	var pid int
	if c, err := s.getContainer(request.ID); err == nil {
		pid = c.getPid()
	}

	return &taskAPI.ConnectResponse{
		ShimPid: uint32(os.Getpid()),
		TaskPid: uint32(pid),
	}, nil
}

func (s *service) Shutdown(ctx context.Context, request *taskAPI.ShutdownRequest) (*emptypb.Empty, error) {
	log.G(ctx).WithField("request", request).Warn("SHUTDOWN")

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.containers) > 0 {
		return &emptypb.Empty{}, nil
	}

	s.sd.Shutdown()

	return &emptypb.Empty{}, nil
}
