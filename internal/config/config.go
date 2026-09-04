package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version   int             `yaml:"version"`
	Scheduler SchedulerConfig `yaml:"scheduler"`
	Worker    WorkerConfig    `yaml:"worker"`
	Storage   StorageConfig   `yaml:"storage"`
	Metrics   MetricsConfig   `yaml:"metrics"`
	Security  SecurityConfig  `yaml:"security"`
}
type SchedulerConfig struct {
	Listen       string        `yaml:"listen"`
	Address      string        `yaml:"address"`
	OfflineAfter time.Duration `yaml:"offline_after"`
}
type WorkerConfig struct {
	ID              string        `yaml:"id"`
	Listen          string        `yaml:"listen"`
	Advertise       string        `yaml:"advertise"`
	AdvertiseVia    string        `yaml:"advertise_via"`
	Scheduler       string        `yaml:"scheduler"`
	FFmpeg          string        `yaml:"ffmpeg"`
	MaxJobs         int           `yaml:"max_jobs"`
	Heartbeat       time.Duration `yaml:"heartbeat"`
	PreferredAccels []string      `yaml:"preferred_accelerators"`
}
type StorageConfig struct {
	MediaPaths    []string `yaml:"media_paths"`
	TranscodePath string   `yaml:"transcode_path"`
}
type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Listen  string `yaml:"listen"`
}
type SecurityConfig struct {
	Token            string   `yaml:"token"`
	TokenFile        string   `yaml:"token_file"`
	AllowEnvironment bool     `yaml:"allow_environment"`
	AllowedPaths     []string `yaml:"allowed_paths"`
}

func Default() Config {
	return Config{Version: 1, Scheduler: SchedulerConfig{Listen: ":7000", Address: "127.0.0.1:7000", OfflineAfter: 20 * time.Second}, Worker: WorkerConfig{ID: "auto", Listen: ":7100", Advertise: "auto", AdvertiseVia: "tailscale", Scheduler: "127.0.0.1:7000", FFmpeg: "ffmpeg", MaxJobs: 1, Heartbeat: 5 * time.Second}, Storage: StorageConfig{MediaPaths: []string{"/media"}, TranscodePath: "/transcode"}, Metrics: MetricsConfig{Enabled: true, Listen: ":9090"}}
}
func Load(path string) (Config, error) {
	c := Default()
	if path != "" {
		b, e := os.ReadFile(path)
		if e != nil {
			return c, e
		}
		if e = yaml.Unmarshal(b, &c); e != nil {
			return c, e
		}
	}
	applyEnv(&c)
	if c.Security.Token == "" && c.Security.TokenFile != "" {
		b, e := os.ReadFile(c.Security.TokenFile)
		if e != nil {
			return c, e
		}
		c.Security.Token = string(bytesTrimSpace(b))
	}
	return c, Validate(c)
}
func Validate(c Config) error {
	if c.Version != 1 {
		return fmt.Errorf("unsupported config version %d", c.Version)
	}
	if c.Scheduler.Listen == "" || c.Worker.Listen == "" {
		return errors.New("listen address cannot be empty")
	}
	if c.Worker.MaxJobs < 1 {
		return errors.New("worker.max_jobs must be at least 1")
	}
	return nil
}
func applyEnv(c *Config) {
	set(&c.Scheduler.Listen, "JF_SCHEDULER_LISTEN")
	set(&c.Scheduler.Address, "JF_SCHEDULER_ADDR")
	set(&c.Worker.ID, "JF_WORKER_ID")
	set(&c.Worker.Listen, "JF_WORKER_LISTEN")
	set(&c.Worker.Advertise, "JF_WORKER_ADVERTISE")
	set(&c.Worker.Scheduler, "JF_SCHEDULER_ADDR")
	set(&c.Worker.FFmpeg, "JF_FFMPEG")
	set(&c.Metrics.Listen, "JF_METRICS_LISTEN")
	set(&c.Security.Token, "JF_CLUSTER_TOKEN")
	if v := os.Getenv("JF_WORKER_MAX_JOBS"); v != "" {
		if n, e := strconv.Atoi(v); e == nil {
			c.Worker.MaxJobs = n
		}
	}
}
func set(dst *string, key string) {
	if v := os.Getenv(key); v != "" {
		*dst = v
	}
}
func bytesTrimSpace(b []byte) []byte {
	start, end := 0, len(b)
	for start < end && (b[start] == ' ' || b[start] == '\n' || b[start] == '\r' || b[start] == '\t') {
		start++
	}
	for end > start && (b[end-1] == ' ' || b[end-1] == '\n' || b[end-1] == '\r' || b[end-1] == '\t') {
		end--
	}
	return b[start:end]
}
