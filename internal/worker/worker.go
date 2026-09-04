package worker

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/example/jf-dispatch/internal/api"
	"google.golang.org/grpc"
)

type Worker struct {
	FFmpeg           string
	AllowedPaths     []string
	AllowEnvironment bool
	mu               sync.Mutex
	running          map[string]*exec.Cmd
}

func New(ffmpeg string) *Worker { return NewWithPolicy(ffmpeg, nil, true) }
func NewWithPolicy(ffmpeg string, allowed []string, allowEnv bool) *Worker {
	return &Worker{FFmpeg: ffmpeg, AllowedPaths: allowed, AllowEnvironment: allowEnv, running: map[string]*exec.Cmd{}}
}
func (w *Worker) Run(r *api.RunRequest, stream grpc.ServerStreamingServer[api.JobEvent]) error {
	if e := w.validate(r); e != nil {
		ev := api.Event(r.Job.ID, api.StateFailed, e.Error(), -1)
		return stream.Send(&ev)
	}
	ctx := stream.Context()
	cmd := exec.CommandContext(ctx, w.FFmpeg, r.Job.Args...)
	cmd.Env = os.Environ()
	for k, v := range r.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	stderr, e := cmd.StderrPipe()
	if e != nil {
		return e
	}
	stdout, e := cmd.StdoutPipe()
	if e != nil {
		return e
	}
	if e = cmd.Start(); e != nil {
		ev := api.Event(r.Job.ID, api.StateFailed, e.Error(), -1)
		return stream.Send(&ev)
	}
	w.mu.Lock()
	w.running[r.Job.ID] = cmd
	w.mu.Unlock()
	defer func() { w.mu.Lock(); delete(w.running, r.Job.ID); w.mu.Unlock() }()
	ev := api.Event(r.Job.ID, api.StateRunning, "ffmpeg started", 0)
	if e = stream.Send(&ev); e != nil {
		return e
	}
	lines := make(chan string, 64)
	scan := func(s *bufio.Scanner) {
		for s.Scan() {
			select {
			case lines <- s.Text():
			case <-ctx.Done():
				return
			}
		}
	}
	go scan(bufio.NewScanner(stderr))
	go scan(bufio.NewScanner(stdout))
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	for {
		select {
		case line := <-lines:
			ev = api.Event(r.Job.ID, api.StateRunning, line, 0)
			if e = stream.Send(&ev); e != nil {
				return e
			}
		case e = <-done:
			state, code := api.StateCompleted, 0
			if e != nil {
				state = api.StateFailed
				code = -1
				if x, ok := e.(*exec.ExitError); ok {
					code = x.ExitCode()
				}
			}
			if ctx.Err() != nil {
				state = api.StateCanceled
			}
			ev = api.Event(r.Job.ID, state, fmt.Sprint(e), code)
			return stream.Send(&ev)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
func (w *Worker) validate(r *api.RunRequest) error {
	if !w.AllowEnvironment && len(r.Env) > 0 {
		return fmt.Errorf("job environment overrides are disabled")
	}
	if len(w.AllowedPaths) == 0 {
		return nil
	}
	for _, arg := range r.Job.Args {
		if !filepath.IsAbs(arg) || strings.HasPrefix(arg, "/dev/") {
			continue
		}
		allowed := false
		for _, root := range w.AllowedPaths {
			rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(arg))
			if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("path %q is outside allowed paths", arg)
		}
	}
	return nil
}
func (w *Worker) Cancel(_ context.Context, r *api.JobRef) (*api.CancelReply, error) {
	w.mu.Lock()
	cmd := w.running[r.ID]
	w.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return &api.CancelReply{}, nil
	}
	e := cmd.Process.Signal(syscall.SIGTERM)
	return &api.CancelReply{Accepted: e == nil}, e
}
func (w *Worker) Active() int { w.mu.Lock(); defer w.mu.Unlock(); return len(w.running) }
