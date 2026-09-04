package network

import (
	"context"
	"errors"
	"net"
	"os/exec"
	"strings"
	"time"
)

func TailscaleIPv4() (string, error) {
	ctx, c := context.WithTimeout(context.Background(), 3*time.Second)
	defer c()
	b, e := exec.CommandContext(ctx, "tailscale", "ip", "-4").Output()
	if e != nil {
		return "", e
	}
	ip := strings.TrimSpace(strings.Split(string(b), "\n")[0])
	if net.ParseIP(ip) == nil {
		return "", errors.New("tailscale returned no IPv4 address")
	}
	return ip, nil
}
func Advertise(configured, via, listen string) (string, error) {
	if configured != "" && configured != "auto" {
		return configured, nil
	}
	_, port, e := net.SplitHostPort(listen)
	if e != nil {
		return "", e
	}
	if via == "tailscale" {
		ip, e := TailscaleIPv4()
		if e != nil {
			return "", e
		}
		return net.JoinHostPort(ip, port), nil
	}
	return net.JoinHostPort("127.0.0.1", port), nil
}
