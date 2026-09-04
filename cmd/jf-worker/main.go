package main

import (
	"context"
	"flag"
	"github.com/example/jf-dispatch/internal/api"
	"github.com/example/jf-dispatch/internal/capability"
	"github.com/example/jf-dispatch/internal/rpc"
	workerpkg "github.com/example/jf-dispatch/internal/worker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"net"
	"os"
	"os/signal"
	"time"
)

func main() {
	id := flag.String("id", env("JF_WORKER_ID", hostname()), "worker id")
	listen := flag.String("listen", env("JF_WORKER_LISTEN", ":7100"), "gRPC listen")
	advertise := flag.String("advertise", env("JF_WORKER_ADVERTISE", "127.0.0.1:7100"), "scheduler-reachable address")
	scheduler := flag.String("scheduler", env("JF_SCHEDULER_ADDR", "127.0.0.1:7000"), "scheduler address")
	ffmpeg := flag.String("ffmpeg", env("JF_FFMPEG", "ffmpeg"), "ffmpeg binary")
	max := flag.Int("max-jobs", 2, "maximum parallel jobs")
	flag.Parse()
	w := workerpkg.New(*ffmpeg)
	g := grpc.NewServer(grpc.ForceServerCodec(rpc.JSONCodec{}))
	rpc.RegisterWorkerServer(g, w)
	l, e := net.Listen("tcp", *listen)
	if e != nil {
		log.Fatal(e)
	}
	go func() { log.Printf("worker %s gRPC on %s", *id, *listen); log.Fatal(g.Serve(l)) }()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	cap := capability.Detect(*id, *max)
	log.Printf("detected arch=%s hwaccels=%v dri=%v nvidia=%v", cap.Arch, cap.HWAccels, cap.DRI, cap.NVIDIA)
	go heartbeat(ctx, *scheduler, *advertise, cap, w)
	<-ctx.Done()
	g.GracefulStop()
}
func heartbeat(ctx context.Context, scheduler, address string, cap api.WorkerCapability, w *workerpkg.Worker) {
	for {
		cc, e := grpc.DialContext(ctx, scheduler, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.ForceCodec(rpc.JSONCodec{})))
		if e == nil {
			c := rpc.NewSchedulerClient(cc)
			status := &api.WorkerStatus{Capability: cap, ActiveJobs: w.Active(), CPULoad: capability.CPULoad(), Address: address}
			_, e = c.Heartbeat(ctx, status)
			cc.Close()
		}
		if e != nil {
			log.Printf("heartbeat: %v", e)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func hostname() string { v, _ := os.Hostname(); return v }
