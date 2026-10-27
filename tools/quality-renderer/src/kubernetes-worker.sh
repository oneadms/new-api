#!/bin/sh
# new-api / QuantumNous — K3s worker startup; never accepts model commands.
set -eu
umask 077
ulimit -c 0
ulimit -n 1024

if [ "$(id -u)" != 1000 ] || [ "$(id -g)" != 1000 ]; then
  echo 'new-api renderer requires UID/GID 1000' >&2
  exit 1
fi
if [ -n "${QUALITY_RENDERER_TOKEN:-}" ]; then
  echo 'new-api renderer worker must not receive ingress credentials' >&2
  exit 1
fi

if [ "$#" -eq 0 ]; then
  set -- node /app/src/worker.mjs
fi
# Keep PID 1 in the original container namespaces for runtime exec probes.
# The child keeps UID 1000, but gets an empty network namespace. Chromium
# must still pass its independent namespace and Seccomp-BPF sandbox checks.
# No node-wide PID configuration or RLIMIT_NPROC is changed here.
export QUALITY_RENDERER_PARENT_NETNS="$(readlink /proc/self/ns/net)"
exec /usr/bin/tini -g -- /usr/bin/unshare --user --map-current-user --net -- "$@"
