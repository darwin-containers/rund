package containerd

import (
	"context"
	taskAPI "github.com/containerd/containerd/api/runtime/task/v2"
	"github.com/containerd/containerd/pkg/shutdown"
	"github.com/containerd/containerd/runtime/v2/shim"
	"github.com/containerd/ttrpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

func NewTaskService(ctx context.Context, publisher shim.Publisher, sd shutdown.Service) (taskAPI.TaskService, error) {
	return service{}, nil
}

type service struct {
}

func (s service) RegisterTTRPC(server *ttrpc.Server) error {
	taskAPI.RegisterTaskService(server, s)
	return nil
}

func (s service) State(ctx context.Context, request *taskAPI.StateRequest) (*taskAPI.StateResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s service) Create(ctx context.Context, request *taskAPI.CreateTaskRequest) (*taskAPI.CreateTaskResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s service) Start(ctx context.Context, request *taskAPI.StartRequest) (*taskAPI.StartResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s service) Delete(ctx context.Context, request *taskAPI.DeleteRequest) (*taskAPI.DeleteResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s service) Pids(ctx context.Context, request *taskAPI.PidsRequest) (*taskAPI.PidsResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s service) Pause(ctx context.Context, request *taskAPI.PauseRequest) (*emptypb.Empty, error) {
	//TODO implement me
	panic("implement me")
}

func (s service) Resume(ctx context.Context, request *taskAPI.ResumeRequest) (*emptypb.Empty, error) {
	//TODO implement me
	panic("implement me")
}

func (s service) Checkpoint(ctx context.Context, request *taskAPI.CheckpointTaskRequest) (*emptypb.Empty, error) {
	//TODO implement me
	panic("implement me")
}

func (s service) Kill(ctx context.Context, request *taskAPI.KillRequest) (*emptypb.Empty, error) {
	//TODO implement me
	panic("implement me")
}

func (s service) Exec(ctx context.Context, request *taskAPI.ExecProcessRequest) (*emptypb.Empty, error) {
	//TODO implement me
	panic("implement me")
}

func (s service) ResizePty(ctx context.Context, request *taskAPI.ResizePtyRequest) (*emptypb.Empty, error) {
	//TODO implement me
	panic("implement me")
}

func (s service) CloseIO(ctx context.Context, request *taskAPI.CloseIORequest) (*emptypb.Empty, error) {
	//TODO implement me
	panic("implement me")
}

func (s service) Update(ctx context.Context, request *taskAPI.UpdateTaskRequest) (*emptypb.Empty, error) {
	//TODO implement me
	panic("implement me")
}

func (s service) Wait(ctx context.Context, request *taskAPI.WaitRequest) (*taskAPI.WaitResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s service) Stats(ctx context.Context, request *taskAPI.StatsRequest) (*taskAPI.StatsResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s service) Connect(ctx context.Context, request *taskAPI.ConnectRequest) (*taskAPI.ConnectResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s service) Shutdown(ctx context.Context, request *taskAPI.ShutdownRequest) (*emptypb.Empty, error) {
	//TODO implement me
	panic("implement me")
}
