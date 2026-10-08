// インライン SVG の小さなアイコン部品(F-12、ADR-0334 §2)。外部のアイコンフォント・ライブラリ・画像ファイルを使わない。
// 線で描き、色は文字色に従う(currentColor)。色の直書きはしない。
// 名前(label)を渡さないものは装飾として支援技術から隠す。名前を渡すと role="img" + aria-label を持つ。

import type { ReactElement } from "react";

/** 24x24 の座標で描く図形(path の d)。線の太さ・丸め・色は Icon が決める。 */
const ICON_SHAPES = {
  // 計算: 電卓
  calc: [
    "M6 3h12a1 1 0 0 1 1 1v16a1 1 0 0 1-1 1H6a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1z",
    "M8 7h8",
    "M8 12h2",
    "M14 12h2",
    "M8 16h2",
    "M14 16h2",
  ],
  // 逆算: 左向きに戻る矢印
  reverse: ["M9 7 4 12l5 5", "M4 12h10a6 6 0 0 1 6 6"],
  // 素早さ: 稲妻
  speed: ["M13 2 4 14h7l-1 8 9-12h-7z"],
  // タイプバランス: 天秤
  balance: ["M12 4v16", "M7 20h10", "M5 8h14", "M5 8l-3 7a3 3 0 0 0 6 0z", "M19 8l-3 7a3 3 0 0 0 6 0z"],
  // 判定: チェックの付いた盾
  judge: ["M12 3 4 6v6c0 4.5 3.2 8 8 9 4.8-1 8-4.5 8-9V6z", "M8.5 12l2.5 2.5L16 9.5"],
  // 構築: 3 つ並んだ丸(パーティ)
  team: [
    "M12 4a3 3 0 1 0 0 6 3 3 0 0 0 0-6z",
    "M5 14a2.5 2.5 0 1 0 0 5 2.5 2.5 0 0 0 0-5z",
    "M19 14a2.5 2.5 0 1 0 0 5 2.5 2.5 0 0 0 0-5z",
    "M8 13.5l2-2",
    "M16 13.5l-2-2",
  ],
  // お気に入り: 星
  favorites: ["M12 3l2.7 5.6 6.1.9-4.4 4.3 1 6.1L12 17l-5.4 2.9 1-6.1-4.4-4.3 6.1-.9z"],
  // 調整: スライダー
  adjust: ["M4 7h9", "M17 7h3", "M15 5v4", "M4 17h3", "M11 17h9", "M9 15v4"],
  // このアプリについて: 丸の中の i(アプリ情報)
  about: ["M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18z", "M12 11v5", "M12 8h.01"],
  // 既定: 四角
  screen: ["M5 5h14v14H5z"],
  // 入れ替え: 上下の矢印
  swap: ["M7 4v14", "M3 14l4 4 4-4", "M17 20V6", "M13 10l4-4 4 4"],
  // 追加: プラス
  plus: ["M12 5v14", "M5 12h14"],
  // 削除: ごみ箱
  trash: ["M4 7h16", "M9 7V4h6v3", "M6 7l1 13h10l1-13", "M10 11v6", "M14 11v6"],
  // 編集: 鉛筆
  edit: ["M4 20l1-4L16 5a2.1 2.1 0 0 1 3 3L8 19z", "M14 7l3 3"],
  // 検索: 虫眼鏡
  search: ["M11 4a7 7 0 1 0 0 14 7 7 0 0 0 0-14z", "M16 16l5 5"],
  // 注意: 三角に !
  alert: ["M12 3 2 20h20z", "M12 10v4", "M12 17h.01"],
  // 情報: 吹き出し
  info: ["M4 5h16v11H9l-5 4z", "M12 9v3", "M12 14h.01"],
  // 完了: チェック
  check: ["M5 12.5l4.5 4.5L19 7"],
} as const satisfies Record<string, readonly string[]>;

export type IconName = keyof typeof ICON_SHAPES;

export const ICON_NAMES = Object.keys(ICON_SHAPES) as readonly IconName[];

export interface IconProps {
  readonly name: IconName;
  /** 意味を持つアイコンの名前。省くと装飾(aria-hidden)。 */
  readonly label?: string;
  readonly size?: number;
  readonly className?: string;
}

export function Icon({ name, label, size = 20, className }: IconProps): ReactElement {
  const accessibility =
    label === undefined
      ? ({ "aria-hidden": "true", focusable: "false" } as const)
      : ({ role: "img", "aria-label": label } as const);
  return (
    <svg
      {...accessibility}
      className={className === undefined ? "ui-icon" : `ui-icon ${className}`}
      viewBox="0 0 24 24"
      width={size}
      height={size}
      fill="none"
      stroke="currentColor"
      strokeWidth={2}
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      {ICON_SHAPES[name].map((d) => (
        <path key={d} d={d} />
      ))}
    </svg>
  );
}
