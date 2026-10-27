# new-api / QuantumNous — K3s renderer deployment

This is a single-node, private deployment of the optional HTML screenshot service.
It does not call models, enable detection schedules, publish samples, or migrate
application data. The existing Go client and `/render` protocol are unchanged.

## Topology and isolation

`deployment.yaml` creates a dedicated Pod with two containers:

- `ingress` has the dedicated Bearer token and exposes port 3000 through the
  ClusterIP Service only. It reads the worker's Unix socket through a read-only
  mount of a dedicated in-memory volume.
- `worker` receives no token, application environment, service-account token,
  application PVC, hostPath, or runtime socket. Its startup chain is
  `tini -> unshare --user --map-current-user --net -> node`.
  Node and Chromium remain UID/GID 1000 in a new, empty network namespace.
  Tini stays PID 1 in the original container namespaces, forwards signals and
  reaps children; worker termination ends the container's private PID namespace.
  Exec probes connect to the filesystem Unix socket and need no IP connection.

Both containers have read-only roots, no capabilities, no privilege escalation,
CPU/memory limits and bounded memory-backed temporary volumes. An unprivileged
init container creates UID-1000, mode-0700 private directories before subPath
mounts are attached. The worker socket remains mode 0600. The worker additionally
uses an architecture-specific OCI seccomp profile that denies IP sockets, legacy
`socketcall`, and `io_uring`. Chromium's actual namespace and Seccomp-BPF sandboxes
are checked on every browser launch. A namespace or sandbox failure stops startup.

The AppArmor profile is derived from containerd's default profile, with additional
execution and executable-mapping denials on `/tmp`, `/dev/shm`, and the socket
volume. Kubernetes `emptyDir` does not expose all Compose tmpfs mount flags; the
AppArmor denials are independently tested rather than claiming the volumes have
an identical `noexec` mount configuration. No extra mount capability is granted.
No-new-privileges, dropped capabilities and runtime device isolation remain in
force. Preserve the Apache-2.0 notices in `../THIRD_PARTY_NOTICES.md`.

`service-network.yaml` permits only same-namespace Pods labelled `app: new-api`
to connect to the renderer on TCP 3000, and denies renderer Pod outbound IP
traffic. It does not restrict the main application's existing upstream/database
connections. NetworkPolicy supplements, rather than replaces, worker isolation.
Both application and renderer must remain on the same node for this private HTTP
configuration; cross-node operation needs a separately configured TLS transport.
There is no Ingress, NodePort, LoadBalancer, hostPort or public endpoint.

## PID policy is an explicit operator decision

These manifests do not change kubelet `podPidsLimit`, containerd configuration,
node sysctls, or `RLIMIT_NPROC`, and do not restart K3s. Kubernetes has no standard
per-container equivalent of Compose `pids_limit`. CPU/memory limits are not PID
limits, and a shell `ulimit -u` is not a cgroup PID quota.

For the deployment accepted on 2026-10-27, the operator explicitly chose to leave
node PID settings unchanged. Kubelet still reported `podPidsLimit: -1`; the worker
cgroup reported an existing runtime `pids.max` of 9466. This deployment did not set
that runtime value, and does not claim the Compose worker limit of 256 applies.
Review the actual runtime limit before using these manifests on another node.

## Prepare the node and image

The target needs unprivileged user/network namespaces, supported AppArmor, and a
containerd runtime compatible with the OCI seccomp profile. The accepted target
was Debian 13, Linux 6.12, amd64, K3s v1.35.9+k3s1, containerd 2.2.7 and Calico 3.33.
Other nodes and arm64 still need their own live acceptance; conversion tests for
arm64 are not runtime acceptance.

1. Build the renderer from the matching source, including Tini/util-linux, the
   Kubernetes startup script and acceptance tests. Prefer a verified published
   digest. A Docker-local image is not automatically present in K3s/containerd.
   For an explicitly imported local image, retain the image archive/build inputs,
   register the digest reference in containerd, and use `imagePullPolicy: Never`
   as supplied. For registry distribution, explicitly change the pull policy and
   configure registry credentials if necessary; those are not the renderer token.
2. Generate an OCI profile for the **target node**, using Node 22+ on the operator
   machine (or the matching renderer image), for example:

   ```sh
   node kubernetes/generate-seccomp.mjs --arch amd64 --kernel 6.12.95+deb13-amd64 > worker-amd64.json
   ```

   The converter expands Docker `archMap`/`includes` conditions and rejects
   unsupported input instead of silently dropping conditions. Install the result
   as `<actual-kubelet-root>/seccomp/new-api-quality-renderer/worker-amd64.json`,
   owned by root and readable by the runtime. The accepted node uses
   `/var/lib/kubelet/seccomp/`. A ConfigMap mounted inside a container does not
   install a seccomp profile. The supplied manifest is amd64-specific; an arm64
   deployment must generate and reference its arm64 profile.
3. Validate and install `apparmor.profile` as the dedicated
   `/etc/apparmor.d/new-api-quality-renderer` profile and load it with
   `apparmor_parser -r`. It applies only to containers explicitly selecting it;
   do not replace `cri-containerd.apparmor.d` or change global AppArmor settings.
   Ensure it is loaded after a node reboot before the renderer starts.
4. Resolve `${QUALITY_RENDERER_IMAGE}` to an immutable image reference and
   `${QUALITY_RENDERER_NODE}` to the selected node in `deployment.yaml`. Preserve
   the production namespace, or adapt all resources and the application URL
   together. Keep generated deployment configuration in a protected operator
   directory and perform `kubectl apply --dry-run=server` before applying it.

## Accept before connecting the application

Convert the resolved Deployment to JSON with `kubectl create --dry-run=client`,
then pipe that JSON into `node kubernetes/create-acceptance-job.mjs`. This derives
the Job from the same worker image, node, security settings, initialization and
volumes instead of maintaining a second set of isolation settings. It removes
the ingress and health probes, explicitly enables browser tests, uses zero Job
retries, and labels the Job differently so it cannot become a production Service
endpoint. Worker credentials or `envFrom` make generation fail.

Apply `acceptance-network.yaml` before creating the generated Job. The Job has no
token and performs only operator-authored fixture renders. Require all ten tests
to pass with zero skips: sandboxed inline drawing, private Unix socket capture,
non-root/no-capability/NNP identity, execution denial on writable paths, and
IPv4/IPv6 TCP/UDP socket creation returning `EPERM`. A timeout or refused
connection is not proof of a seccomp socket denial.

If any check fails, retain the previous deployment and investigate. Do not remove
the worker network check, disable Chromium's sandbox, run privileged, grant host
capabilities, use an unconfined profile or silently fall back to the Pod network.

## Enable and connect

- Create a dedicated `production/quality-renderer-auth` Secret with a `token`
  key through a protected credential process. Never commit its value, print a
  rendered Secret, pass it as a command argument, or inject it into the worker.
- Apply `service-network.yaml` and the resolved Deployment, then wait for the
  complete Pod to become Ready. Ingress probes only check TCP readiness; worker
  socket readiness requires a successful startup sandbox/render check. Neither
  probe contacts a model or proves application credential matching.
- Verify service DNS and authenticated fixed-fixture rendering from the actual
  main application's network **before** changing its configuration. Also check
  an unrelated Pod can reach a control service but cannot reach the renderer.
- Back up the main application's current connection fields and record its actual
  running image digest. Recreating a Deployment using `latest` must not silently
  upgrade it. Preserve its PVC, working directory, envFrom, other environment
  variables, replicas, update strategy, probes and normal application network.
- Apply `new-api-env.patch.yaml` as a **strategic merge patch**, not a replacement
  Deployment. The base URL has no `/render` suffix. The token is a reference to
  the same Secret used by ingress. The accepted application's single-replica
  `Recreate` strategy causes a brief interruption; no K3s restart is needed.
- After rollout, validate HTTP 401 for missing/wrong credentials and HTTP 200 for
  the application's real injected token, a fully valid 1200×800 PNG within 2 MiB,
  and application `/api/status` health. Keep credentials out of arguments/logs.
  `renderer_available` alone is not this acceptance.

Changing a Secret does not update an existing process environment. Rotate the
credential deliberately and recreate ingress and application together. The
worker does not receive credentials or need a restart for credential rotation.
Existing samples are not automatically rerendered and no model run is started.

## Rollback and cleanup

Restore only the previous renderer environment fields and original running image
if connection acceptance fails. `rollout undo` does not restore Secret content or
pin a moving `latest`. Keep databases, quality metadata and application PVCs.
Do not change `QUALITY_ARTIFACT_DIR` as part of this deployment.

Once K3s and the application pass acceptance, remove only the temporary renderer
Docker project, its private bridge and its dedicated socket volume. Keep the
accepted image/build inputs and protected rollback records. Archive results and
remove completed/failed acceptance Jobs; they also have a TTL. Remove node
profiles only after all containers referring to them have stopped.

## Accepted deployment record — 2026-10-27

- Renderer: `production/quality-renderer`, two Ready containers, private Service.
- Node: `k3s-152-53-167-188`; no node PID-policy change or K3s restart.
- Deployment-local renderer image:
  `docker.io/library/new-api-quality-renderer@sha256:980ad4e3227a0a75390f303203799dae23f0c6ae543e65d7a3e90b183ed54fbd`.
  This is a locally built/imported adaptation, not a newly published GHCR release.
  Its published base digest was verified against the repository workflow's Cosign
  identity; that signature is not a signature for the locally modified image.
- Main application image content preserved at
  `ghcr.io/oneadms/new-api@sha256:b58124a5179b55084a4ba8e155f13d6f22d2e06302856f5ee5ff052d36437e7f`.
- Native K3s browser/isolation tests: 10 passed, 0 skipped. The actual application
  network and injected token returned HTTP 200 and a checksum-valid 1200×800 PNG;
  missing/wrong tokens returned 401. Unapproved Pod access was blocked while its
  control request to the application succeeded.
- Operator records, resolved manifests, image reference and rollback patch are
  under `/opt/new-api-quality-renderer/kubernetes/` on the server. No token is
  included in repository records. Temporary Docker renderer containers, network
  and socket volume were removed after acceptance. Other Deployments remained
  available. No upstream model request or schedule change was made by acceptance.
