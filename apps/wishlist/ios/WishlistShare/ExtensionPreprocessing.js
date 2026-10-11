// Safari の共有シートで開いたページから、url・title・imageURL を読んで共有拡張(Swift)へ渡す。
// NSExtensionJavaScriptPreprocessingFile = ExtensionPreprocessing（WishlistShare-Info.plist）。
// サーバーはページを取りに行かない（駿河屋のようにサーバーから読めないページでも登録できる）ので、ここで読んだ値が下書きのもとになる。

var ExtensionPreprocessingJS = {
  run: function (arguments) {
    arguments.completionFunction({
      url: document.URL,
      title: ExtensionPreprocessingJS.title(),
      imageURL: ExtensionPreprocessingJS.imageURL(),
    });
  },

  // 受け取った値の後始末は要らない。
  finalize: function (arguments) {},

  meta: function (property) {
    var element =
      document.querySelector('meta[property="' + property + '"]') ||
      document.querySelector('meta[name="' + property + '"]');
    return element ? (element.getAttribute("content") || "").trim() : "";
  },

  // og:title。無ければ document.title。
  title: function () {
    return ExtensionPreprocessingJS.meta("og:title") || (document.title || "").trim();
  },

  // og:image（無ければ twitter:image）を絶対 URL にしたもの。
  // 無ければ、ページで描画サイズ（naturalWidth × naturalHeight）がいちばん大きい img の currentSrc || src。それも無ければ空。
  imageURL: function () {
    var declared =
      ExtensionPreprocessingJS.meta("og:image") || ExtensionPreprocessingJS.meta("twitter:image");
    if (declared) {
      try {
        return new URL(declared, document.baseURI).href;
      } catch (e) {
        // 壊れた値は読み飛ばして img を探す
      }
    }
    var best = "";
    var bestArea = 0;
    var images = document.getElementsByTagName("img");
    for (var i = 0; i < images.length; i++) {
      var area = (images[i].naturalWidth || 0) * (images[i].naturalHeight || 0);
      var source = images[i].currentSrc || images[i].src;
      if (source && area > bestArea) {
        best = source;
        bestArea = area;
      }
    }
    return best;
  },
};
