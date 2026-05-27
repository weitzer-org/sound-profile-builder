const CACHE_NAME = "qc2-modeler-cache-v1";
const ASSETS_TO_CACHE = [
  "/login",
  "/static/manifest.json"
];

// Install Event
self.addEventListener("install", (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => {
      return cache.addAll(ASSETS_TO_CACHE);
    }).then(() => self.skipWaiting())
  );
});

// Activate Event
self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches.keys().then((cacheNames) => {
      return Promise.all(
        cacheNames.map((cache) => {
          if (cache !== CACHE_NAME) {
            return caches.delete(cache);
          }
        })
      );
    }).then(() => self.clients.claim())
  );
});

// Fetch Event (Network-First with Cache Fallback)
self.addEventListener("fetch", (event) => {
  // Only process standard GET requests
  if (event.request.method !== "GET") return;

  event.respondWith(
    fetch(event.request)
      .then((response) => {
        // If successful response, check if we want to cache it or just return it
        return response;
      })
      .catch(() => {
        // If network request fails, look in cache
        return caches.match(event.request);
      })
  );
});
