import type { CSSProperties } from "react";
import type { Genre } from "../api/types";

/** ジャンルの絵柄(24x24 の線画)。docs/phase5-spec.md「5-2 デザインの決まり」。 */
const GENRE_PATHS = {
  all: "M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3zM14 14h7v7h-7z",
  card: "M5 2h14v20H5zM12 7l1.5 3 3.3.5-2.4 2.3.6 3.2L12 14.5 9 16l.6-3.2-2.4-2.3 3.3-.5z",
  figure: "M12 2a3 3 0 1 1 0 6 3 3 0 0 1 0-6zM12 8v7M7 11l5-1 5 1M9 22l3-7 3 7",
  robot: "M6 4h12v9H6zM9 8h.01M15 8h.01M12 1v3M8 13v4M16 13v4M5 21h5M14 21h5",
  goods: "M12 3a9 9 0 1 1 0 18 9 9 0 0 1 0-18zM3 12h6M15 12h6",
  star: "M12 2l3 7 7 .6-5.3 4.7 1.7 7.2L12 17.8 5.6 21.5l1.7-7.2L2 9.6 9 9z",
  box: "M3 7l9-4 9 4v10l-9 4-9-4zM3 7l9 4 9-4M12 11v10",
} as const;

export type GenreGlyphName = keyof typeof GENRE_PATHS;
export type GenreIconName = Genre["icon"];

/** 絵柄を選ぶ場面(ジャンル編集)の aria-label。選べるのは「すべて」を除く 6 つ。 */
export const GENRE_ICONS: readonly { icon: GenreIconName; label: string }[] = [
  { icon: "card", label: "カード" },
  { icon: "figure", label: "フィギュア" },
  { icon: "robot", label: "ロボ" },
  { icon: "goods", label: "グッズ" },
  { icon: "star", label: "星" },
  { icon: "box", label: "箱" },
];

const svgProps = {
  viewBox: "0 0 24 24",
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 2,
  strokeLinecap: "round",
  strokeLinejoin: "round",
  "aria-hidden": true,
  focusable: false,
} as const;

/** ジャンルの絵柄。`data-icon` に絵柄の名前を持つ。 */
export function GenreGlyph({ icon, size = 22 }: { icon: GenreGlyphName; size?: number }) {
  return (
    <svg {...svgProps} width={size} height={size} data-icon={icon}>
      <path d={GENRE_PATHS[icon]} />
    </svg>
  );
}

/** 画面の部品に使う小さな線画。 */
const UI_PATHS = {
  plus: "M12 5v14M5 12h14",
  gear: "M12 9a3 3 0 1 1 0 6 3 3 0 0 1 0-6zM12 2v3M12 19v3M2 12h3M19 12h3M4.9 4.9l2.1 2.1M17 17l2.1 2.1M4.9 19.1L7 17M17 7l2.1-2.1",
  close: "M6 6l12 12M18 6L6 18",
  back: "M15 5l-7 7 7 7",
  clock: "M12 3a9 9 0 1 1 0 18 9 9 0 0 1 0-18zM12 7v5l3 2",
  search: "M11 4a7 7 0 1 1 0 14 7 7 0 0 1 0-14zM21 21l-5-5M8 11h6",
  warn: "M12 3l10 18H2zM12 10v5M12 18h.01",
  offline: "M2 2l20 20M7 18a5 5 0 0 1-.6-9.9M10 6.2A6 6 0 0 1 17.7 9 4.5 4.5 0 0 1 20 13M9 18h7",
  clipboard: "M8 4h8v3H8zM6 5h2M16 5h2M6 5v16h12V5",
  link: "M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1",
  camera: "M3 8h4l2-3h6l2 3h4v12H3zM12 10a4 4 0 1 1 0 8 4 4 0 0 1 0-8z",
  check: "M5 12l5 5 9-10",
  edit: "M4 20h4L19 9l-4-4L4 16zM13 7l4 4",
  external: "M14 4h6v6M20 4l-9 9M18 14v6H4V6h6",
  trash: "M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13",
  up: "M12 19V5M6 11l6-6 6 6",
  image: "M3 4h18v16H3zM3 16l5-5 4 4 3-3 6 6M9 9h.01",
  tag: "M3 3h9l9 9-9 9-9-9zM8 8h.01",
  down: "M6 9l6 6 6-6",
  refresh: "M20 11a8 8 0 0 0-14-4M4 4v4h4M4 13a8 8 0 0 0 14 4M20 20v-4h-4",
} as const;

export type UiIconName = keyof typeof UI_PATHS;

export function UiIcon({
  name,
  size = 20,
  strokeWidth = 2,
}: {
  name: UiIconName;
  size?: number;
  strokeWidth?: number;
}) {
  return (
    <svg {...svgProps} strokeWidth={strokeWidth} width={size} height={size}>
      <path d={UI_PATHS[name]} />
    </svg>
  );
}

/** ジャンルの色を子孫の CSS から引けるようにする(`--genre-color`)。同じ値を `data-genre-color` にも置くのは呼び出し側。 */
export const genreColorStyle = (color: string): CSSProperties =>
  ({ "--genre-color": color }) as CSSProperties;

/** 12 色の見本(ジャンル編集)。 */
export const COLOR_SWATCHES: readonly { color: string; label: string }[] = [
  { color: "#e0393e", label: "赤" },
  { color: "#f0642a", label: "朱" },
  { color: "#f59e0b", label: "橙" },
  { color: "#f2b705", label: "黄" },
  { color: "#7cb518", label: "黄緑" },
  { color: "#16a34a", label: "緑" },
  { color: "#0d9488", label: "青緑" },
  { color: "#0ea5e9", label: "水色" },
  { color: "#1f6fe5", label: "青" },
  { color: "#4f46e5", label: "藍" },
  { color: "#7c4dff", label: "紫" },
  { color: "#e04f9a", label: "桃" },
];

/** 新規ジャンルの初期色・ジャンルが見つからないときの色(強調色) */
export const DEFAULT_GENRE_COLOR = "#1d5fd1";

/** 商品のジャンルの色(ジャンルが見つからなければ強調色) */
export const colorOf = (genres: readonly Genre[], genreId: number): string =>
  genres.find((g) => g.id === genreId)?.color ?? DEFAULT_GENRE_COLOR;

/** 見分けやすい枠・ボタンの共通属性(色の値の持ち方を 1 か所に) */
export const genreColorProps = (color: string) => ({
  "data-genre-color": color,
  style: genreColorStyle(color),
});
