#!/usr/bin/env bash
set -Eeuo pipefail

IMAGE="ghcr.io/wate123/jf-dispatch:latest"
CONTAINER="jf-dispatch-worker"
MEDIA_MOUNT="/mnt/jf-media"
TRANSCODE_MOUNT="/mnt/jf-transcode"
MAX_JOBS="2"
METRICS_LISTEN="0.0.0.0:19091"
NFS_VERSION="3"
GPU="none"

usage() {
  cat <<'EOF'
Usage: sudo ./install-docker-worker.sh [options]

Required:
  --id NAME                 Worker ID, for example x86-server
  --advertise HOST:PORT     Address reachable by the scheduler
  --scheduler HOST:PORT     Scheduler address
  --nfs-server ADDRESS      Unraid LAN address

Options:
  --media-export PATH       Default: /mnt/user/Media
  --transcode-export PATH   Default: /mnt/user/transcode
  --media-mount PATH        Default: /mnt/jf-media
  --transcode-mount PATH    Default: /mnt/jf-transcode
  --max-jobs NUMBER         Default: 2
  --gpu none|intel|nvidia   Default: none
  --image IMAGE             Override container image
  --nfs-version 3|4         Default: 3
EOF
}

die() { echo "error: $*" >&2; exit 1; }
require_value() { [[ $# -ge 2 && -n "$2" ]] || die "missing value for $1"; }

WORKER_ID=""
ADVERTISE=""
SCHEDULER=""
NFS_SERVER=""
MEDIA_EXPORT="/mnt/user/Media"
TRANSCODE_EXPORT="/mnt/user/transcode"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --id) require_value "$@"; WORKER_ID=$2; shift 2 ;;
    --advertise) require_value "$@"; ADVERTISE=$2; shift 2 ;;
    --scheduler) require_value "$@"; SCHEDULER=$2; shift 2 ;;
    --nfs-server) require_value "$@"; NFS_SERVER=$2; shift 2 ;;
    --media-export) require_value "$@"; MEDIA_EXPORT=$2; shift 2 ;;
    --transcode-export) require_value "$@"; TRANSCODE_EXPORT=$2; shift 2 ;;
    --media-mount) require_value "$@"; MEDIA_MOUNT=$2; shift 2 ;;
    --transcode-mount) require_value "$@"; TRANSCODE_MOUNT=$2; shift 2 ;;
    --max-jobs) require_value "$@"; MAX_JOBS=$2; shift 2 ;;
    --gpu) require_value "$@"; GPU=$2; shift 2 ;;
    --image) require_value "$@"; IMAGE=$2; shift 2 ;;
    --nfs-version) require_value "$@"; NFS_VERSION=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown option: $1" ;;
  esac
done

[[ $EUID -eq 0 ]] || die "run with sudo"
[[ -n $WORKER_ID ]] || die "--id is required"
[[ -n $ADVERTISE ]] || die "--advertise is required"
[[ -n $SCHEDULER ]] || die "--scheduler is required"
[[ -n $NFS_SERVER ]] || die "--nfs-server is required"
if [[ -z ${JF_CLUSTER_TOKEN:-} && -r /dev/tty ]]; then
  read -r -s -p "Cluster token: " JF_CLUSTER_TOKEN </dev/tty
  echo >/dev/tty
fi
[[ ${JF_CLUSTER_TOKEN:-} != "" && ${JF_CLUSTER_TOKEN:-} != "change-me" ]] || \
  die "enter the scheduler token or set JF_CLUSTER_TOKEN"
[[ $MAX_JOBS =~ ^[1-9][0-9]*$ ]] || die "--max-jobs must be a positive integer"
[[ $NFS_VERSION == 3 || $NFS_VERSION == 4 ]] || die "--nfs-version must be 3 or 4"
[[ $GPU == none || $GPU == intel || $GPU == nvidia ]] || die "--gpu must be none, intel, or nvidia"

command -v docker >/dev/null || die "Docker is required"
if ! command -v mount.nfs >/dev/null; then
  if command -v apt-get >/dev/null; then
    apt-get update
    DEBIAN_FRONTEND=noninteractive apt-get install -y nfs-common
  elif command -v dnf >/dev/null; then
    dnf install -y nfs-utils
  elif command -v yum >/dev/null; then
    yum install -y nfs-utils
  else
    die "install the NFS client package first"
  fi
fi

mkdir -p "$MEDIA_MOUNT" "$TRANSCODE_MOUNT" /etc/jf-dispatch
FSTAB=/etc/fstab
MEDIA_SPEC="$NFS_SERVER:$MEDIA_EXPORT $MEDIA_MOUNT nfs ro,nfsvers=$NFS_VERSION,_netdev,nofail,x-systemd.automount 0 0"
TRANSCODE_SPEC="$NFS_SERVER:$TRANSCODE_EXPORT $TRANSCODE_MOUNT nfs rw,nfsvers=$NFS_VERSION,_netdev,nofail,x-systemd.automount 0 0"

append_fstab() {
  local spec=$1 mountpoint=$2 tmp
  tmp=$(mktemp)
  awk -v mp="$mountpoint" '$2 != mp { print }' "$FSTAB" > "$tmp"
  printf '%s\n' "$spec" >> "$tmp"
  install -m 0644 "$tmp" "$FSTAB"
  rm -f "$tmp"
}

append_fstab "$MEDIA_SPEC" "$MEDIA_MOUNT"
append_fstab "$TRANSCODE_SPEC" "$TRANSCODE_MOUNT"
mount "$MEDIA_MOUNT" 2>/dev/null || mountpoint -q "$MEDIA_MOUNT" || die "cannot mount media export"
mount "$TRANSCODE_MOUNT" 2>/dev/null || mountpoint -q "$TRANSCODE_MOUNT" || die "cannot mount transcode export"
touch "$TRANSCODE_MOUNT/.jf-dispatch-write-test" || die "transcode export is not writable"
rm -f "$TRANSCODE_MOUNT/.jf-dispatch-write-test"

umask 077
printf '%s\n' "$JF_CLUSTER_TOKEN" > /etc/jf-dispatch/cluster-token
cat > /etc/jf-dispatch/worker.yaml <<EOF
version: 1
worker:
  id: "$WORKER_ID"
  listen: "0.0.0.0:7100"
  advertise: "$ADVERTISE"
  scheduler: "$SCHEDULER"
  ffmpeg: "/usr/bin/ffmpeg"
  max_jobs: $MAX_JOBS
  preferred_accelerators: ["cpu"]
storage:
  media_paths: ["/media"]
  transcode_path: "/transcode"
metrics:
  enabled: true
  listen: "$METRICS_LISTEN"
security:
  token_file: "/etc/jf-dispatch/cluster-token"
  allowed_paths: ["/media", "/transcode"]
EOF
chmod 0600 /etc/jf-dispatch/cluster-token
chmod 0644 /etc/jf-dispatch/worker.yaml

docker_args=(
  run -d --name "$CONTAINER" --restart unless-stopped --network host
  -v "$MEDIA_MOUNT:/media:ro"
  -v "$TRANSCODE_MOUNT:/transcode"
  -v /etc/jf-dispatch:/etc/jf-dispatch:ro
)
case "$GPU" in
  intel)
    [[ -d /dev/dri ]] || die "--gpu intel requested but /dev/dri is absent"
    docker_args+=(--device /dev/dri:/dev/dri)
    ;;
  nvidia)
    command -v nvidia-smi >/dev/null || die "--gpu nvidia requested but nvidia-smi is absent"
    docker_args+=(--gpus all)
    ;;
esac
docker_args+=("$IMAGE" worker -config /etc/jf-dispatch/worker.yaml)

docker pull "$IMAGE"
if docker container inspect "$CONTAINER" >/dev/null 2>&1; then
  docker rm -f "$CONTAINER" >/dev/null
fi
docker "${docker_args[@]}" >/dev/null
sleep 2
docker container inspect -f '{{.State.Status}}' "$CONTAINER"
docker logs --tail 30 "$CONTAINER"
echo "worker $WORKER_ID installed; verify it on the scheduler with: jf-dispatch nodes"
