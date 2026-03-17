const CACHE_NAME = "todo-pwa-v4";
const OFFLINE_URL = "/static/pwa/offline.html";
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

async function fetchAndCache(request, preloadResponsePromise) {
  const networkResponse = (await preloadResponsePromise) || (await fetch(request));
  if (!networkResponse || networkResponse.status !== 200 || networkResponse.type !== "basic") {
    return networkResponse;
  }

  const responseClone = networkResponse.clone();
  caches.open(CACHE_NAME).then((cache) => cache.put(request, responseClone));
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

  if (request.mode === "navigate") {
    event.respondWith(
      caches.match(request).then((cachedResponse) => {
        const networkResponse = fetchAndCache(request, event.preloadResponse).catch(() => null);

        if (cachedResponse) {
          event.waitUntil(networkResponse);
          return cachedResponse;
        }

        return networkResponse.then((response) => response || caches.match(OFFLINE_URL));
      })
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
    caches.match(request).then((cachedResponse) => {
      const networkResponse = fetchAndCache(request).catch(() => null);

      if (cachedResponse) {
        event.waitUntil(networkResponse);
        return cachedResponse;
      }

      return networkResponse.then((response) => response || caches.match(request));
    })
  );
});
