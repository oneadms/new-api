import assert from 'node:assert/strict';
import test from 'node:test';
import { assertWorkerIsolation, DOCUMENT_URL } from '../src/policy.mjs';
import { QualityRenderer } from '../src/renderer.mjs';
import { browserFixture, deferred, pngFixture, SANDBOX_STATUS } from './fixtures.mjs';

test('browser launches sandboxed with an explicit credential-free environment', async (t) => {
  const previous = process.env.QUALITY_RENDERER_TOKEN;
  process.env.QUALITY_RENDERER_TOKEN = 'gateway-only-secret';
  t.after(() => { if (previous === undefined) delete process.env.QUALITY_RENDERER_TOKEN; else process.env.QUALITY_RENDERER_TOKEN = previous; });
  const fixture = browserFixture();
  await new QualityRenderer(fixture.chromium).render('<p>quality</p>');
  const options = fixture.state.launches[0];
  assert.equal(options.chromiumSandbox, true);
  assert.ok(!options.args.some((arg) => /no-sandbox|disable.*sandbox/.test(arg)));
  assert.deepEqual(Object.keys(options.env).sort(), ['HOME', 'LANG', 'LC_ALL', 'PATH', 'TMPDIR', 'TZ']);
  assert.ok(!JSON.stringify(options).includes('gateway-only-secret'));
});

test('pinned Chromium namespace and seccomp status permits rendering', async () => {
  const fixture = browserFixture({ sandbox: SANDBOX_STATUS });
  const image = await new QualityRenderer(fixture.chromium).render('<p>quality</p>');
  assert.deepEqual(image, pngFixture());
  assert.equal(fixture.state.documents.length, 1);
  assert.equal(fixture.state.closes, 1);
});

for (const scenario of [
  { name: 'unrecognized status', sandbox: 'Namespace sandbox Yes\nSeccomp-BPF sandbox Yes' },
  { name: 'missing layer-one status', sandbox: SANDBOX_STATUS.replace('Layer 1 Sandbox\tNamespace', '') },
  { name: 'setuid instead of namespace sandbox', sandbox: SANDBOX_STATUS.replace('Layer 1 Sandbox\tNamespace', 'Layer 1 Sandbox\tSUID') },
  { name: 'disabled PID namespaces', sandbox: SANDBOX_STATUS.replace('PID namespaces\tYes', 'PID namespaces\tNo') },
  { name: 'disabled network namespaces', sandbox: SANDBOX_STATUS.replace('Network namespaces\tYes', 'Network namespaces\tNo') },
  { name: 'disabled seccomp with TSYNC present', sandbox: SANDBOX_STATUS.replace('Seccomp-BPF sandbox\tYes', 'Seccomp-BPF sandbox\tNo') },
  { name: 'missing seccomp status with TSYNC present', sandbox: SANDBOX_STATUS.replace('Seccomp-BPF sandbox\tYes', '') },
  { name: 'inexact affirmative value', sandbox: SANDBOX_STATUS.replace('PID namespaces\tYes', 'PID namespaces\tYes (unverified)') },
]) {
  test(`${scenario.name} rejects rendering before model HTML is delivered`, async () => {
    const fixture = browserFixture({ sandbox: scenario.sandbox });
    await assert.rejects(new QualityRenderer(fixture.chromium).render('model HTML'), { code: 'sandbox_unavailable' });
    assert.equal(fixture.state.documents.length, 0);
    assert.equal(fixture.state.closes, 1);
  });
}

test('only the single synthetic document is fulfilled and all other routes are aborted', async () => {
  const fixture = browserFixture();
  await new QualityRenderer(fixture.chromium).render('<p>offline</p>');
  const context = fixture.state.contexts.find((entry) => entry.options?.viewport);
  assert.equal(context.options.offline, true);
  assert.equal(context.options.serviceWorkers, 'block');
  assert.equal(context.options.acceptDownloads, false);
  assert.equal(context.options.bypassCSP, false);
  assert.deepEqual(context.options.permissions, []);
  const policy = fixture.state.documents[0].headers['content-security-policy'];
  for (const directive of ["connect-src 'none'", "frame-src 'none'", "object-src 'none'", "worker-src 'none'", "form-action 'none'", "base-uri 'none'", 'sandbox allow-scripts']) {
    assert.ok(policy.split('; ').includes(directive));
  }
  assert.ok(!policy.includes('allow-same-origin'));
  assert.ok(!policy.includes('unsafe-eval'));
  for (const url of ['https://example.invalid/resource', 'file:///not-an-input', DOCUMENT_URL]) {
    let aborted = false;
    await context.routes[0].handler({
      request: () => ({ url: () => url }),
      abort: async () => { aborted = true; },
      fulfill: async () => assert.fail('No further document or remote resource may be fulfilled'),
    });
    assert.equal(aborted, true);
  }
  let socketClosed = false;
  context.wsRoutes[0].handler({ close: () => { socketClosed = true; } });
  assert.equal(socketClosed, true);
});

test('capture policy uses a fixed viewport, environment, sample time and PNG options', async () => {
  const fixture = browserFixture();
  await new QualityRenderer(fixture.chromium).render('quality');
  const options = fixture.state.contexts.find((entry) => entry.options?.viewport).options;
  assert.deepEqual(options.viewport, { width: 1200, height: 800 });
  assert.equal(options.deviceScaleFactor, 1);
  assert.equal(options.locale, 'en-US');
  assert.equal(options.timezoneId, 'UTC');
  assert.equal(fixture.state.sampleTime, 1000);
  assert.equal(fixture.state.fonts.params.fontFamilies.standard, 'Noto Sans');
  assert.deepEqual(fixture.state.captures[0], {
    type: 'png', fullPage: false, animations: 'disabled', caret: 'hide', scale: 'css', timeout: 5000,
  });
});

test('each completed request gets fresh contexts and closes its browser', async () => {
  const fixture = browserFixture();
  const renderer = new QualityRenderer(fixture.chromium);
  await renderer.render('first');
  await renderer.render('second');
  assert.equal(fixture.state.launches.length, 2);
  assert.equal(fixture.state.closes, 2);
  assert.equal(fixture.state.contexts.filter((entry) => entry.options?.viewport).length, 2);
});

test('caller cancellation closes the browser and returns a distinct cancellation error', async () => {
  const entered = deferred();
  const screenshot = deferred();
  const fixture = browserFixture({
    screenshot: () => { entered.resolve(); return screenshot.promise; },
    close: () => screenshot.reject(new Error('Browser closed')),
  });
  const controller = new AbortController();
  const task = new QualityRenderer(fixture.chromium).render('quality', controller.signal);
  const assertion = assert.rejects(task, { code: 'render_cancelled' });
  await entered.promise;
  controller.abort();
  await assertion;
  assert.ok(fixture.state.closes >= 1);
});

test('render deadline interrupts browser work without waiting for wall-clock sleeps', async (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  const entered = deferred();
  const screenshot = deferred();
  const fixture = browserFixture({
    screenshot: () => { entered.resolve(); return screenshot.promise; },
    close: () => screenshot.reject(new Error('Browser closed')),
  });
  const task = new QualityRenderer(fixture.chromium, () => assert.fail('Cleanup completed before the watchdog')).render('quality');
  const assertion = assert.rejects(task, { code: 'render_timeout' });
  await entered.promise;
  t.mock.timers.tick(15_000);
  await assertion;
});

test('cleanup that cannot finish triggers worker termination instead of reuse', async (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  const entered = deferred();
  const cleanup = deferred();
  let terminated = false;
  const fixture = browserFixture({ close: () => { entered.resolve(); return cleanup.promise; } });
  const renderer = new QualityRenderer(fixture.chromium, () => { terminated = true; });
  const task = renderer.render('quality');
  await entered.promise;
  await assert.rejects(renderer.render('next'), { code: 'renderer_busy' });
  t.mock.timers.tick(17_000);
  assert.equal(terminated, true);
  cleanup.resolve();
  assert.deepEqual(await task, pngFixture());
});

test('production worker refuses root, non-Linux and external network interfaces', () => {
  const isolated = { platform: 'linux', uid: 1000, interfaces: { lo: [{ internal: true }] } };
  assert.doesNotThrow(() => assertWorkerIsolation(isolated));
  assert.throws(() => assertWorkerIsolation({ ...isolated, uid: 0 }), /non-root/);
  assert.throws(() => assertWorkerIsolation({ ...isolated, platform: 'win32' }), /Linux/);
  assert.throws(() => assertWorkerIsolation({ ...isolated, interfaces: { eth0: [{ internal: false }] } }), /network=none/);
});
