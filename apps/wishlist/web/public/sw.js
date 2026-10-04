// PWA の Service Worker。戦略の判定は sw-strategy.js(classify)に任せる。
importScripts("sw-strategy.js");

const { classify, shouldCache, cacheNames } = self.wishlistSw;
const scope = self.registration.scope;
const shellUrl = new URL("./", scope).href;

self.addEventListener("install", (event) => {
  // 初回のオフライン用にシェル(HTML)を先に入れる。失敗しても install は成功させる。
  event.waitUntil(
    caches
      .open(cacheNames.shell)
      .then((cache) => cache.add(shellUrl))
      .catch(() => undefined)
      .then(() => self.skipWaiting()),
  );
});

self.addEventListener("activate", (event) => {
  const keep = Object.values(cacheNames);
  event.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.filter((k) => !keep.includes(k)).map((k) => caches.delete(k))))
      .then(() => self.clients.claim()),
  );
});

async function put(name, req, res) {
  if (!shouldCache(res)) return;
  const cache = await caches.open(name);
  await cache.put(req, res.clone());
}

async function cacheFirst(req) {
  const hit = await caches.match(req);
  if (hit) return hit;
  const res = await fetch(req);
  await put(cacheNames.images, req, res);
  return res;
}

async function networkFirstShell(req) {
  try {
    const res = await fetch(req);
    await put(cacheNames.shell, shellUrl, res);
    return res;
  } catch (err) {
    const hit = await caches.match(shellUrl);
    if (hit) return hit;
    throw err;
  }
}

async function staleWhileRevalidate(req) {
  const hit = await caches.match(req);
  const refresh = fetch(req)
    .then(async (res) => {
      await put(cacheNames.shell, req, res);
      return res;
    })
    .catch(() => undefined);
  if (hit) return hit;
  const res = await refresh;
  return res ?? Response.error();
}

self.addEventListener("fetch", (event) => {
  const req = event.request;
  const strategy = classify({ url: req.url, method: req.method, mode: req.mode }, scope);
  if (strategy === "cache-first") event.respondWith(cacheFirst(req));
  else if (strategy === "network-first-shell") event.respondWith(networkFirstShell(req));
  else if (strategy === "stale-while-revalidate") event.respondWith(staleWhileRevalidate(req));
  // network-only / ignore はブラウザの既定(ネットワーク)に任せる。
});
