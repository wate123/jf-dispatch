package capability

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/example/jf-dispatch/internal/api"
)

func Detect(id string, maxJobs int) api.WorkerCapability {
	c := api.WorkerCapability{ID: id, Arch: runtime.GOARCH, MaxJobs: maxJobs}
	c.DRI = exists("/dev/dri/renderD128")
	c.NVIDIA = commandOK("nvidia-smi")
	c.HWAccels = ffmpegList("-hwaccels")
	c.Decoders = ffmpegCodecs("-decoders")
	c.Encoders = ffmpegCodecs("-encoders")
	c.ToneMap = contains(c.HWAccels, "opencl") || contains(c.HWAccels, "cuda") || c.DRI
	return c
}
func exists(p string) bool { _, e := os.Stat(p); return e == nil }
func commandOK(n string) bool {
	ctx, c := context.WithTimeout(context.Background(), 2*time.Second)
	defer c()
	return exec.CommandContext(ctx, n, "--query-gpu=name", "--format=csv,noheader").Run() == nil
}
func ffmpegList(flag string) []string {
	o, e := exec.Command("ffmpeg", "-hide_banner", flag).CombinedOutput()
	if e != nil {
		return nil
	}
	var r []string
	s := bufio.NewScanner(strings.NewReader(string(o)))
	for s.Scan() {
		v := strings.TrimSpace(s.Text())
		if v != "" && !strings.Contains(v, "Hardware acceleration") {
			r = append(r, strings.Fields(v)[0])
		}
	}
	return unique(r)
}
func ffmpegCodecs(flag string) []string {
	o, e := exec.Command("ffmpeg", "-hide_banner", flag).CombinedOutput()
	if e != nil {
		return nil
	}
	var r []string
	s := bufio.NewScanner(strings.NewReader(string(o)))
	for s.Scan() {
		f := strings.Fields(s.Text())
		if len(f) > 1 && len(f[0]) == 6 {
			r = append(r, normalize(f[1]))
		}
	}
	return unique(r)
}
func normalize(v string) string {
	if strings.Contains(v, "264") {
		return "h264"
	}
	if strings.Contains(v, "265") {
		return "hevc"
	}
	for _, p := range []string{"h264", "hevc", "av1", "vp9", "mpeg2video", "aac"} {
		if strings.Contains(v, p) {
			return p
		}
	}
	return v
}
func contains(a []string, v string) bool {
	for _, x := range a {
		if x == v {
			return true
		}
	}
	return false
}
func unique(a []string) []string {
	m := map[string]bool{}
	r := []string{}
	for _, x := range a {
		if !m[x] {
			m[x] = true
			r = append(r, x)
		}
	}
	return r
}
func CPULoad() float64 {
	b, e := os.ReadFile("/proc/loadavg")
	if e != nil {
		return 0
	}
	f := strings.Fields(string(b))
	if len(f) == 0 {
		return 0
	}
	v, _ := strconv.ParseFloat(f[0], 64)
	return v * 100 / float64(runtime.NumCPU())
}
