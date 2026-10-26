import { timingSafeEqual } from 'node:crypto';
import { createServer } from 'node:http';
import {
  JSON_LIMIT, parseHTMLRequest, RENDER_VERSION, REQUEST_TIMEOUT_MS,
  RenderError, validatePNG, validateToken,
} from './policy.mjs';

export function createRenderServer({ render, token, socketOnly = false }) {
  if (!socketOnly) validateToken(token);
  const authorization = socketOnly ? null : Buffer.from(`Bearer ${token}`);
  let busy = false;
  const server = createServer({ maxHeaderSize: 16 * 1024 }, async (req, res) => {
    res.setHeader('Cache-Control', 'no-store');
    res.setHeader('X-Content-Type-Options', 'nosniff');
    res.setHeader('Connection', 'close');
    let ownsSlot = false;
    let uploadTimer;
    const abort = new AbortController();
    const deadline = setTimeout(() => abort.abort(), REQUEST_TIMEOUT_MS);
    const disconnected = () => { if (!res.writableEnded) abort.abort(); };
    res.on('close', disconnected);
    try {
      // The uncredentialed worker must only accept a Unix-domain connection.
      if (socketOnly && req.socket.remoteAddress !== undefined) {
        throw new RenderError(403, 'unix_socket_required');
      }
      if (!socketOnly) {
        const supplied = Buffer.from(req.headers.authorization ?? '');
        if (req.headersDistinct.authorization?.length !== 1 || supplied.length !== authorization.length ||
            !timingSafeEqual(supplied, authorization)) {
          throw new RenderError(401, 'unauthorized');
        }
      }
      if (req.url !== '/render') throw new RenderError(404, 'not_found');
      if (req.method !== 'POST') throw new RenderError(405, 'method_not_allowed');
      if (!/^application\/json(?:\s*;\s*charset=utf-8)?$/i.test(req.headers['content-type'] ?? '') ||
          (req.headers['content-encoding'] && req.headers['content-encoding'] !== 'identity')) {
        throw new RenderError(415, 'unsupported_media_type');
      }
      if (Number(req.headers['content-length']) > JSON_LIMIT) throw new RenderError(413, 'input_too_large');
      if (busy) throw new RenderError(429, 'renderer_busy');
      busy = true;
      ownsSlot = true;
      // Limit one body in memory and do not enqueue further render requests.
      let size = 0;
      const chunks = [];
      uploadTimer = setTimeout(() => { abort.abort(); req.destroy(); }, 5_000);
      for await (const chunk of req.iterator({ destroyOnReturn: false })) {
        size += chunk.length;
        if (size > JSON_LIMIT) throw new RenderError(413, 'input_too_large');
        chunks.push(chunk);
      }
      clearTimeout(uploadTimer);
      if (abort.signal.aborted) throw new RenderError(504, 'request_timeout');
      const html = parseHTMLRequest(Buffer.concat(chunks, size));
      const image = validatePNG(await render(html, abort.signal));
      if (abort.signal.aborted) throw new RenderError(504, 'request_timeout');
      res.writeHead(200, {
        'Content-Type': 'image/png',
        'Content-Length': image.length,
        'X-Quality-Renderer-Version': RENDER_VERSION,
      });
      res.end(image);
    } catch (error) {
      if (!res.destroyed && !res.headersSent) {
        const status = error instanceof RenderError ? error.status : 502;
        const code = error instanceof RenderError ? error.code : 'render_failed';
        res.writeHead(status, { 'Content-Type': 'application/json; charset=utf-8' });
        res.end(JSON.stringify({ error: code }));
      }
    } finally {
      clearTimeout(uploadTimer);
      clearTimeout(deadline);
      res.removeListener('close', disconnected);
      if (ownsSlot) busy = false;
    }
  });
  server.headersTimeout = 5_000;
  server.requestTimeout = 5_000;
  server.keepAliveTimeout = 1_000;
  server.maxRequestsPerSocket = 1;
  server.maxHeadersCount = 32;
  server.maxConnections = 16;
  server.on('upgrade', (_req, socket) => socket.destroy());
  server.on('connect', (_req, socket) => socket.destroy());
  return server;
}
