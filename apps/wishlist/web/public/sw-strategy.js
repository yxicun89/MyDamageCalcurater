// Service Worker の戦略(どのリクエストをどう扱うか)。classic script。sw.js が importScripts で読み込み、
// テストは vm でこのファイルを読む。self.wishlistSw に classify / shouldCache / cacheNames を定義する。
(function () {
  "use strict";

  var VERSION = "v1";
  var cacheNames = { shell: "wishlist-shell-" + VERSION, images: "wishlist-images-" + VERSION };

  // テストの vm には URL が無いので、origin と pathname は正規表現で取り出す(Service Worker の req.url は絶対 URL)。
  function parse(u) {
    var m = /^([a-z][a-z0-9+.-]*:\/\/[^/?#]*)([^?#]*)/i.exec(u);
    return { origin: m ? m[1].toLowerCase() : "", pathname: m ? m[2] || "/" : "" };
  }

  // 戻り値: ignore / network-only / cache-first / network-first-shell / stale-while-revalidate
  function classify(req, scopeUrl) {
    if (req.method !== "GET") return "ignore";
    var url = parse(req.url);
    var scope = parse(scopeUrl);
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
