import { request } from 'node:http';
import { PNG_LIMIT, RenderError, SOCKET_PATH, validatePNG } from './policy.mjs';

// No URL, hostname, port, proxy or redirect can be supplied by an HTML request.
// The only destination is the deployment's local Unix-domain socket.
export function renderViaWorker(html, signal, socketPath = SOCKET_PATH) {
  return new Promise((resolve, reject) => {
    const body = Buffer.from(JSON.stringify({ html }));
    const req = request({
      socketPath, path: '/render', method: 'POST', agent: false, signal,
      headers: { 'Content-Type': 'application/json', 'Content-Length': body.length },
    }, async (res) => {
      try {
        if (res.statusCode !== 200) {
          const status = [429, 503, 504].includes(res.statusCode) ? res.statusCode : 502;
          throw new RenderError(status, 'worker_unavailable');
        }
        if (res.headers['content-type'] !== 'image/png' || res.headers['content-encoding'] ||
            Number(res.headers['content-length']) > PNG_LIMIT) {
          throw new RenderError(502, 'invalid_worker_response');
        }
        const chunks = [];
        let size = 0;
        for await (const chunk of res) {
          size += chunk.length;
          if (size > PNG_LIMIT) throw new RenderError(502, 'invalid_png_size');
          chunks.push(chunk);
        }
        resolve(validatePNG(Buffer.concat(chunks, size)));
      } catch (error) {
        res.destroy();
        reject(error instanceof RenderError ? error : new RenderError(502, 'worker_unavailable'));
      }
    });
    req.on('error', () => reject(new RenderError(signal.aborted ? 504 : 503, 'worker_unavailable')));
    req.end(body);
  });
}
