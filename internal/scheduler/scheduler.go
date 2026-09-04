package scheduler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/example/jf-dispatch/internal/api"
	"github.com/example/jf-dispatch/internal/auth"
	"github.com/example/jf-dispatch/internal/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type Scheduler struct {
	mu       sync.RWMutex
	jobs     map[string]*api.Job
	workers  map[string]*api.WorkerStatus
	watchers map[string][]chan api.JobEvent
	seq      uint64
	token    string
}

func NewWithToken(token string) *Scheduler { s := New(); s.token = token; return s }

func New() *Scheduler {
	return &Scheduler{jobs: map[string]*api.Job{}, workers: map[string]*api.WorkerStatus{}, watchers: map[string][]chan api.JobEvent{}}
}
func (s *Scheduler) Submit(_ context.Context, r *api.SubmitRequest) (*api.Job, error) {
	if len(r.Args) == 0 {
		return nil, status.Error(codes.InvalidArgument, "ffmpeg args required")
	}
	s.mu.Lock()
	s.seq++
	id := fmt.Sprintf("job-%06d", s.seq)
	now := time.Now().Unix()
	j := &api.Job{ID: id, State: api.StateSubmitted, Args: append([]string(nil), r.Args...), Requirements: r.Requirements, CreatedUnix: now, UpdatedUnix: now}
	s.jobs[id] = j
	s.mu.Unlock()
	s.publish(api.Event(id, api.StateSubmitted, "accepted", 0))
	go s.dispatch(id, r.Env)
	return clone(j), nil
}
func (s *Scheduler) Get(_ context.Context, r *api.JobRef) (*api.Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j := s.jobs[r.ID]
	if j == nil {
		return nil, status.Error(codes.NotFound, "job not found")
	}
	return clone(j), nil
}
func (s *Scheduler) Register(_ context.Context, w *api.WorkerStatus) (*api.RegisterReply, error) {
	s.upsert(w)
	return &api.RegisterReply{HeartbeatSeconds: 5}, nil
}
func (s *Scheduler) Heartbeat(_ context.Context, w *api.WorkerStatus) (*api.HeartbeatReply, error) {
	s.upsert(w)
	return &api.HeartbeatReply{Accepted: true}, nil
}
func (s *Scheduler) ListWorkers(_ context.Context, _ *api.Empty) (*api.WorkerList, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := &api.WorkerList{Workers: make([]api.WorkerStatus, 0, len(s.workers))}
	for _, worker := range s.workers {
		out.Workers = append(out.Workers, *worker)
	}
	sort.Slice(out.Workers, func(i, j int) bool { return out.Workers[i].Capability.ID < out.Workers[j].Capability.ID })
	return out, nil
}
func (s *Scheduler) upsert(w *api.WorkerStatus) {
	w.SeenUnix = time.Now().Unix()
	s.mu.Lock()
	c := *w
	s.workers[w.Capability.ID] = &c
	s.mu.Unlock()
	log.Printf("worker=%s arch=%s active=%d address=%s heartbeat", w.Capability.ID, w.Capability.Arch, w.ActiveJobs, w.Address)
}
func (s *Scheduler) Watch(r *api.JobRef, stream grpc.ServerStreamingServer[api.JobEvent]) error {
	s.mu.Lock()
	j := s.jobs[r.ID]
	if j == nil {
		s.mu.Unlock()
		return status.Error(codes.NotFound, "job not found")
	}
	ch := make(chan api.JobEvent, 64)
	s.watchers[r.ID] = append(s.watchers[r.ID], ch)
	message := "snapshot"
	if j.Error != "" {
		message = j.Error
	}
	current := api.Event(j.ID, j.State, message, j.ExitCode)
	s.mu.Unlock()
	defer s.removeWatcher(r.ID, ch)
	if err := stream.Send(&current); err != nil {
		return err
	}
	if terminal(current.State) {
		return nil
	}
	for {
		select {
		case e := <-ch:
			if err := stream.Send(&e); err != nil {
				return err
			}
			if terminal(e.State) {
				return nil
			}
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}
func (s *Scheduler) Cancel(ctx context.Context, r *api.JobRef) (*api.CancelReply, error) {
	s.mu.RLock()
	j := s.jobs[r.ID]
	if j == nil {
		s.mu.RUnlock()
		return nil, status.Error(codes.NotFound, "job not found")
	}
	wid := j.WorkerID
	s.mu.RUnlock()
	if terminal(j.State) {
		return &api.CancelReply{}, nil
	}
	if wid != "" {
		s.mu.RLock()
		w := s.workers[wid]
		s.mu.RUnlock()
		if w != nil {
			cc, e := dial(ctx, w.Address, s.token)
			if e == nil {
				defer cc.Close()
				_, _ = rpc.NewWorkerClient(cc).Cancel(ctx, r)
			}
		}
	}
	s.update(r.ID, api.StateCanceled, "canceled", -1)
	return &api.CancelReply{Accepted: true}, nil
}
func (s *Scheduler) dispatch(id string, env map[string]string) {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		w := s.choose(id)
		if w != nil {
			s.run(id, w, env)
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	s.mu.RLock()
	req := s.jobs[id].Requirements
	s.mu.RUnlock()
	log.Printf("job=%s no capable worker requirements=%+v", id, req)
	s.update(id, api.StateFailed, "no capable worker available", -1)
}
func (s *Scheduler) choose(id string) *api.WorkerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j := s.jobs[id]
	var c []*api.WorkerStatus
	for _, w := range s.workers {
		if time.Now().Unix()-w.SeenUnix <= 15 && matches(j.Requirements, w.Capability) && w.ActiveJobs < w.Capability.MaxJobs {
			v := *w
			c = append(c, &v)
		}
	}
	sort.Slice(c, func(i, k int) bool { return score(c[i]) < score(c[k]) })
	if len(c) > 0 {
		return c[0]
	}
	return nil
}
func score(w *api.WorkerStatus) float64 {
	return float64(w.ActiveJobs)*100 + w.CPULoad*.3 + w.GPULoad*.5
}
func matches(r api.Requirements, c api.WorkerCapability) bool {
	return has(c.Decoders, r.DecodeCodec) && has(c.Encoders, r.EncodeCodec) && has(c.HWAccels, r.HWAccel) && (!r.ToneMap || c.ToneMap)
}
func has(xs []string, v string) bool {
	if v == "" || v == "cpu" {
		return true
	}
	for _, x := range xs {
		if strings.EqualFold(x, v) || strings.Contains(strings.ToLower(x), strings.ToLower(v)) {
			return true
		}
	}
	return false
}
func (s *Scheduler) run(id string, w *api.WorkerStatus, env map[string]string) {
	s.mu.Lock()
	j := s.jobs[id]
	if j == nil || terminal(j.State) {
		s.mu.Unlock()
		return
	}
	j.WorkerID = w.Capability.ID
	j.State = api.StateAssigned
	j.UpdatedUnix = time.Now().Unix()
	req := api.RunRequest{Job: *clone(j), Env: env}
	s.mu.Unlock()
	s.publish(api.Event(id, api.StateAssigned, "assigned to "+w.Capability.ID, 0))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cc, e := dial(ctx, w.Address, s.token)
	if e != nil {
		s.update(id, api.StateFailed, e.Error(), -1)
		return
	}
	defer cc.Close()
	stream, e := rpc.NewWorkerClient(cc).Run(ctx, &req)
	if e != nil {
		s.update(id, api.StateFailed, e.Error(), -1)
		return
	}
	for {
		ev, e := stream.Recv()
		if errors.Is(e, io.EOF) {
			return
		}
		if e != nil {
			s.update(id, api.StateFailed, e.Error(), -1)
			return
		}
		s.update(id, ev.State, ev.Message, ev.ExitCode)
		if terminal(ev.State) {
			return
		}
	}
}
func dial(ctx context.Context, a, token string) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.ForceCodec(rpc.JSONCodec{}))}
	if token != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(auth.TokenCredentials{Token: token}))
	}
	return grpc.DialContext(ctx, a, opts...)
}
func (s *Scheduler) update(id, state, msg string, code int) {
	s.mu.Lock()
	j := s.jobs[id]
	if j == nil || terminal(j.State) {
		s.mu.Unlock()
		return
	}
	j.State = state
	j.ExitCode = code
	j.UpdatedUnix = time.Now().Unix()
	if state == api.StateFailed {
		j.Error = msg
	}
	s.mu.Unlock()
	s.publish(api.Event(id, state, msg, code))
}
func (s *Scheduler) publish(e api.JobEvent) {
	s.mu.RLock()
	chs := append([]chan api.JobEvent(nil), s.watchers[e.JobID]...)
	s.mu.RUnlock()
	for _, ch := range chs {
		select {
		case ch <- e:
		default:
		}
	}
}
func (s *Scheduler) removeWatcher(id string, ch chan api.JobEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.watchers[id]
	for i, v := range a {
		if v == ch {
			s.watchers[id] = append(a[:i], a[i+1:]...)
			break
		}
	}
}
func terminal(v string) bool {
	return v == api.StateCompleted || v == api.StateFailed || v == api.StateCanceled
}
func clone(j *api.Job) *api.Job { c := *j; c.Args = append([]string(nil), j.Args...); return &c }
