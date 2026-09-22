// テスト専用: テストファイルからの相対パスをファイルシステムの絶対パスにする。
// jsdom 環境では URL が jsdom の実装に置き換わり、node:fs に URL オブジェクトを直接渡せないため、
// href(文字列)を経由してパスに変換する。

import { fileURLToPath } from "node:url";

/**
 * テストファイルからの相対パス(`relative`、`import.meta.url` を `base` に渡す)をファイルシステムの
 * 絶対パスにする。node:fs の readFileSync 等にそのまま渡せる文字列を返す。
 */
export function localPath(relative: string, base: string): string {
  return fileURLToPath(new URL(relative, base).href);
}
