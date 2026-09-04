package worker

import (
	"github.com/example/jf-dispatch/internal/api"
	"testing"
)

func TestPathPolicy(t *testing.T) {
	w := NewWithPolicy("ffmpeg", []string{"/media", "/transcode"}, false)
	if err := w.validate(&api.RunRequest{Job: api.Job{Args: []string{"-i", "/media/movie.mkv", "/transcode/out.mp4"}}}); err != nil {
		t.Fatal(err)
	}
	if err := w.validate(&api.RunRequest{Job: api.Job{Args: []string{"-i", "/etc/passwd"}}}); err == nil {
		t.Fatal("expected path policy rejection")
	}
	if err := w.validate(&api.RunRequest{Env: map[string]string{"LD_PRELOAD": "bad"}}); err == nil {
		t.Fatal("expected environment rejection")
	}
}
