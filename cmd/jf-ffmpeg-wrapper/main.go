package main

import (
	"context"
	"fmt"
	"github.com/example/jf-dispatch/internal/api"
	"github.com/example/jf-dispatch/internal/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"io"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: jf-ffmpeg-wrapper <ffmpeg args>")
		os.Exit(2)
	}
	ctx := context.Background()
	addr := env("JF_SCHEDULER_ADDR", "127.0.0.1:7000")
	cc, e := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.ForceCodec(rpc.JSONCodec{})))
	if e != nil {
		fatal(e)
	}
	defer cc.Close()
	c := rpc.NewSchedulerClient(cc)
	req := &api.SubmitRequest{Args: os.Args[1:], Requirements: infer(os.Args[1:])}
	j, e := c.Submit(ctx, req)
	if e != nil {
		fatal(e)
	}
	st, e := c.Watch(ctx, &api.JobRef{ID: j.ID})
	if e != nil {
		fatal(e)
	}
	for {
		ev, e := st.Recv()
		if e == io.EOF {
			return
		}
		if e != nil {
			fatal(e)
		}
		if ev.Message != "" && ev.Message != "snapshot" {
			fmt.Fprintln(os.Stderr, ev.Message)
		}
		switch ev.State {
		case api.StateCompleted:
			os.Exit(ev.ExitCode)
		case api.StateFailed, api.StateCanceled:
			if ev.ExitCode == 0 {
				ev.ExitCode = 1
			}
			os.Exit(ev.ExitCode)
		}
	}
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
func fatal(e error) { fmt.Fprintln(os.Stderr, e); os.Exit(1) }
