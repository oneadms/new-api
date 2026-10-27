import assert from 'node:assert/strict';
import test from 'node:test';
import { createConnection } from 'node:net';
import { createSocket } from 'node:dgram';
import { once } from 'node:events';
import { chmod, copyFile, mkdtemp, readFile, readlink, rm, stat } from 'node:fs/promises';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { dirname, join } from 'node:path';
import { chromium } from 'playwright';
import { createRenderServer } from '../src/http.mjs';
import { assertWorkerIsolation, SOCKET_PATH, validatePNG } from '../src/policy.mjs';
import { QualityRenderer } from '../src/renderer.mjs';
import { renderViaWorker } from '../src/worker-client.mjs';

// Explicit opt-in: run inside the supplied non-root network=none worker image.
// Never silently fall back to an unsandboxed host browser to make this pass.
test('isolated Chromium executes inline drawing and returns the fixed viewport PNG', {
  skip: process.env.QUALITY_RENDERER_INTEGRATION !== '1',
}, async () => {
  assertWorkerIsolation();
  const renderer = new QualityRenderer(chromium);
  const painted = await renderer.render('<!doctype html><body><script>document.body.style.backgroundColor="rgb(12, 34, 56)";</script>');
  const staticPage = await renderer.render('<!doctype html><body style="background-color:rgb(12, 34, 56)">');
  assert.equal(validatePNG(painted), painted);
  assert.deepEqual(painted, staticPage);
});

test('non-root worker serves a PNG through an owner-only socket on the shared volume', {
  skip: process.env.QUALITY_RENDERER_INTEGRATION !== '1',
}, async (t) => {
  assertWorkerIsolation();
  const directory = await mkdtemp(join(dirname(SOCKET_PATH), 'acceptance-'));
  const socketPath = join(directory, 'worker.sock');
  const renderer = new QualityRenderer(chromium);
  const server = createRenderServer({ socketOnly: true, render: renderer.render.bind(renderer) });
  t.after(async () => {
    try {
      if (server.listening) {
        const closed = once(server, 'close');
        server.closeAllConnections();
        server.close();
        await closed;
      }
    } finally {
      await rm(directory, { recursive: true, force: true });
    }
  });
  server.listen(socketPath);
  await once(server, 'listening');
  await chmod(socketPath, 0o600);
  const socket = await stat(socketPath);
  assert.equal(socket.uid, process.getuid());
  assert.equal(socket.mode & 0o777, 0o600);
  const image = await renderViaWorker('<!doctype html><title>new-api / QuantumNous acceptance</title>', new AbortController().signal, socketPath);
  assert.equal(validatePNG(image), image);
});

test('worker runs without credentials, capabilities or privilege escalation', {
  skip: process.env.QUALITY_RENDERER_INTEGRATION !== '1',
}, async () => {
  assertWorkerIsolation();
  assert.equal(process.getuid(), 1000);
  assert.equal(process.env.QUALITY_RENDERER_TOKEN, undefined);
  const status = await readFile('/proc/self/status', 'utf8');
  assert.match(status, /^NoNewPrivs:\s+1$/m);
  assert.match(status, /^Seccomp:\s+2$/m);
  assert.match(status, /^CapEff:\s+0+$/m);
  if (process.env.QUALITY_RENDERER_PARENT_NETNS) {
    assert.notEqual(await readlink('/proc/self/ns/net'), process.env.QUALITY_RENDERER_PARENT_NETNS);
  }
});

const writableDirectories = ['/tmp', '/dev/shm'];
if (process.env.QUALITY_RENDERER_PARENT_NETNS) writableDirectories.push(dirname(SOCKET_PATH));
for (const root of writableDirectories) {
  test(`worker cannot execute files from writable ${root}`, {
    skip: process.env.QUALITY_RENDERER_INTEGRATION !== '1',
  }, async (t) => {
    assertWorkerIsolation();
    const directory = await mkdtemp(join(root, 'execution-acceptance-'));
    t.after(() => rm(directory, { recursive: true, force: true }));
    const executable = join(directory, 'true');
    await copyFile('/bin/true', executable);
    await chmod(executable, 0o700);
    await assert.rejects(promisify(execFile)(executable), { code: 'EACCES' });
  });
}

for (const address of ['127.0.0.1', '::1']) {
  test(`worker seccomp denies TCP socket creation for ${address}`, {
    skip: process.env.QUALITY_RENDERER_INTEGRATION !== '1',
  }, async () => {
    assertWorkerIsolation();
    const socket = createConnection({ host: address, port: 9 });
    const failure = once(socket, 'error');
    try {
      const [error] = await failure;
      // Refusal, timeout or no route would mean an IP socket was permitted.
      assert.equal(error.code, 'EPERM');
    } finally {
      socket.destroy();
    }
  });
}

for (const type of ['udp4', 'udp6']) {
  test(`worker seccomp denies ${type} socket creation`, {
    skip: process.env.QUALITY_RENDERER_INTEGRATION !== '1',
  }, async () => {
    assertWorkerIsolation();
    const socket = createSocket(type);
    const failure = new Promise((resolve, reject) => {
      socket.once('error', resolve);
      socket.once('listening', () => reject(new Error('IP sockets must be denied')));
    });
    socket.bind(0);
    try {
      const error = await failure;
      assert.equal(error.code, 'EPERM');
    } finally {
      socket.close();
    }
  });
}
