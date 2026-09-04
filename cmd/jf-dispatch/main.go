package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/example/jf-dispatch/internal/api"
	"github.com/example/jf-dispatch/internal/auth"
	"github.com/example/jf-dispatch/internal/capability"
	"github.com/example/jf-dispatch/internal/config"
	networkutil "github.com/example/jf-dispatch/internal/network"
	"github.com/example/jf-dispatch/internal/rpc"
	schedulerpkg "github.com/example/jf-dispatch/internal/scheduler"
	workerpkg "github.com/example/jf-dispatch/internal/worker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var version = "dev"

func main() {
	// The installer creates these compatibility links so Jellyfin can invoke the
	// wrapper as a normal FFmpeg executable without supplying a subcommand.
	switch filepath.Base(os.Args[0]) {
	case "jf-ffmpeg-wrapper":
		if err := runFFmpeg(os.Args[1:]); err != nil {
			log.Print(err)
			os.Exit(1)
		}
		return
	case "jf-scheduler":
		if err := runScheduler(os.Args[1:]); err != nil {
			log.Print(err)
			os.Exit(1)
		}
		return
	case "jf-worker":
		if err := runWorker(os.Args[1:]); err != nil {
			log.Print(err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "scheduler":
		err = runScheduler(os.Args[2:])
	case "worker":
		err = runWorker(os.Args[2:])
	case "ffmpeg":
		err = runFFmpeg(os.Args[2:])
	case "doctor":
		err = runDoctor(os.Args[2:])
	case "nodes":
		err = runNodes(os.Args[2:])
	case "config":
		err = runConfig(os.Args[2:])
	case "version", "--version", "-version":
		fmt.Println(version)
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
func usage() {
	fmt.Fprintln(os.Stderr, "jf-dispatch <scheduler|worker|ffmpeg|doctor|nodes|config validate|version>")
}
func load(args []string, name string) (config.Config, []string, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	path := fs.String("config", env("JF_CONFIG", ""), "YAML config path")
	if err := fs.Parse(args); err != nil {
		return config.Config{}, nil, err
	}
	c, err := config.Load(*path)
	return c, fs.Args(), err
}

func runScheduler(args []string) error {
	c, rest, e := load(args, "scheduler")
	if e != nil {
		return e
	}
	if len(rest) > 0 {
		return fmt.Errorf("unexpected arguments: %v", rest)
	}
	s := schedulerpkg.NewWithToken(c.Security.Token)
	g := grpc.NewServer(grpc.ForceServerCodec(rpc.JSONCodec{}), grpc.UnaryInterceptor(auth.Unary(c.Security.Token)), grpc.StreamInterceptor(auth.Stream(c.Security.Token)))
	rpc.RegisterSchedulerServer(g, s)
	l, e := net.Listen("tcp", c.Scheduler.Listen)
	if e != nil {
		return e
	}
	if c.Metrics.Enabled {
		go serveMetrics(c.Metrics.Listen)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() { <-ctx.Done(); g.GracefulStop() }()
	log.Printf("scheduler %s listening on %s", version, c.Scheduler.Listen)
	return g.Serve(l)
}
func serveMetrics(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprintln(w, "ok") })
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintln(w, "# HELP jf_scheduler_up Scheduler availability\n# TYPE jf_scheduler_up gauge\njf_scheduler_up 1")
	})
	if e := http.ListenAndServe(addr, mux); e != nil {
		log.Printf("metrics: %v", e)
	}
}

func runWorker(args []string) error {
	c, rest, e := load(args, "worker")
	if e != nil {
		return e
	}
	if len(rest) > 0 {
		return fmt.Errorf("unexpected arguments: %v", rest)
	}
	id := c.Worker.ID
	if id == "" || id == "auto" {
		id, _ = os.Hostname()
	}
	advertise, e := networkutil.Advertise(c.Worker.Advertise, c.Worker.AdvertiseVia, c.Worker.Listen)
	if e != nil {
		return fmt.Errorf("advertise address: %w", e)
	}
	allowed := append([]string{}, c.Security.AllowedPaths...)
	if len(allowed) == 0 {
		allowed = append(allowed, c.Storage.MediaPaths...)
		if c.Storage.TranscodePath != "" {
			allowed = append(allowed, c.Storage.TranscodePath)
		}
	}
	w := workerpkg.NewWithPolicy(c.Worker.FFmpeg, allowed, c.Security.AllowEnvironment)
	g := grpc.NewServer(grpc.ForceServerCodec(rpc.JSONCodec{}), grpc.UnaryInterceptor(auth.Unary(c.Security.Token)), grpc.StreamInterceptor(auth.Stream(c.Security.Token)))
	rpc.RegisterWorkerServer(g, w)
	l, e := net.Listen("tcp", c.Worker.Listen)
	if e != nil {
		return e
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	cap := capability.Detect(id, c.Worker.MaxJobs)
	go heartbeat(ctx, c.Worker.Scheduler, advertise, c.Security.Token, c.Worker.Heartbeat, cap, w)
	go func() { <-ctx.Done(); g.GracefulStop() }()
	log.Printf("worker=%s arch=%s advertise=%s hwaccels=%v", id, cap.Arch, advertise, cap.HWAccels)
	return g.Serve(l)
}
func heartbeat(ctx context.Context, address, advertise, token string, interval time.Duration, cap api.WorkerCapability, w *workerpkg.Worker) {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		cc, e := dial(ctx, address, token)
		if e == nil {
			_, e = rpc.NewSchedulerClient(cc).Heartbeat(ctx, &api.WorkerStatus{Capability: cap, ActiveJobs: w.Active(), CPULoad: capability.CPULoad(), Address: advertise})
			cc.Close()
		}
		if e != nil {
			log.Printf("heartbeat: %v", e)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func runFFmpeg(args []string) error {
	configPath := env("JF_CONFIG", "")
	ffargs := args
	if len(args) >= 2 && args[0] == "-config" {
		configPath, ffargs = args[1], args[2:]
	}
	c, e := config.Load(configPath)
	if e != nil {
		return e
	}
	if len(ffargs) == 0 {
		return fmt.Errorf("ffmpeg arguments required")
	}
	ctx := context.Background()
	cc, e := dial(ctx, c.Scheduler.Address, c.Security.Token)
	if e != nil {
		return e
	}
	defer cc.Close()
	client := rpc.NewSchedulerClient(cc)
	j, e := client.Submit(ctx, &api.SubmitRequest{Args: ffargs, Requirements: infer(ffargs)})
	if e != nil {
		return e
	}
	st, e := client.Watch(ctx, &api.JobRef{ID: j.ID})
	if e != nil {
		return e
	}
	for {
		ev, e := st.Recv()
		if e == io.EOF {
			return nil
		}
		if e != nil {
			return e
		}
		if ev.Message != "" && ev.Message != "snapshot" {
			fmt.Fprintln(os.Stderr, ev.Message)
		}
		if ev.State == api.StateCompleted {
			return nil
		}
		if ev.State == api.StateFailed || ev.State == api.StateCanceled {
			return fmt.Errorf("job %s: %s", ev.State, ev.Message)
		}
	}
}

func runNodes(args []string) error {
	c, rest, e := load(args, "nodes")
	if e != nil {
		return e
	}
	if len(rest) > 0 {
		return fmt.Errorf("unexpected arguments: %v", rest)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cc, e := dial(ctx, c.Scheduler.Address, c.Security.Token)
	if e != nil {
		return e
	}
	defer cc.Close()
	list, e := rpc.NewSchedulerClient(cc).ListWorkers(ctx, &api.Empty{})
	if e != nil {
		return e
	}
	sort.Slice(list.Workers, func(i, j int) bool { return list.Workers[i].Capability.ID < list.Workers[j].Capability.ID })
	fmt.Printf("%-18s %-7s %-22s %-8s %s\n", "NODE", "ARCH", "ACCELERATORS", "JOBS", "STATUS")
	now := time.Now().Unix()
	for _, w := range list.Workers {
		state := "healthy"
		if now-w.SeenUnix > 20 {
			state = "offline"
		}
		fmt.Printf("%-18s %-7s %-22s %d/%-6d %s\n", w.Capability.ID, w.Capability.Arch, strings.Join(w.Capability.HWAccels, ","), w.ActiveJobs, w.Capability.MaxJobs, state)
	}
	return nil
}

func runDoctor(args []string) error {
	c, rest, e := load(args, "doctor")
	if e != nil {
		return e
	}
	if len(rest) > 0 {
		return fmt.Errorf("unexpected arguments: %v", rest)
	}
	failed := false
	check := func(name string, e error) {
		if e == nil {
			fmt.Printf("✓ %s\n", name)
		} else {
			failed = true
			fmt.Printf("✗ %s: %v\n", name, e)
		}
	}
	_, e = exec.LookPath(c.Worker.FFmpeg)
	check("FFmpeg available", e)
	ip, e := networkutil.TailscaleIPv4()
	if e == nil {
		check("Tailscale address "+ip, nil)
	} else {
		check("Tailscale", e)
	}
	for _, p := range c.Storage.MediaPaths {
		e = readable(p)
		check("media path "+p, e)
	}
	if c.Storage.TranscodePath != "" {
		e = writable(c.Storage.TranscodePath)
		check("transcode path "+c.Storage.TranscodePath, e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cc, e := dial(ctx, c.Worker.Scheduler, c.Security.Token)
	if e == nil {
		_, e = rpc.NewSchedulerClient(cc).ListWorkers(ctx, &api.Empty{})
		cc.Close()
	}
	check("scheduler reachable "+c.Worker.Scheduler, e)
	cap := capability.Detect("doctor", 1)
	if len(cap.Encoders) == 0 {
		check("FFmpeg encoders", fmt.Errorf("none detected"))
	} else {
		check(fmt.Sprintf("capabilities arch=%s hw=%s", cap.Arch, strings.Join(cap.HWAccels, ",")), nil)
	}
	if failed {
		return fmt.Errorf("doctor found problems")
	}
	return nil
}
func readable(p string) error {
	f, e := os.Open(p)
	if e == nil {
		e = f.Close()
	}
	return e
}
func writable(p string) error {
	f, e := os.CreateTemp(p, ".jf-dispatch-check-")
	if e != nil {
		return e
	}
	name := f.Name()
	f.Close()
	return os.Remove(name)
}
func runConfig(args []string) error {
	if len(args) == 0 || args[0] != "validate" {
		return fmt.Errorf("usage: jf-dispatch config validate [-config path]")
	}
	c, rest, e := load(args[1:], "config validate")
	if e != nil {
		return e
	}
	if len(rest) > 0 {
		return fmt.Errorf("unexpected arguments: %v", rest)
	}
	fmt.Printf("configuration valid (version %d)\n", c.Version)
	return nil
}
func dial(ctx context.Context, address, token string) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.ForceCodec(rpc.JSONCodec{}))}
	if token != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(auth.TokenCredentials{Token: token}))
	}
	return grpc.DialContext(ctx, address, opts...)
}
func infer(a []string) api.Requirements {
	r := api.Requirements{HWAccel: "cpu"}
	for i, v := range a {
		switch v {
		case "-hwaccel":
			if i+1 < len(a) {
				r.HWAccel = a[i+1]
			}
		case "-c:v", "-codec:v", "-vcodec":
			if i+1 < len(a) {
				r.EncodeCodec = codec(a[i+1])
			}
		}
	}
	return r
}
func codec(v string) string {
	v = strings.ToLower(v)
	if strings.Contains(v, "264") {
		return "h264"
	}
	if strings.Contains(v, "265") {
		return "hevc"
	}
	for _, x := range []string{"h264", "hevc", "av1", "vp9", "mpeg2video"} {
		if strings.Contains(v, x) {
			return x
		}
	}
	return v
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
