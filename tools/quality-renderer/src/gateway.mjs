// new-api / QuantumNous — authenticated ingress; Chromium runs in another container.
import { createRenderServer } from './http.mjs';
import { renderViaWorker } from './worker-client.mjs';

try {
  if (process.getuid?.() === 0) throw new Error('Non-root user required');
  const server = createRenderServer({
    token: process.env.QUALITY_RENDERER_TOKEN,
    render: renderViaWorker,
  });
  server.on('error', () => { console.error('new-api quality renderer listener failed'); process.exit(1); });
  server.listen(3000, '0.0.0.0');
  const stop = () => { server.closeAllConnections(); server.close(() => process.exit(0)); };
  process.on('SIGTERM', stop);
  process.on('SIGINT', stop);
} catch {
  console.error('new-api quality renderer requires non-root execution and a valid authentication token');
  process.exit(1);
}
