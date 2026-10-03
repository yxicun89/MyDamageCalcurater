// k3d の e2e(e2e-k3d/k3d.spec.ts)が「通信の失敗」として数えないもの(許容を狭く保つための唯一の置き場)。
//
// /images/manifest.json の 404: ポケモン画像は任意で、画像が無い既定の構成では gateway が JSON の 404 を返す。
// ADR-0807 は「manifest が 404 = 画像なし = タイプ色エンブレム」を仕様上の正常としている(ADR-0325)。
// 許容するのは「404」かつ「パスが /images/manifest.json と完全一致」だけ。ほかの 4xx/5xx、
// /images/ 配下の別パス(画像本体の 404 など)、manifest の 5xx は従来どおり失敗として数える。

const IMAGES_MANIFEST_PATH = "/images/manifest.json";

/** 応答 1 件を失敗として数えるか。 */
export function isCountedResponseFailure(status: number, pathname: string): boolean {
  if (status < 400) {
    return false;
  }
  if (status === 404 && pathname === IMAGES_MANIFEST_PATH) {
    return false;
  }
  return true;
}
