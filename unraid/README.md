# Unraid template

The scheduler template uses the public multi-architecture image:

```text
ghcr.io/wate123/jf-dispatch:latest
```

Install the template on Unraid:

```bash
mkdir -p /boot/config/plugins/dockerMan/templates-user
curl -fsSL \
  https://raw.githubusercontent.com/wate123/jf-dispatch/main/unraid/jf-dispatch-scheduler.xml \
  -o /boot/config/plugins/dockerMan/templates-user/my-jf-dispatch-scheduler.xml
```

Open **Docker → Add Container**, select `jf-dispatch-scheduler` from the template list, then set:

- Cluster token: output of `openssl rand -hex 32`
- Scheduler address: the Unraid Tailscale name or IP plus `:7000`
- Media and transcode host paths

The template uses host networking so the scheduler can reach Tailscale workers directly. It exposes gRPC on TCP 7000 and health/metrics on TCP 9090.
