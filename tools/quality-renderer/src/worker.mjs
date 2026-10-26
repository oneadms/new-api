// new-api / QuantumNous — requires a non-root, network=none Linux container.
import { chmod, unlink } from 'node:fs/promises';
import { chromium } from 'playwright';
import { createRenderServer } from './http.mjs';
import { assertWorkerIsolation, SOCKET_PATH } from './policy.mjs';
import { QualityRenderer } from './renderer.mjs';

try {
  assertWorkerIsolation();
  // Never configure or inherit gateway credentials in this container.
  if (process.env.QUALITY_RENDERER_TOKEN) throw new Error('Worker must not receive the gateway token');
  const renderer = new QualityRenderer(chromium);
  // Fail startup before listening when the real Chromium sandbox is unavailable.
  await renderer.render('<!doctype html><title>new-api / QuantumNous renderer readiness</title>');
  await unlink(SOCKET_PATH).catch((error) => { if (error.code !== 'ENOENT') throw error; });
  process.umask(0o077);
  const server = createRenderServer({ socketOnly: true, render: renderer.render.bind(renderer) });
  server.on('error', () => { console.error('new-api quality renderer worker listener failed'); process.exit(1); });
  server.listen(SOCKET_PATH, () => { void chmod(SOCKET_PATH, 0o600); });
  // Exiting PID 1's child causes container teardown, including all browser children.
  process.on('SIGTERM', () => process.exit(0));
  process.on('SIGINT', () => process.exit(0));
} catch {
  console.error('new-api quality renderer worker isolation or sandbox check failed; refusing to serve');
  process.exit(1);
}
