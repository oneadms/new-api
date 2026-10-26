# new-api / QuantumNous — optional quality renderer

This component accepts bounded HTML and returns a static PNG. The Go application
never installs or launches a browser, invokes model-generated shell code, or uses
ordinary relay/billing paths to render a thumbnail. The component is optional.

## Application contract

Configure only the main application's trusted server environment:

- `QUALITY_RENDERER_URL=http://quality-renderer:3000` is the renderer **base URL**.
  The Go client appends `/render`. Optional reverse-proxy path prefixes are
  supported. The URL must never come from a request, model output, or public setting.
- `QUALITY_RENDERER_TOKEN` is a dedicated, nonempty printable ASCII token, ideally
  at least 32 random bytes encoded as hex/base64. The same token goes to the
  renderer **ingress container only**, never its browser worker.

`service.QualityRendererAvailable()` checks configuration, not network health.
`service.RenderQualityHTML(ctx, html)` returns PNG bytes or an error. Missing or
invalid deployment configuration satisfies
`errors.Is(err, service.ErrQualityRendererUnavailable)`. The caller must preserve
successful model generation/output and record a separate unavailable/failed
**render** status. A missing thumbnail is not a failed upstream generation.
There is no local-browser or URL-rendering fallback and no retry.

The wire contract is authenticated `POST /render`,
`Content-Type: application/json`, `Authorization: Bearer <token>`, with exactly
`{"html":"..."}`. Additional keys, URLs, viewport options, compressed requests,
empty strings, invalid Unicode, and unsupported methods are rejected. The only
successful response is `image/png`, 1200×800, at most 2 MiB. All errors are small
static JSON codes; HTML, browser errors, paths, and credentials are never returned.
The service has no public HTML endpoint, proxy, WebSocket endpoint, or `CONNECT`
handler. Both ingress and worker reject overlapping renders with HTTP 429 rather
than creating an unbounded queue.

Limits are 1 MiB of UTF-8 HTML, a maximum `6 × 1 MiB + 64` JSON envelope (to allow
Go JSON escaping), 5 seconds for body upload, 15 seconds for browser work, 2 more
seconds for forced worker-container termination if browser cleanup hangs, and an
18-second ingress deadline. The Go client uses a 20-second context, honors earlier
caller cancellation, disables redirects/environment proxies/compression, and
bounds streamed responses even without `Content-Length`. It validates the full
PNG stream and checksums after checking dimensions, not only the PNG signature.
The Node boundary checks output size, signature, IHDR dimensions and IEND.

## Mandatory production isolation and topology

The supplied Compose deployment deliberately has **two containers**, separating
the network/authentication surface from Chromium:

1. The main new-api container joins `new-api-quality-renderer-ingress` in addition
   to its existing application network. It calls `http://quality-renderer:3000`.
2. `quality-renderer` is the authenticated Node ingress. It joins only that
   `internal: true` network. No ports are published on the host. It talks to one
   fixed Unix-domain socket, not a configurable worker URL.
3. `quality-renderer-worker` has `network_mode: none`, no TCP listener, no token,
   and no access to the application's network. It receives HTML through
   `/run/quality-renderer/worker.sock`. The socket is owned by UID 1000 with mode
   0600 and lives in a dedicated **Docker-managed named volume**, not a host bind
   mount, database/artifact volume, credential mount, or Docker socket. The ingress
   mounts this directory read-only; the worker owns the socket. No other service
   should mount this volume.

An `internal` network alone does **not** prevent reaching peers on that network.
That is why the browser is in a different container with no network at all; simply
placing Chromium next to the main application on an internal bridge is not an
acceptable substitute. In addition, the worker seccomp profile denies IPv4/IPv6
socket creation (including loopback), legacy `socketcall`, and `io_uring`; only
Unix and netlink sockets are permitted. No reverse proxy or egress gateway runs
in the worker. Application/browser request routing is not the sole network barrier.

To connect an already running main application container, the operator can use
`docker network connect new-api-quality-renderer-ingress <new-api-container>`
after this deployment creates the network. For declarative application Compose,
add that network as an **external** network and attach only the application
service. Keep its existing application network for normal upstream access. A host
process cannot reach this service by default; use a containerized application or
a separately reviewed private ingress setup with TLS and appropriate firewall
rules rather than publishing an unauthenticated port. HTTP here is confined to
the dedicated local container network; cross-host transport requires TLS.

Both containers are non-root UID/GID 1000, have read-only roots, no host bind
mounts, `cap_drop: ALL`, `no-new-privileges`, no core dumps, and bounded temporary
filesystems. The ingress has 0.5 CPU / 256 MiB / 64 PIDs; the worker has 1 CPU /
1 GiB / 128 PIDs, 128 MiB private shared memory and a 256 MiB `/tmp` tmpfs.
No host IPC or privileged capability is requested. The temporary Playwright
profile is created only inside this tmpfs and removed when each browser exits.
Each request gets a new browser, new context, and new page; no session, cookies,
profile or page is reused across requests. After a hard timeout the container
exits so PID-namespace teardown kills remaining descendants. The worker is not
suitable for running as an uncontained host process.

At startup the worker refuses root, non-Linux systems, and any non-loopback
network interface. Before serving, and on every subsequent browser launch, it
reads Chromium's actual `chrome://sandbox/` status and requires both the
namespace and Seccomp-BPF sandboxes. Chromium is explicitly launched with
`chromiumSandbox: true`; `--no-sandbox`, `--disable-setuid-sandbox`, privileged
containers and unconfined seccomp are not fallbacks. Linux must support
unprivileged user namespaces under its configured AppArmor/SELinux/seccomp
policies. If a host policy or runtime blocks this, startup fails and rendering
remains unavailable; correct the confined deployment policy rather than weaken
the browser. Compose health checks only report readiness after this startup
verification, without submitting model data or consuming a render slot.

Chromium's process environment is a fixed allowlist (`PATH`, `HOME`, `TMPDIR`,
`LANG`, `LC_ALL`, `TZ`). It never inherits the gateway token, database secrets,
proxy environment, `NODE_OPTIONS`, or loader variables. Avoid passing the main
application's `.env`, secrets, or volumes to either container. The browser still
needs its image's executable/libraries/fonts; this is not a claim that its OS
process cannot read those files. Untrusted documents cannot fetch filesystem
resources: the opaque-origin CSP sandbox, resource policies, and routes deny
file/external resources. Chromium sandboxing and the isolated mount namespace
are the additional protection against a browser compromise.

## Document policy and capture version

The capture identifier is `new-api-quality-v1-pw1.64.1`, returned in
`X-Quality-Renderer-Version`. Keep this identifier, the exact npm dependency, and
the Playwright image version in sync when changing browser/capture behavior.
Pin the built image by digest in production to keep OS/font package versions
stable; rebuild and version captures intentionally when applying security updates.

The browser receives one fixed synthetic document address,
`http://quality-render.invalid/document`. Playwright fulfills that **one initial
main-frame GET entirely from memory** with the supplied HTML and an HTTP CSP
header; it is never resolved or fetched. No request can choose this URL or any
other destination. Every subsequent request is aborted, including another
navigation to that address. The context is also offline, service workers are
blocked, all WebSocket routes are closed, downloads are canceled, dialogs are
dismissed, and additional pages are closed.

The response-header CSP allows only inline JavaScript/styles, data images/fonts,
and an opaque-origin `sandbox allow-scripts`. It prohibits connections, frames,
objects, workers, media, manifests, forms, base URLs, same-origin privileges,
external scripts/styles, eval, popups and downloads. Model HTML cannot remove or
relax a response-header policy. No browser bindings expose Node capabilities.
Only screenshot bytes leave the browser worker; raw HTML is never served to users.

Capture policy v1 is:

- Fixed 1200×800 CSS-pixel viewport, scale 1, light color scheme, reduced motion,
  `en-US` locale and `UTC` timezone. Installed Noto Sans / Serif / Sans Mono and
  Noto CJK fonts provide fixed defaults and local glyph fallback.
- Playwright's virtual clock starts at `2026-01-01T00:00:00Z`, pauses before the
  document loads, then advances 1000 ms after `DOMContentLoaded` and local fonts
  are ready. There is no network-idle wait and no second model call.
- One top-left viewport PNG, not a full-page scroll capture. The caret is hidden;
  Playwright fast-forwards finite CSS animations and cancels infinite ones for
  capture (`animations: disabled`). No interaction replay or HTML export occurs.

This fixes the rendering environment and sampling rule, not all model behavior:
randomness and model scripts can still affect pixels, and a screenshot cannot
prove that an animation or interaction works correctly.

## Build, verify, and operate

Use the standalone Node package, independently of the web frontend. Its
`package-lock.json` pins Playwright 1.64.1 and playwright-core 1.64.1. The Docker
base is `mcr.microsoft.com/playwright:v1.64.1-noble`; browsers are not downloaded
at runtime. No user script or model code is ever passed to a shell.

Set `QUALITY_RENDERER_TOKEN` in your shell or private Compose environment (never
commit it). Then, from this directory, run `docker compose build`. Before enabling
thumbnails, run `docker compose run --rm --no-deps -e QUALITY_RENDERER_INTEGRATION=1
quality-renderer-worker npm run test:browser` on the target Linux runtime. This
checks the real Chromium sandbox, drawing/capture and the OS IP-socket prohibition.
Then run `docker compose up -d` and check worker health and an authenticated test
render through the application-facing network. A successful unit test or parsed
Compose file is **not** proof of deployment isolation or sandbox compatibility.

Local contract checks are `npm ci --ignore-scripts`, `npm run check`, and
`npm test`. Tests use real HTTP parsing and a mocked Playwright boundary to check
authentication, validation, concurrency, output limits, CSP/routes, runtime
sandbox rejection, environment isolation, cancellation and cleanup. No model or
main application service is contacted. `npm run test:browser` without the explicit
integration flag reports skips; it must not be recorded as a live-browser pass.
From the repository root run
`go test ./service -run '^TestQualityRenderer' -count=1` for the Go client.

The current implementation environment had Node 24.18.0 and Go 1.27.0; the Docker
CLI was present but its Linux engine was unavailable. Consequently image build,
real Chromium sandbox/network enforcement, fonts and live screenshot acceptance
still require the target-runtime checks above. Go race detection was also blocked
by the environment's disabled CGO; regular renderer regression tests and service
vet checks can run independently. Do not enable an unsafe fallback to hide a
failed deployment check.

## Licenses and attribution

This optional component remains part of new-api by QuantumNous under the
repository's AGPLv3 license and additional attribution terms. Playwright and playwright-core are maintained
Microsoft packages under Apache-2.0; there is no copyleft exception or additional
commercial browser-automation service requirement. Apache-2.0 dependencies are
compatible with this AGPLv3 project when their notices/license terms are retained.
Chromium, the Playwright container's OS packages and Noto fonts carry additional
licenses. Preserve upstream license files in distributed images; see
`THIRD_PARTY_NOTICES.md`. This assessment is about declared licenses, not a legal
warranty or a security audit of every transitive OS/browser component.
