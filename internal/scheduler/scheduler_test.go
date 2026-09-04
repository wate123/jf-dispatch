package scheduler

import (
	"github.com/example/jf-dispatch/internal/api"
	"testing"
)

func TestCapabilityMatching(t *testing.T) {
	c := api.WorkerCapability{Decoders: []string{"hevc"}, Encoders: []string{"h264"}, HWAccels: []string{"vaapi"}, ToneMap: true}
	cases := []struct {
		name string
		r    api.Requirements
		want bool
	}{{"cpu", api.Requirements{EncodeCodec: "h264"}, true}, {"vaapi", api.Requirements{DecodeCodec: "hevc", EncodeCodec: "h264", HWAccel: "vaapi", ToneMap: true}, true}, {"wrong codec", api.Requirements{EncodeCodec: "av1"}, false}, {"wrong accelerator", api.Requirements{HWAccel: "cuda"}, false}}
	for _, x := range cases {
		t.Run(x.name, func(t *testing.T) {
			if got := matches(x.r, c); got != x.want {
				t.Fatalf("matches=%v want %v", got, x.want)
			}
		})
	}
}
func TestLeastLoadedScore(t *testing.T) {
	a := &api.WorkerStatus{ActiveJobs: 1, CPULoad: 10}
	b := &api.WorkerStatus{ActiveJobs: 2}
	if score(a) >= score(b) {
		t.Fatal("expected one-job worker to score lower")
	}
}
