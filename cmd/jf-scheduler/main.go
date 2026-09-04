package main

import (
	"flag"
	"fmt"
	"github.com/example/jf-dispatch/internal/rpc"
	sched "github.com/example/jf-dispatch/internal/scheduler"
	"google.golang.org/grpc"
	"log"
	"net"
	"net/http"
)

func main() {
	listen := flag.String("listen", env("JF_SCHEDULER_LISTEN", ":7000"), "gRPC listen address")
	metrics := flag.String("metrics", env("JF_METRICS_LISTEN", ":9090"), "metrics listen address")
	flag.Parse()
	s := sched.New()
	g := grpc.NewServer(grpc.ForceServerCodec(rpc.JSONCodec{}))
	rpc.RegisterSchedulerServer(g, s)
	l, e := net.Listen("tcp", *listen)
	if e != nil {
		log.Fatal(e)
	}
	go func() {
		http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprintln(w, "ok") })
		http.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain; version=0.0.4")
			fmt.Fprintln(w, "# HELP jf_scheduler_up Scheduler availability\n# TYPE jf_scheduler_up gauge\njf_scheduler_up 1")
		})
		log.Printf("metrics on %s", *metrics)
		log.Print(http.ListenAndServe(*metrics, nil))
	}()
	log.Printf("scheduler gRPC on %s", *listen)
	log.Fatal(g.Serve(l))
}
func env(k, d string) string {
	if v := lookup(k); v != "" {
		return v
	}
	return d
}

var lookup = func(k string) string { v, _ := syscallGetenv(k); return v }
var syscallGetenv = func(k string) (string, bool) { return getEnv(k) }
