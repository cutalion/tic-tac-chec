const CACHE_NAME = "ttc-shell-__ASSET_VERSION__";
const APP_SHELL = [
  "/",
  "/app.js?v=__ASSET_VERSION__",
  "/style.css?v=__ASSET_VERSION__",
  "/manifest.json?v=__ASSET_VERSION__",
  "/icon.svg?v=__ASSET_VERSION__",
  "/icon-192.png?v=__ASSET_VERSION__",
  "/icon-512.png?v=__ASSET_VERSION__",
  "/apple-touch-icon.png?v=__ASSET_VERSION__",
  "/favicon.ico?v=__ASSET_VERSION__",
];

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => {
      return cache.addAll(APP_SHELL);
    }),
  );
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches.keys().then((keys) => {
      return Promise.all(
        keys.map((key) => {
          if (key !== CACHE_NAME) {
            return caches.delete(key);
          }
          return Promise.resolve();
        }),
      );
    }),
  );
  self.clients.claim();
});

self.addEventListener("fetch", (event) => {
  const requestURL = new URL(event.request.url);

  if (requestURL.origin !== self.location.origin) {
    return;
  }

  if (requestURL.pathname.startsWith("/api/") || requestURL.pathname.startsWith("/ws/")) {
    return;
  }

  if (event.request.mode === "navigate") {
    event.respondWith(
      fetch(event.request).catch(async () => {
        const cache = await caches.open(CACHE_NAME);
        return cache.match("/") || Response.error();
      }),
    );
    return;
  }

  event.respondWith(
    caches.match(event.request).then((cached) => {
      if (cached) {
        return cached;
      }

      return fetch(event.request).then((response) => {
        if (!response.ok) {
          return response;
        }

        const copy = response.clone();
        caches.open(CACHE_NAME).then((cache) => {
          cache.put(event.request, copy);
        });
        return response;
      });
    }),
  );
});
