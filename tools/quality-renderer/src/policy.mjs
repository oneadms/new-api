// new-api / QuantumNous — optional isolated screenshot service.
import { networkInterfaces } from 'node:os';

export const HTML_LIMIT = 1024 * 1024;
// JSON escaping can expand one source byte into six (including Go's HTML escaping).
export const JSON_LIMIT = HTML_LIMIT * 6 + 64;
export const PNG_LIMIT = 2 * 1024 * 1024;
export const VIEWPORT = Object.freeze({ width: 1200, height: 800 });
export const RENDER_VERSION = 'new-api-quality-v1-pw1.64.1';
export const DOCUMENT_URL = 'http://quality-render.invalid/document';
export const SOCKET_PATH = '/run/quality-renderer/worker.sock';
export const RENDER_TIMEOUT_MS = 15_000;
export const CLEANUP_TIMEOUT_MS = 2_000;
export const REQUEST_TIMEOUT_MS = 18_000;

// Delivered as an HTTP response header, not a removable HTML meta element.
// No unsafe-eval, external scripts, same-origin privileges, popups or downloads.
export const CSP = [
  "default-src 'none'",
  "script-src 'unsafe-inline'",
  "style-src 'unsafe-inline'",
  'img-src data:',
  'font-src data:',
  "connect-src 'none'",
  "frame-src 'none'",
  "child-src 'none'",
  "object-src 'none'",
  "worker-src 'none'",
  "media-src 'none'",
  "manifest-src 'none'",
  "base-uri 'none'",
  "form-action 'none'",
  "frame-ancestors 'none'",
  'sandbox allow-scripts',
].join('; ');

export class RenderError extends Error {
  constructor(status, code) {
    super(code);
    this.status = status;
    this.code = code;
  }
}

export function validateToken(token) {
  if (typeof token !== 'string' || !/^[\x21-\x7e]{1,4096}$/.test(token)) {
    throw new Error('A nonempty printable QUALITY_RENDERER_TOKEN is required');
  }
}

export function parseHTMLRequest(bytes) {
  if (bytes.length > JSON_LIMIT) throw new RenderError(413, 'input_too_large');
  let value;
  try {
    value = JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(bytes));
  } catch {
    throw new RenderError(400, 'invalid_json');
  }
  if (!value || typeof value !== 'object' || Array.isArray(value) ||
      Object.keys(value).length !== 1 || typeof value.html !== 'string' ||
      !value.html.isWellFormed() || value.html.length === 0) {
    throw new RenderError(400, 'invalid_html_request');
  }
  if (Buffer.byteLength(value.html, 'utf8') > HTML_LIMIT) {
    throw new RenderError(413, 'input_too_large');
  }
  return value.html;
}

export function validatePNG(bytes) {
  if (!Buffer.isBuffer(bytes) || bytes.length > PNG_LIMIT) {
    throw new RenderError(502, 'invalid_png_size');
  }
  const signature = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]);
  const end = Buffer.from([0, 0, 0, 0, 73, 69, 78, 68, 174, 66, 96, 130]);
  if (bytes.length < 45 || !bytes.subarray(0, 8).equals(signature) ||
      bytes.readUInt32BE(8) !== 13 || bytes.toString('ascii', 12, 16) !== 'IHDR' ||
      bytes.readUInt32BE(16) !== VIEWPORT.width || bytes.readUInt32BE(20) !== VIEWPORT.height ||
      !bytes.subarray(-12).equals(end)) {
    throw new RenderError(502, 'invalid_png');
  }
  // The Go consumer additionally decodes all pixels and verifies PNG checksums.
  return bytes;
}

export function assertWorkerIsolation(runtime = {
  platform: process.platform,
  uid: process.getuid?.(),
  interfaces: networkInterfaces(),
}) {
  if (runtime.platform !== 'linux' || !Number.isInteger(runtime.uid) || runtime.uid === 0) {
    throw new Error('The renderer worker requires non-root Linux isolation');
  }
  if (Object.values(runtime.interfaces).flat().some((address) => !address.internal)) {
    throw new Error('The renderer worker requires a network=none namespace');
  }
}

// Never inherit process.env: the gateway token, DB credentials, proxies,
// NODE_OPTIONS and LD_PRELOAD must not reach Chromium or its subprocesses.
export function browserLaunchOptions() {
  return {
    channel: 'chromium',
    headless: true,
    chromiumSandbox: true,
    timeout: 8_000,
    env: {
      PATH: '/usr/local/bin:/usr/bin:/bin',
      HOME: '/tmp',
      TMPDIR: '/tmp',
      LANG: 'C.UTF-8',
      LC_ALL: 'C.UTF-8',
      TZ: 'UTC',
    },
    args: [
      '--disable-background-networking',
      '--disable-component-update',
      '--disable-domain-reliability',
      '--disable-sync',
      '--no-first-run',
      '--force-webrtc-ip-handling-policy=disable_non_proxied_udp',
      '--host-resolver-rules=MAP * ~NOTFOUND',
    ],
  };
}
