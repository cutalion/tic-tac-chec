const CACHE_NAME = "ttc-shell-v10";
const APP_SHELL = [
  "/",
  "/app.js",
  "/style.css",
  "/manifest.json",
  "/icon-192-v2.png",
  "/icon-512-v2.png",
  "/icon-512-maskable-v2.png",
  "/apple-touch-icon-v2.png",
  "/favicon.ico",
  "/fonts/fraunces-500.woff2",
  "/fonts/inter-400.woff2",
  "/fonts/inter-500.woff2",
  "/ttc_logo.png",
  "/sounds/move.mp3",
  "/sounds/capture.mp3",
  "/sounds/win.mp3",
  "/sounds/lose.mp3",
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

  event.respondWith(
    fetch(event.request)
      .then((response) => {
        if (response.ok) {
          const copy = response.clone();
          caches.open(CACHE_NAME).then((cache) => {
            cache.put(event.request, copy);
          });
        }
        return response;
      })
      .catch(() => {
        return caches.match(event.request).then((cached) => {
          if (cached) {
            return cached;
          }

          if (event.request.mode === "navigate") {
            return caches.match("/");
          }

          return Response.error();
        });
      }),
  );
});
