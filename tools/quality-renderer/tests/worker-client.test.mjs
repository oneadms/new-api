import assert from 'node:assert/strict';
import { once } from 'node:events';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import { createRenderServer } from '../src/http.mjs';
import { QualityRenderer } from '../src/renderer.mjs';
import { renderViaWorker } from '../src/worker-client.mjs';
import { browserFixture, deferred, pngFixture } from './fixtures.mjs';

let sequence = 0;
async function workerFixture(t, options = {}) {
  const fixture = browserFixture(options);
  const renderer = new QualityRenderer(fixture.chromium, () => assert.fail('Unexpected termination'));
  const server = createRenderServer({ socketOnly: true, render: renderer.render.bind(renderer) });
  const name = `new-api-quality-test-${process.pid}-${sequence++}`;
  const socketPath = process.platform === 'win32' ? `\\\\.\\pipe\\${name}` : join(tmpdir(), `${name}.sock`);
  server.listen(socketPath);
  await once(server, 'listening');
  t.after(() => { server.closeAllConnections(); server.close(); });
  return { ...fixture, socketPath };
}

test('gateway uses the local socket and returns the worker PNG without credentials', async (t) => {
  const fixture = await workerFixture(t);
  const result = await renderViaWorker('<p>local only</p>', new AbortController().signal, fixture.socketPath);
  assert.deepEqual(result, pngFixture());
  assert.equal(fixture.state.documents[0].body, '<p>local only</p>');
});

test('worker sandbox failures remain unavailable errors with no browser detail', async (t) => {
  const fixture = await workerFixture(t, { sandbox: 'private-error-details' });
  await assert.rejects(renderViaWorker('quality', new AbortController().signal, fixture.socketPath), {
    status: 503, code: 'worker_unavailable', message: 'worker_unavailable',
  });
});

test('cancellation closes the local request and interrupts the worker browser', async (t) => {
  const entered = deferred();
  const capture = deferred();
  const closed = deferred();
  const fixture = await workerFixture(t, {
    screenshot: () => { entered.resolve(); return capture.promise; },
    close: () => { capture.reject(new Error('Closed')); closed.resolve(); },
  });
  const controller = new AbortController();
  const request = renderViaWorker('quality', controller.signal, fixture.socketPath);
  const assertion = assert.rejects(request, { status: 504, code: 'worker_unavailable' });
  await entered.promise;
  controller.abort();
  await assertion;
  await closed.promise;
});
