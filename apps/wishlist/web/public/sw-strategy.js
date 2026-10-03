// Service Worker の戦略(どのリクエストをどう扱うか)。classic script。sw.js が importScripts で読み込み、
// テストは vm でこのファイルを読む。self.wishlistSw に classify / shouldCache / cacheNames を定義する。
(function () {
  "use strict";

  var VERSION = "v1";
  var cacheNames = { shell: "wishlist-shell-" + VERSION, images: "wishlist-images-" + VERSION };

  // 戻り値: ignore / network-only / cache-first / network-first-shell / stale-while-revalidate
  function classify(req, scopeUrl) {
    if (req.method !== "GET") return "ignore";
    var url = new URL(req.url);
    var scope = new URL(scopeUrl);
    if (url.origin !== scope.origin) return "ignore";
    if (url.pathname.indexOf(scope.pathname) !== 0) return "ignore";
    var rel = url.pathname.slice(scope.pathname.length);
    // API は個人データ。キャッシュしない(オフライン時はアプリ側の localStorage が受け持つ)。
    if (rel === "api" || rel.indexOf("api/") === 0) return "network-only";
    if (rel === "healthz" || rel === "sw.js") return "network-only";
    if (rel.indexOf("images/") === 0) return "cache-first";
    if (req.mode === "navigate") return "network-first-shell";
    return "stale-while-revalidate";
  }

  // 200 の同一オリジン応答だけ保存する(エラー応答・opaque を残さない)。
  function shouldCache(res) {
    return res.status === 200 && res.type === "basic";
  }

  self.wishlistSw = { classify: classify, shouldCache: shouldCache, cacheNames: cacheNames };
})();
