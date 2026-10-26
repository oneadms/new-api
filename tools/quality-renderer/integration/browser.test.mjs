import assert from 'node:assert/strict';
import test from 'node:test';
import { createConnection } from 'node:net';
import { chromium } from 'playwright';
import { assertWorkerIsolation, validatePNG } from '../src/policy.mjs';
import { QualityRenderer } from '../src/renderer.mjs';

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

test('worker seccomp denies even loopback IP socket creation', {
  skip: process.env.QUALITY_RENDERER_INTEGRATION !== '1',
}, async () => {
  assertWorkerIsolation();
  await new Promise((resolve, reject) => {
    const socket = createConnection({ host: '127.0.0.1', port: 9 });
    socket.on('connect', () => { socket.destroy(); reject(new Error('IP sockets must be denied')); });
    socket.on('error', (error) => {
      // ECONNREFUSED is not sufficient: that would mean the kernel permitted an
      // IP socket and merely found no listener. This requires the seccomp deny.
      try { assert.equal(error.code, 'EPERM'); resolve(); } catch (failure) { reject(failure); }
    });
  });
});
