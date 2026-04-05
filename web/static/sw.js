const CACHE_NAME = "todo-pwa-v13";
const OFFLINE_URL = "/static/pwa/offline.html";
const NAVIGATION_NETWORK_TIMEOUT_MS = 4500;
const STATIC_NETWORK_TIMEOUT_MS = 2200;
const STATIC_ASSETS = [
  "/static/styles.css",
  "/static/date-picker.js",
  "/static/focus-page.js",
  "/static/task-cards.js",
  "/static/realtime-sync.js",
  "/static/postpone-picker.js",
  "/static/composer-panel.js",
  "/static/pwa-register.js",
  "/manifest.webmanifest",
  "/static/pwa/favicon-64.png",
  "/static/pwa/favicon-32.png",
  "/static/pwa/favicon-16.png",
  "/static/pwa/apple-touch-icon.png",
  "/static/pwa/icon-192.png",
  "/static/pwa/icon-512.png",
  "/static/pwa/maskable-512.png",
  "/favicon.ico",
  OFFLINE_URL
];

function timeoutReject(ms) {
  return new Promise((_, reject) => {
    setTimeout(() => reject(new Error("timeout")), ms);
  });
}

async function fetchWithTimeout(request, timeoutMs) {
  if (!timeoutMs || timeoutMs <= 0) {
    return fetch(request);
  }

  if (typeof AbortController === "function") {
    const controller = new AbortController();
    const timer = setTimeout(() => {
      controller.abort();
    }, timeoutMs);

    try {
      return await fetch(request, { signal: controller.signal });
    } finally {
      clearTimeout(timer);
    }
  }

  return Promise.race([fetch(request), timeoutReject(timeoutMs)]);
}

async function resolvePreloadedResponse(preloadResponsePromise, timeoutMs) {
  if (!preloadResponsePromise) {
    return null;
  }

  try {
    const response = await Promise.race([preloadResponsePromise, timeoutReject(timeoutMs)]);
    return response || null;
  } catch (_error) {
    return null;
  }
}

function staticCacheKey(url) {
  return url.search ? `${url.pathname}${url.search}` : url.pathname;
}

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => cache.addAll(STATIC_ASSETS)).then(() => self.skipWaiting())
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    Promise.all([
      caches.keys().then((keys) =>
        Promise.all(
          keys
            .filter((key) => key !== CACHE_NAME)
            .map((key) => caches.delete(key))
        )
      ),
      self.registration.navigationPreload
        ? self.registration.navigationPreload.enable().catch(() => {})
        : Promise.resolve(),
    ]).then(() => self.clients.claim())
  );
});

async function fetchAndCache(request, preloadResponsePromise, options = {}) {
  const {
    cacheKey = request,
    timeoutMs = 0,
  } = options;
  const preloadedResponse = await resolvePreloadedResponse(preloadResponsePromise, timeoutMs);
  const networkResponse = preloadedResponse || (await fetchWithTimeout(request, timeoutMs));
  if (!networkResponse || networkResponse.status !== 200 || networkResponse.type !== "basic") {
    return networkResponse;
  }

  const cacheControl = networkResponse.headers.get("Cache-Control") || "";
  if (/\bno-store\b/i.test(cacheControl)) {
    return networkResponse;
  }

  const responseClone = networkResponse.clone();
  caches.open(CACHE_NAME).then((cache) => cache.put(cacheKey, responseClone));
  return networkResponse;
}

self.addEventListener("fetch", (event) => {
  const { request } = event;
  if (request.method !== "GET") {
    return;
  }

  const url = new URL(request.url);
  if (url.origin !== self.location.origin) {
    return;
  }

  if (url.pathname === "/events" || url.pathname === "/dashboard/snapshot") {
    event.respondWith(fetch(request));
    return;
  }

  if (url.pathname === "/me") {
    event.respondWith(
      fetch(request).catch(async () => {
        const offlineResponse = await caches.match(OFFLINE_URL);
        return offlineResponse || Response.error();
      })
    );
    return;
  }

  if (request.mode === "navigate") {
    event.respondWith(
      (async () => {
        try {
          const networkResponse = await fetchAndCache(request, event.preloadResponse, {
            timeoutMs: NAVIGATION_NETWORK_TIMEOUT_MS,
          });
          if (networkResponse) {
            return networkResponse;
          }
        } catch (_error) {
          // Fall through to cache/offline fallback.
        }

        const cachedResponse = await caches.match(request);
        if (cachedResponse) {
          return cachedResponse;
        }

        return caches.match(OFFLINE_URL);
      })()
    );
    return;
  }

  const isStaticAsset =
    url.pathname.startsWith("/static/") ||
    url.pathname === "/manifest.webmanifest" ||
    url.pathname === "/favicon.ico";

  if (!isStaticAsset) {
    event.respondWith(fetch(request));
    return;
  }

  event.respondWith(
    (async () => {
      const cacheKey = staticCacheKey(url);
      const cachedResponse = (await caches.match(request)) || (await caches.match(cacheKey));
      const networkResponse = fetchAndCache(request, null, {
        cacheKey,
        timeoutMs: STATIC_NETWORK_TIMEOUT_MS,
      }).catch(() => null);

      if (cachedResponse) {
        event.waitUntil(networkResponse);
        return cachedResponse;
      }

      const response = await networkResponse;
      return response || caches.match(cacheKey);
    })()
  );
});
