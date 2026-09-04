package main

import "testing"

func TestInfer(t *testing.T) {
	r := infer([]string{"-hwaccel", "vaapi", "-i", "x", "-c:v", "h264_vaapi", "out"})
	if r.HWAccel != "vaapi" || r.EncodeCodec != "h264" {
		t.Fatalf("unexpected: %+v", r)
	}
}
