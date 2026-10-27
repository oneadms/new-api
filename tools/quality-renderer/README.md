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

## Publish, configure, and verify

### Prebuilt images are a separate deliverable

The main new-api image does **not** include or start this renderer. Publishing or
updating that image cannot enable screenshots on its own. Text and extracted
HTML remain available without the renderer; screenshots require both isolated
containers and the application connection described below.

The separate GitHub Actions workflow
[quality-renderer-image.yml](../../.github/workflows/quality-renderer-image.yml)
publishes `ghcr.io/<owner>/<repository>-quality-renderer`, with the owner and
repository normalized to lowercase. For this repository that is
`ghcr.io/oneadms/new-api-quality-renderer`. It runs when renderer files or the
workflow change on `main`, and can also be dispatched manually. It:

- Runs the pinned Node dependency install, syntax checks and renderer contract
  tests before publication; these tests mock the browser boundary.
- Builds `linux/amd64` and `linux/arm64` on native runners, with provenance and
  software-bill-of-materials attestations. No QEMU browser acceptance is implied.
- Signs each image digest and the combined manifest with keyless Cosign, using
  GitHub's identity provider and job-scoped package/identity permissions.
- Publishes `sha-<full-40-character-commit>` and architecture-specific
  `sha-<commit>-amd64` / `sha-<commit>-arm64` tags. Only a run still at the current
  `main` commit updates `latest`; dispatching a different ref does not update it.

Wait for **this renderer workflow** to complete successfully before pulling the
image. A workflow file, an application image, or a registry tag alone is not proof
that the renderer is running or that its sandbox works on your host. The workflow
never deploys services or calls a model. GHCR packages may initially be private;
operators need package read permission and `docker login ghcr.io` unless package
visibility has explicitly been made public.

For production, use the manifest's `@sha256:...` reference from the workflow
summary rather than the moving `latest` tag. A commit tag identifies source;
a registry digest pins the exact image, including its OS/font packages. Verify
its Cosign signature against the publishing repository's workflow identity and
`https://token.actions.githubusercontent.com` issuer, not an arbitrary signer.
Rebuild and re-validate intentionally when applying browser or OS security fixes.

### Prebuilt deployment on the target Linux host

These are operator steps, not actions performed by publication. Use Docker
Compose v2.24.0 or newer (`!reset` support is required). Obtain `compose.yaml`,
`compose.image.yaml`, `compose.application.yaml`, and `seccomp_profile.json` from
**the same source commit as the image**, keeping them together. The image-only
override removes both build contexts and retains every isolation/resource setting
from `compose.yaml`; no source checkout or local browser installation is needed
for the renderer containers themselves.

1. Set `QUALITY_RENDERER_IMAGE` to the verified published image digest (or the
   matching commit tag for evaluation). Store a dedicated `QUALITY_RENDERER_TOKEN`
   in protected deployment configuration, not source control or frontend settings.
   For example, `openssl rand -hex 32` generates a suitable value. Inject **the same
   value** into the main application and the renderer ingress; do not give the
   worker that variable or the main application's full environment. The shell
   examples below assume both variables are already set and deliberately do not
   print their values.
2. From this directory, validate and pull only the prebuilt images:

   ```sh
   docker compose -f compose.yaml -f compose.image.yaml config --quiet
   docker compose -f compose.yaml -f compose.image.yaml pull
   ```

   Use `config --quiet`, not plain `config` in shared logs: a rendered Compose
   configuration contains the ingress token. If pulling fails, check the image
   name, completed publication and GHCR permissions; the image-only override
   intentionally does not silently build a different local image.
3. Before enabling thumbnails, run the real-browser acceptance on the **target
   Linux runtime** with the worker's unchanged no-network/seccomp settings:

   ```sh
   docker compose -f compose.yaml -f compose.image.yaml run --rm --no-deps --pull never \
     -e QUALITY_RENDERER_INTEGRATION=1 quality-renderer-worker npm run test:browser
   ```

   This checks actual Chromium sandbox status, inline drawing/capture and the OS
   prohibition on IP socket creation. All browser integration tests must execute
   and pass, not skip. A denied user namespace, sandbox startup failure or socket
   policy failure is a deployment blocker; never work around it by disabling the
   sandbox, adding privileges or giving the worker a network.
4. After that acceptance succeeds, start the two containers and inspect health:

   ```sh
   docker compose -f compose.yaml -f compose.image.yaml up -d --no-build --wait
   docker compose -f compose.yaml -f compose.image.yaml ps
   ```

   The worker health check becomes ready only after its startup sandbox check;
   it does not perform a screenshot, validate the main application's token, or
   prove main-application connectivity.

### Connect the main application

Start the renderer project first so `new-api-quality-renderer-ingress` exists.
For the repository's application `docker-compose.yml`, the supplied
`compose.application.yaml` adds **only** the application to that external network
while retaining its existing `new-api-network` connection and normal upstream
access. From the repository root, the operator can apply it with:

```sh
docker compose -f docker-compose.yml -f tools/quality-renderer/compose.application.yaml config --quiet
docker compose -f docker-compose.yml -f tools/quality-renderer/compose.application.yaml up -d --no-deps new-api
```

Use the same exported/protected `QUALITY_RENDERER_TOKEN` value used for renderer
startup. If your main application uses a different Compose path or service name,
adapt this **application-only override** to that service, preserving all existing
networks; do not merge the entire renderer project into the application project.
For another deployment manager, add the equivalent external network attachment
and server environment directly:

- `QUALITY_RENDERER_URL=http://quality-renderer:3000` — a base URL without
  `/render`, not the public site URL, `localhost`, or a browser-facing address.
  The Go client appends `/render` itself.
- `QUALITY_RENDERER_TOKEN` — the same dedicated token as the ingress container.
  The worker must still have no token and `network_mode: none`.

Recreate affected application/ingress containers when environment values change;
a container restart alone does not inject new environment configuration. Keep the
worker's environment, ports, mounts and network unchanged. Only the main
application and ingress belong on this internal network. Never publish renderer
port 3000 to the host to work around a missing network attachment.

Finally, from the application-facing private network, perform an authenticated
`POST http://quality-renderer:3000/render` with a fixed, operator-authored HTML
fixture (not a model request). Require HTTP 200, `image/png`, a valid 1200×800 PNG
within 2 MiB, and confirm the real application's network can reach the ingress.
Check an unauthenticated request is rejected. Keep the token in the environment
and out of command arguments/logs. These checks establish connectivity and
capture; the settings banner's `renderer_available` only checks whether the URL
and token are syntactically valid. It does not contact the service, authenticate,
or prove worker health. Enabling the service does not automatically rerender old
samples or rerun their upstream model requests.

### Local builds and contract checks

The unchanged local-build image identifier remains
`new-api-quality-renderer:1.0.0-pw1.64.1` in `compose.yaml`. To build it from a
checkout, set the dedicated ingress token and use only the base Compose file:

```sh
docker compose -f compose.yaml build
docker compose -f compose.yaml run --rm --no-deps --pull never \
  -e QUALITY_RENDERER_INTEGRATION=1 quality-renderer-worker npm run test:browser
docker compose -f compose.yaml up -d --no-build --wait
```

Use the standalone Node package, independently of the web frontend. Its
`package-lock.json` pins Playwright 1.64.1 and playwright-core 1.64.1. The Docker
base is `mcr.microsoft.com/playwright:v1.64.1-noble`; browsers are not downloaded
at runtime. No user script or model code is ever passed to a shell.

Local contract checks are `npm ci --ignore-scripts`, `npm run check`, and
`npm test`. Tests use real HTTP parsing and a mocked Playwright boundary to check
authentication, validation, concurrency, output limits, CSP/routes, runtime
sandbox rejection, environment isolation, cancellation and cleanup. No model or
main application service is contacted. `npm run test:browser` without the explicit
integration flag reports skips; it must not be recorded as a live-browser pass.
From the repository root run
`go test ./service -run '^TestQualityRenderer' -count=1` for the Go client.

The current implementation environment had Node 24.18.0 and Go 1.27.0; the Docker
CLI was present but its Linux engine was unavailable. The pinned Playwright base
manifest was confirmed to contain both Linux architectures and its pinned npm
packages installed successfully. Image build, real Chromium sandbox/network
enforcement, fonts and live screenshot acceptance still require the target-runtime
checks above. A parsed Compose file, contract test, base-image registry lookup or
successful image publication is **not** proof of that acceptance. Go race
detection was also blocked by the environment's disabled CGO; regular renderer
regression tests and service vet checks can run independently. Do not enable an
unsafe fallback to hide a failed deployment check.

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
