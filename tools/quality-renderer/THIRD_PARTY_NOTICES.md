# Third-party notices — new-api / QuantumNous quality renderer

The component itself remains subject to the new-api repository's AGPLv3 license,
additional attribution terms, and QuantumNous attribution. This file does not
replace or remove them. Original project: https://github.com/QuantumNous/new-api

Repository license: https://github.com/QuantumNous/new-api/blob/main/LICENSE
Attribution terms: https://github.com/QuantumNous/new-api#-license

## Playwright and playwright-core

- Pinned npm versions: **1.64.1**, Apache License 2.0.
- Copyright (c) Microsoft Corporation.
- Source: https://github.com/microsoft/playwright/tree/v1.64.1
- License: https://github.com/microsoft/playwright/blob/v1.64.1/LICENSE
- Full license and upstream NOTICE files are preserved by `npm ci` inside the
  installed `playwright` / `playwright-core` packages. The Docker image retains
  these package files. The local full Apache-2.0 text is `LICENSE.apache-2.0`.
- Upstream notice: “Playwright. Copyright (c) Microsoft Corporation. This software
  contains code derived from the Puppeteer project
  (https://github.com/puppeteer/puppeteer), available under the Apache 2.0 license
  (https://github.com/puppeteer/puppeteer/blob/master/LICENSE).”

`seccomp_profile.json` is derived from the Playwright Docker profile at
https://github.com/microsoft/playwright/blob/v1.64.1/utils/docker/seccomp_profile.json
(Apache-2.0; based on Docker/Moby's default profile). Modifications for this
component: retain x86/ARM architectures used by the image; omit capability-gated
rules because all host capabilities are dropped; deny legacy `socketcall` and
`io_uring`; restrict `socket` to Unix/netlink address families; and return ENOSYS
for `clone3` so modern glibc uses the allowed `clone` fallback. Namespace creation
for the real Chromium sandbox remains allowed. No unconfined or privileged
fallback is included. Copyright for these new-api modifications remains with
QuantumNous contributors under the project license, subject to upstream terms.

## Browser, base image, fonts and Node.js

- Container: `mcr.microsoft.com/playwright:v1.64.1-noble`.
- Chromium is distributed under its BSD-style license and bundled third-party
  notices, not exclusively under Playwright's Apache license. Preserve the
  browser's bundled credits, license files and notices.
- Ubuntu base packages have their respective licenses and notices under
  `/usr/share/doc/*/copyright`. Node.js includes MIT and bundled dependency notices.
- Added `fonts-noto-core`, `fonts-noto-cjk`, and `fonts-noto-mono` packages include
  SIL Open Font License 1.1 notices under their package copyright directories.

Image distributions must retain these upstream materials. Browser/OS security
updates require coordinated dependency/image/capture-version review; no statement
here implies an exhaustive review or perpetual safety of a pinned release.
