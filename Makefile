.PHONY: build test demo
build:
	go build -o jf-dispatch ./cmd/jf-dispatch
	go build ./cmd/...
test:
	go test ./...
demo: build
	@mkdir -p work/demo
	@./jf-scheduler -listen 127.0.0.1:17000 -metrics 127.0.0.1:19090 >work/demo/scheduler.log 2>&1 & SCHED_PID=$$!; ./jf-worker -id local -listen 127.0.0.1:17100 -advertise 127.0.0.1:17100 -scheduler 127.0.0.1:17000 >work/demo/worker.log 2>&1 & WORKER_PID=$$!; trap 'kill $$SCHED_PID $$WORKER_PID 2>/dev/null || true' EXIT; sleep 2; JF_SCHEDULER_ADDR=127.0.0.1:17000 ./jf-ffmpeg-wrapper -f lavfi -i testsrc=size=320x240:rate=10 -t 2 -c:v libx264 -y work/demo/demo.mp4
