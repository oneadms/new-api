import assert from 'node:assert/strict';
import { request } from 'node:http';
import { once } from 'node:events';
import test from 'node:test';
import { createRenderServer } from '../src/http.mjs';
import { HTML_LIMIT, JSON_LIMIT } from '../src/policy.mjs';
import { QualityRenderer } from '../src/renderer.mjs';
import { browserFixture, deferred, pngFixture } from './fixtures.mjs';

async function fixture(t, options = {}) {
  const browser = browserFixture(options);
  const renderer = new QualityRenderer(browser.chromium, () => assert.fail('Unexpected worker termination'));
  const server = createRenderServer({ token: 'test-token', render: renderer.render.bind(renderer) });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  t.after(() => { server.closeAllConnections(); server.close(); });
  return { ...browser, server, url: `http://127.0.0.1:${server.address().port}` };
}

function submit(url, body, init = {}) {
  return fetch(`${url}/render`, {
    method: 'POST', headers: { authorization: 'Bearer test-token', 'content-type': 'application/json' },
    body: JSON.stringify(body), ...init,
  });
}

test('valid authenticated HTML returns only a fixed-size PNG', async (t) => {
  const instance = await fixture(t);
  const response = await submit(instance.url, { html: '<p>new-api / QuantumNous</p>' });
  assert.equal(response.status, 200);
  assert.equal(response.headers.get('content-type'), 'image/png');
  assert.equal(response.headers.get('cache-control'), 'no-store');
  assert.match(response.headers.get('x-quality-renderer-version'), /^new-api-quality-v1/);
  assert.deepEqual(Buffer.from(await response.arrayBuffer()), pngFixture());
  assert.equal(instance.state.documents[0].body, '<p>new-api / QuantumNous</p>');
});

test('missing or invalid tokens cannot launch a browser', async (t) => {
  const instance = await fixture(t);
  for (const authorization of ['', 'Bearer wrong-token', 'Basic test-token']) {
    const response = await submit(instance.url, { html: '<p>ignored</p>' }, {
      headers: { authorization, 'content-type': 'application/json' },
    });
    assert.equal(response.status, 401);
    assert.deepEqual(await response.json(), { error: 'unauthorized' });
  }
  assert.equal(instance.state.launches.length, 0);
  for (const token of [undefined, '', ' ', 'bad\r\nvalue']) {
    assert.throws(() => createRenderServer({ token, render: () => {} }), /nonempty printable/);
  }
});

test('request schema rejects URLs, renderer options, empty HTML and malformed JSON', async (t) => {
  const instance = await fixture(t);
  for (const body of [null, [], { url: 'https://example.invalid/' }, { html: 'x', width: 1 }, { html: '' }, { html: 2 }, { html: '\ud800' }]) {
    const response = await submit(instance.url, body);
    assert.equal(response.status, 400);
  }
  assert.equal((await submit(instance.url, null, { body: '{broken' })).status, 400);
  assert.equal(instance.state.launches.length, 0);
});

test('HTML limit counts UTF-8 bytes and accepts the exact one MiB boundary', async (t) => {
  const instance = await fixture(t);
  assert.equal((await submit(instance.url, { html: 'x'.repeat(HTML_LIMIT) })).status, 200);
  assert.equal((await submit(instance.url, { html: 'é'.repeat(HTML_LIMIT / 2 + 1) })).status, 413);
  assert.equal(instance.state.launches.length, 1);
});

test('escaped HTML is accepted within the JSON envelope limit', async (t) => {
  const instance = await fixture(t);
  // Go's common.Marshal escapes HTML delimiters this way.
  const response = await submit(instance.url, null, { body: '{"html":"' + '\\u003c'.repeat(HTML_LIMIT) + '"}' });
  assert.equal(response.status, 200);
});

test('chunked bodies exceeding the envelope limit receive 413 before rendering', async (t) => {
  const instance = await fixture(t);
  const status = await new Promise((resolve, reject) => {
    const req = request(`${instance.url}/render`, {
      method: 'POST', headers: { authorization: 'Bearer test-token', 'content-type': 'application/json' },
    }, (res) => { res.resume(); resolve(res.statusCode); });
    req.on('error', reject);
    req.write(Buffer.alloc(JSON_LIMIT, 32));
    req.end(' ');
  });
  assert.equal(status, 413);
  assert.equal(instance.state.launches.length, 0);
});

test('wrong methods, paths, encodings and media types are rejected', async (t) => {
  const instance = await fixture(t);
  assert.equal((await submit(instance.url, undefined, { method: 'GET', body: undefined })).status, 405);
  assert.equal((await fetch(`${instance.url}/other`, { headers: { authorization: 'Bearer test-token' } })).status, 404);
  for (const headers of [
    { 'content-type': 'text/html' },
    { 'content-type': 'application/json', 'content-encoding': 'gzip' },
  ]) {
    assert.equal((await submit(instance.url, { html: 'x' }, { headers: { authorization: 'Bearer test-token', ...headers } })).status, 415);
  }
  assert.equal(instance.state.launches.length, 0);
});

test('a second request is rejected instead of queued while a browser is busy', async (t) => {
  const entered = deferred();
  const release = deferred();
  t.after(() => release.resolve(pngFixture()));
  const instance = await fixture(t, { screenshot: () => { entered.resolve(); return release.promise; } });
  const first = submit(instance.url, { html: '<p>first</p>' });
  await entered.promise;
  assert.equal((await submit(instance.url, { html: '<p>second</p>' })).status, 429);
  release.resolve(pngFixture());
  assert.equal((await first).status, 200);
  assert.equal((await submit(instance.url, { html: '<p>third</p>' })).status, 200);
});

test('oversized or non-PNG browser output never reaches clients as success', async (t) => {
  for (const png of [Buffer.alloc((2 << 20) + 1), Buffer.from('<html>bad output</html>'), pngFixture(1, 1)]) {
    const instance = await fixture(t, { png });
    const response = await submit(instance.url, { html: 'x' });
    assert.equal(response.status, 502);
    assert.match(response.headers.get('content-type'), /^application\/json/);
  }
});

test('the uncredentialed worker listener refuses TCP requests', async (t) => {
  const server = createRenderServer({ socketOnly: true, render: () => assert.fail('Must not render') });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  t.after(() => { server.closeAllConnections(); server.close(); });
  const response = await submit(`http://127.0.0.1:${server.address().port}`, { html: 'x' });
  assert.equal(response.status, 403);
});
