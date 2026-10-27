import { crc32, deflateSync } from 'node:zlib';

// Minimal valid grayscale PNG, independent of the renderer's validation logic.
export function pngFixture(width = 1200, height = 800) {
  const chunk = (name, data) => {
    const type = Buffer.from(name);
    const result = Buffer.alloc(data.length + 12);
    result.writeUInt32BE(data.length, 0);
    type.copy(result, 4);
    data.copy(result, 8);
    result.writeUInt32BE(crc32(Buffer.concat([type, data])), result.length - 4);
    return result;
  };
  const header = Buffer.alloc(13);
  header.writeUInt32BE(width, 0);
  header.writeUInt32BE(height, 4);
  header[8] = 8;
  return Buffer.concat([
    Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
    chunk('IHDR', header), chunk('IDAT', deflateSync(Buffer.alloc((width + 1) * height))),
    chunk('IEND', Buffer.alloc(0)),
  ]);
}

export function deferred() {
  let resolve;
  let reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}

// Observed chrome://sandbox/ body from the pinned Playwright 1.64.1 Chromium.
export const SANDBOX_STATUS = `Sandbox Status
Layer 1 Sandbox\tNamespace
PID namespaces\tYes
Network namespaces\tYes
Seccomp-BPF sandbox\tYes
Seccomp-BPF sandbox supports TSYNC\tYes
Ptrace Protection with Yama LSM (Broker)\tNo
Ptrace Protection with Yama LSM (Non-broker)\tNo

You are adequately sandboxed.`;

// Only the external Playwright boundary is replaced. HTTP parsing, limits,
// policy construction, routing, sandbox checks, and cleanup remain production code.
export function browserFixture(options = {}) {
  const state = { launches: [], contexts: [], captures: [], documents: [], closes: 0, sockets: [] };
  const chromium = {
    async launch(launchOptions) {
      state.launches.push(launchOptions);
      return {
        async newContext(contextOptions) {
          const context = { options: contextOptions, events: {}, routes: [], wsRoutes: [], closed: false };
          state.contexts.push(context);
          const page = {
            events: {},
            on(name, handler) { this.events[name] = handler; },
            mainFrame() { return page; },
            locator() { return { async innerText() { return options.sandbox ?? SANDBOX_STATUS; } }; },
            async goto(url) {
              if (url === 'chrome://sandbox/') return;
              const route = {
                request() { return { url: () => url, method: () => 'GET', isNavigationRequest: () => true, frame: () => page }; },
                async fulfill(document) { state.documents.push(document); },
                async abort() { throw new Error('Test document was blocked'); },
              };
              await context.routes[0].handler(route);
            },
            clock: {
              async install(value) { state.clock = value; },
              async pauseAt(value) { state.pausedAt = value; },
              async runFor(value) { state.sampleTime = value; },
            },
            async evaluate() {},
            async screenshot(capture) {
              state.captures.push(capture);
              if (options.screenshot) return options.screenshot();
              return options.png ?? pngFixture();
            },
            async close() {},
          };
          context.page = page;
          return Object.assign(context, {
            async newPage() { return page; },
            setDefaultTimeout() {},
            setDefaultNavigationTimeout() {},
            on(name, handler) { this.events[name] = handler; },
            async route(pattern, handler) { this.routes.push({ pattern, handler }); },
            async routeWebSocket(pattern, handler) { this.wsRoutes.push({ pattern, handler }); },
            async newCDPSession() { return { async send(method, params) { state.fonts = { method, params }; } }; },
            async close() { this.closed = true; },
          });
        },
        async close() {
          state.closes += 1;
          if (options.close) await options.close();
        },
      };
    },
  };
  return { state, chromium };
}
