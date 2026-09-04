package rpc

import (
	"context"
	"github.com/example/jf-dispatch/internal/api"
	"google.golang.org/grpc"
)

type SchedulerServer interface {
	Submit(context.Context, *api.SubmitRequest) (*api.Job, error)
	Get(context.Context, *api.JobRef) (*api.Job, error)
	Watch(*api.JobRef, grpc.ServerStreamingServer[api.JobEvent]) error
	Cancel(context.Context, *api.JobRef) (*api.CancelReply, error)
	Register(context.Context, *api.WorkerStatus) (*api.RegisterReply, error)
	Heartbeat(context.Context, *api.WorkerStatus) (*api.HeartbeatReply, error)
	ListWorkers(context.Context, *api.Empty) (*api.WorkerList, error)
}
type WorkerServer interface {
	Run(*api.RunRequest, grpc.ServerStreamingServer[api.JobEvent]) error
	Cancel(context.Context, *api.JobRef) (*api.CancelReply, error)
}

func RegisterSchedulerServer(s *grpc.Server, impl SchedulerServer) {
	s.RegisterService(&grpc.ServiceDesc{ServiceName: "jfdispatch.v1.Scheduler", HandlerType: (*SchedulerServer)(nil), Methods: []grpc.MethodDesc{{MethodName: "Submit", Handler: unarySubmit}, {MethodName: "Get", Handler: unaryGet}, {MethodName: "Cancel", Handler: unaryCancel}, {MethodName: "Register", Handler: unaryRegister}, {MethodName: "Heartbeat", Handler: unaryHeartbeat}, {MethodName: "ListWorkers", Handler: unaryListWorkers}}, Streams: []grpc.StreamDesc{{StreamName: "Watch", Handler: streamWatch, ServerStreams: true}}}, impl)
}
func RegisterWorkerServer(s *grpc.Server, impl WorkerServer) {
	s.RegisterService(&grpc.ServiceDesc{ServiceName: "jfdispatch.v1.Worker", HandlerType: (*WorkerServer)(nil), Methods: []grpc.MethodDesc{{MethodName: "Cancel", Handler: workerCancel}}, Streams: []grpc.StreamDesc{{StreamName: "Run", Handler: streamRun, ServerStreams: true}}}, impl)
}

func unarySubmit(s any, c context.Context, d func(any) error, i grpc.UnaryServerInterceptor) (any, error) {
	in := new(api.SubmitRequest)
	if e := d(in); e != nil {
		return nil, e
	}
	if i == nil {
		return s.(SchedulerServer).Submit(c, in)
	}
	return i(c, in, &grpc.UnaryServerInfo{Server: s, FullMethod: "/jfdispatch.v1.Scheduler/Submit"}, func(c context.Context, r any) (any, error) {
		return s.(SchedulerServer).Submit(c, r.(*api.SubmitRequest))
	})
}
func unaryGet(s any, c context.Context, d func(any) error, i grpc.UnaryServerInterceptor) (any, error) {
	in := new(api.JobRef)
	if e := d(in); e != nil {
		return nil, e
	}
	if i == nil {
		return s.(SchedulerServer).Get(c, in)
	}
	return i(c, in, &grpc.UnaryServerInfo{Server: s, FullMethod: "/jfdispatch.v1.Scheduler/Get"}, func(c context.Context, r any) (any, error) { return s.(SchedulerServer).Get(c, r.(*api.JobRef)) })
}
func unaryCancel(s any, c context.Context, d func(any) error, i grpc.UnaryServerInterceptor) (any, error) {
	in := new(api.JobRef)
	if e := d(in); e != nil {
		return nil, e
	}
	if i == nil {
		return s.(SchedulerServer).Cancel(c, in)
	}
	return i(c, in, &grpc.UnaryServerInfo{Server: s, FullMethod: "/jfdispatch.v1.Scheduler/Cancel"}, func(c context.Context, r any) (any, error) { return s.(SchedulerServer).Cancel(c, r.(*api.JobRef)) })
}
func unaryRegister(s any, c context.Context, d func(any) error, i grpc.UnaryServerInterceptor) (any, error) {
	in := new(api.WorkerStatus)
	if e := d(in); e != nil {
		return nil, e
	}
	if i == nil {
		return s.(SchedulerServer).Register(c, in)
	}
	return i(c, in, &grpc.UnaryServerInfo{Server: s, FullMethod: "/jfdispatch.v1.Scheduler/Register"}, func(c context.Context, r any) (any, error) {
		return s.(SchedulerServer).Register(c, r.(*api.WorkerStatus))
	})
}
func unaryHeartbeat(s any, c context.Context, d func(any) error, i grpc.UnaryServerInterceptor) (any, error) {
	in := new(api.WorkerStatus)
	if e := d(in); e != nil {
		return nil, e
	}
	if i == nil {
		return s.(SchedulerServer).Heartbeat(c, in)
	}
	return i(c, in, &grpc.UnaryServerInfo{Server: s, FullMethod: "/jfdispatch.v1.Scheduler/Heartbeat"}, func(c context.Context, r any) (any, error) {
		return s.(SchedulerServer).Heartbeat(c, r.(*api.WorkerStatus))
	})
}
func unaryListWorkers(s any, c context.Context, d func(any) error, i grpc.UnaryServerInterceptor) (any, error) {
	in := new(api.Empty)
	if e := d(in); e != nil {
		return nil, e
	}
	if i == nil {
		return s.(SchedulerServer).ListWorkers(c, in)
	}
	return i(c, in, &grpc.UnaryServerInfo{Server: s, FullMethod: "/jfdispatch.v1.Scheduler/ListWorkers"}, func(c context.Context, r any) (any, error) { return s.(SchedulerServer).ListWorkers(c, r.(*api.Empty)) })
}
func workerCancel(s any, c context.Context, d func(any) error, i grpc.UnaryServerInterceptor) (any, error) {
	in := new(api.JobRef)
	if e := d(in); e != nil {
		return nil, e
	}
	if i == nil {
		return s.(WorkerServer).Cancel(c, in)
	}
	return i(c, in, &grpc.UnaryServerInfo{Server: s, FullMethod: "/jfdispatch.v1.Worker/Cancel"}, func(c context.Context, r any) (any, error) { return s.(WorkerServer).Cancel(c, r.(*api.JobRef)) })
}
func streamWatch(s any, ss grpc.ServerStream) error {
	in := new(api.JobRef)
	if e := ss.RecvMsg(in); e != nil {
		return e
	}
	return s.(SchedulerServer).Watch(in, &serverStream[api.JobEvent]{ss})
}
func streamRun(s any, ss grpc.ServerStream) error {
	in := new(api.RunRequest)
	if e := ss.RecvMsg(in); e != nil {
		return e
	}
	return s.(WorkerServer).Run(in, &serverStream[api.JobEvent]{ss})
}

type serverStream[T any] struct{ grpc.ServerStream }

func (s *serverStream[T]) Send(v *T) error { return s.SendMsg(v) }

type SchedulerClient struct{ cc grpc.ClientConnInterface }

func NewSchedulerClient(c grpc.ClientConnInterface) *SchedulerClient { return &SchedulerClient{c} }
func (c *SchedulerClient) Submit(ctx context.Context, in *api.SubmitRequest) (*api.Job, error) {
	out := new(api.Job)
	e := c.cc.Invoke(ctx, "/jfdispatch.v1.Scheduler/Submit", in, out)
	return out, e
}
func (c *SchedulerClient) Get(ctx context.Context, in *api.JobRef) (*api.Job, error) {
	out := new(api.Job)
	e := c.cc.Invoke(ctx, "/jfdispatch.v1.Scheduler/Get", in, out)
	return out, e
}
func (c *SchedulerClient) Cancel(ctx context.Context, in *api.JobRef) (*api.CancelReply, error) {
	out := new(api.CancelReply)
	e := c.cc.Invoke(ctx, "/jfdispatch.v1.Scheduler/Cancel", in, out)
	return out, e
}
func (c *SchedulerClient) Register(ctx context.Context, in *api.WorkerStatus) (*api.RegisterReply, error) {
	out := new(api.RegisterReply)
	e := c.cc.Invoke(ctx, "/jfdispatch.v1.Scheduler/Register", in, out)
	return out, e
}
func (c *SchedulerClient) Heartbeat(ctx context.Context, in *api.WorkerStatus) (*api.HeartbeatReply, error) {
	out := new(api.HeartbeatReply)
	e := c.cc.Invoke(ctx, "/jfdispatch.v1.Scheduler/Heartbeat", in, out)
	return out, e
}
func (c *SchedulerClient) ListWorkers(ctx context.Context, in *api.Empty) (*api.WorkerList, error) {
	out := new(api.WorkerList)
	e := c.cc.Invoke(ctx, "/jfdispatch.v1.Scheduler/ListWorkers", in, out)
	return out, e
}
func (c *SchedulerClient) Watch(ctx context.Context, in *api.JobRef) (grpc.ServerStreamingClient[api.JobEvent], error) {
	sd := &grpc.StreamDesc{ServerStreams: true}
	st, e := c.cc.NewStream(ctx, sd, "/jfdispatch.v1.Scheduler/Watch")
	if e != nil {
		return nil, e
	}
	if e = st.SendMsg(in); e != nil {
		return nil, e
	}
	if e = st.CloseSend(); e != nil {
		return nil, e
	}
	return &clientStream[api.JobEvent]{st}, nil
}

type WorkerClient struct{ cc grpc.ClientConnInterface }

func NewWorkerClient(c grpc.ClientConnInterface) *WorkerClient { return &WorkerClient{c} }
func (c *WorkerClient) Run(ctx context.Context, in *api.RunRequest) (grpc.ServerStreamingClient[api.JobEvent], error) {
	st, e := c.cc.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true}, "/jfdispatch.v1.Worker/Run")
	if e != nil {
		return nil, e
	}
	if e = st.SendMsg(in); e != nil {
		return nil, e
	}
	if e = st.CloseSend(); e != nil {
		return nil, e
	}
	return &clientStream[api.JobEvent]{st}, nil
}
func (c *WorkerClient) Cancel(ctx context.Context, in *api.JobRef) (*api.CancelReply, error) {
	out := new(api.CancelReply)
	e := c.cc.Invoke(ctx, "/jfdispatch.v1.Worker/Cancel", in, out)
	return out, e
}

type clientStream[T any] struct{ grpc.ClientStream }

func (c *clientStream[T]) Recv() (*T, error) {
	v := new(T)
	if e := c.RecvMsg(v); e != nil {
		return nil, e
	}
	return v, nil
}
