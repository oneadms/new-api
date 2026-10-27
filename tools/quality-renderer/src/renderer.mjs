// new-api / QuantumNous — all untrusted content stays in this isolated worker.
import {
  browserLaunchOptions, CLEANUP_TIMEOUT_MS, CSP, DOCUMENT_URL,
  RENDER_TIMEOUT_MS, RenderError, validatePNG, VIEWPORT,
} from './policy.mjs';

export class QualityRenderer {
  constructor(chromium, fatal = () => process.exit(1)) {
    this.chromium = chromium;
    this.fatal = fatal;
    this.busy = false;
  }

  async render(html, signal = new AbortController().signal) {
    if (this.busy) throw new RenderError(429, 'renderer_busy');
    if (signal.aborted) throw new RenderError(499, 'render_cancelled');
    this.busy = true;
    let browser;
    let timedOut = false;
    // If Chromium or cleanup hangs, exit the whole worker container. Container
    // PID namespace teardown kills descendants; never reuse a stuck browser.
    const watchdog = setTimeout(this.fatal, RENDER_TIMEOUT_MS + CLEANUP_TIMEOUT_MS);
    const interrupt = () => { void browser?.close().catch(() => {}); };
    const deadline = setTimeout(() => {
      timedOut = true;
      interrupt();
    }, RENDER_TIMEOUT_MS);
    signal.addEventListener('abort', interrupt, { once: true });
    try {
      browser = await this.chromium.launch(browserLaunchOptions());
      if (signal.aborted || timedOut) throw new RenderError(504, 'render_interrupted');

      // Chromium can start in a weakened configuration on incompatible hosts.
      // Verify real runtime status on every launch, before loading model HTML.
      const probe = await browser.newContext();
      try {
        const page = await probe.newPage();
        await page.goto('chrome://sandbox/', { waitUntil: 'domcontentloaded', timeout: 5_000 });
        const status = await page.locator('body').innerText({ timeout: 5_000 });
        // The pinned Chromium reports the layer-one mechanism and namespaces
        // separately. Match complete rows; TSYNC alone is not seccomp evidence.
        const rows = new Set(status.split(/\r?\n/).map((line) => line.trim().replace(/\s+/g, ' ')));
        if (!rows.has('Layer 1 Sandbox Namespace') || !rows.has('PID namespaces Yes') ||
            !rows.has('Network namespaces Yes') || !rows.has('Seccomp-BPF sandbox Yes')) {
          throw new RenderError(503, 'sandbox_unavailable');
        }
      } finally {
        await probe.close();
      }

      const context = await browser.newContext({
        viewport: { ...VIEWPORT },
        screen: { ...VIEWPORT },
        deviceScaleFactor: 1,
        locale: 'en-US',
        timezoneId: 'UTC',
        colorScheme: 'light',
        reducedMotion: 'reduce',
        forcedColors: 'none',
        acceptDownloads: false,
        serviceWorkers: 'block',
        permissions: [],
        offline: true,
        bypassCSP: false,
        javaScriptEnabled: true,
      });
      context.setDefaultTimeout(10_000);
      context.setDefaultNavigationTimeout(10_000);
      const page = await context.newPage();
      page.on('dialog', (dialog) => { void dialog.dismiss().catch(() => {}); });
      page.on('download', (download) => { void download.cancel().catch(() => {}); });
      context.on('page', (extraPage) => { void extraPage.close().catch(() => {}); });
      await context.routeWebSocket('**/*', (socket) => {
        socket.close({ code: 1008, reason: 'Network disabled' });
      });
      let documentDelivered = false;
      await context.route('**/*', async (route) => {
        const request = route.request();
        if (!documentDelivered && request.url() === DOCUMENT_URL &&
            request.method() === 'GET' && request.isNavigationRequest() && request.frame() === page.mainFrame()) {
          documentDelivered = true;
          // This fixed synthetic URL is fulfilled from memory, never fetched or
          // resolved. A header policy cannot be removed by model HTML/scripts.
          await route.fulfill({
            status: 200,
            headers: {
              'content-type': 'text/html; charset=utf-8',
              'content-security-policy': CSP,
              'referrer-policy': 'no-referrer',
              'x-content-type-options': 'nosniff',
              'cache-control': 'no-store',
            },
            body: html,
          });
          return;
        }
        await route.abort('blockedbyclient');
      });
      const session = await context.newCDPSession(page);
      await session.send('Page.setFontFamilies', {
        fontFamilies: {
          standard: 'Noto Sans', fixed: 'Noto Sans Mono', serif: 'Noto Serif',
          sansSerif: 'Noto Sans', cursive: 'Noto Sans', fantasy: 'Noto Sans',
        },
      });
      const epoch = new Date('2026-01-01T00:00:00.000Z');
      await page.clock.install({ time: epoch });
      await page.clock.pauseAt(epoch);
      await page.goto(DOCUMENT_URL, { waitUntil: 'domcontentloaded' });
      await page.evaluate(() => document.fonts.ready);
      await page.clock.runFor(1_000);
      const png = await page.screenshot({
        type: 'png', fullPage: false, animations: 'disabled', caret: 'hide',
        scale: 'css', timeout: 5_000,
      });
      if (signal.aborted || timedOut) throw new RenderError(504, 'render_interrupted');
      return validatePNG(png);
    } catch (error) {
      if (timedOut) throw new RenderError(504, 'render_timeout');
      if (signal.aborted) throw new RenderError(499, 'render_cancelled');
      if (error instanceof RenderError) throw error;
      // Playwright exceptions can contain untrusted HTML, paths or page console
      // output. Do not log or return them across the worker boundary.
      throw new RenderError(502, 'render_failed');
    } finally {
      clearTimeout(deadline);
      signal.removeEventListener('abort', interrupt);
      try {
        if (browser) await browser.close();
      } catch {
        // Losing reliable cleanup is a fatal isolation failure, not a reason to
        // put the worker back into service.
        this.fatal();
      } finally {
        clearTimeout(watchdog);
        this.busy = false;
      }
    }
  }
}
