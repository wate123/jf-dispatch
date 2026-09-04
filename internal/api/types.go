package api

import "time"

type Empty struct{}
type Requirements struct {
	DecodeCodec string `json:"decode_codec"`
	EncodeCodec string `json:"encode_codec"`
	HWAccel     string `json:"hwaccel"`
	ToneMap     bool   `json:"tone_map"`
}
type SubmitRequest struct {
	Args         []string          `json:"args"`
	Requirements Requirements      `json:"requirements"`
	Env          map[string]string `json:"env,omitempty"`
}
type Job struct {
	ID           string       `json:"id"`
	State        string       `json:"state"`
	WorkerID     string       `json:"worker_id,omitempty"`
	Args         []string     `json:"args"`
	Requirements Requirements `json:"requirements"`
	ExitCode     int          `json:"exit_code"`
	Error        string       `json:"error,omitempty"`
	CreatedUnix  int64        `json:"created_unix"`
	UpdatedUnix  int64        `json:"updated_unix"`
}
type JobRef struct {
	ID string `json:"id"`
}
type CancelReply struct {
	Accepted bool `json:"accepted"`
}
type WorkerCapability struct {
	ID       string   `json:"id"`
	Arch     string   `json:"arch"`
	HWAccels []string `json:"hwaccels"`
	Decoders []string `json:"decoders"`
	Encoders []string `json:"encoders"`
	DRI      bool     `json:"dri"`
	NVIDIA   bool     `json:"nvidia"`
	ToneMap  bool     `json:"tone_map"`
	MaxJobs  int      `json:"max_jobs"`
}
type WorkerStatus struct {
	Capability WorkerCapability `json:"capability"`
	ActiveJobs int              `json:"active_jobs"`
	CPULoad    float64          `json:"cpu_load"`
	GPULoad    float64          `json:"gpu_load"`
	Address    string           `json:"address"`
	SeenUnix   int64            `json:"seen_unix"`
}
type RegisterReply struct {
	HeartbeatSeconds int `json:"heartbeat_seconds"`
}
type HeartbeatReply struct {
	Accepted bool `json:"accepted"`
}
type WorkerList struct {
	Workers []WorkerStatus `json:"workers"`
}
type RunRequest struct {
	Job Job               `json:"job"`
	Env map[string]string `json:"env,omitempty"`
}
type JobEvent struct {
	JobID    string `json:"job_id"`
	State    string `json:"state"`
	Message  string `json:"message,omitempty"`
	ExitCode int    `json:"exit_code"`
	Unix     int64  `json:"unix"`
}

func Event(id, state, msg string, code int) JobEvent {
	return JobEvent{JobID: id, State: state, Message: msg, ExitCode: code, Unix: time.Now().Unix()}
}

const (
	StateSubmitted = "submitted"
	StateAssigned  = "assigned"
	StateRunning   = "running"
	StateCompleted = "completed"
	StateFailed    = "failed"
	StateCanceled  = "canceled"
)
