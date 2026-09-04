package capability

import "testing"

func TestDetectFFmpeg(t *testing.T) {
	c := Detect("test", 1)
	if len(c.Encoders) == 0 || len(c.Decoders) == 0 {
		t.Fatalf("ffmpeg capabilities not detected: %+v", c)
	}
	t.Logf("detected: %+v", c)
}
