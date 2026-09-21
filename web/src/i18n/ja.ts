// 画面の文言資源(日本語)。タイプの表示名はここに ID → 表示名で持つ(ADR-0016 §4)。
// タイプの一覧(どの ID が存在するか)は相性表のデータ(testdata/golden/typechart.json の types)が正であり、
// ここでは全 ID を過不足なく覆う対応表だけを持つ(コーディング規約 §2: マスタをコードに埋め込まない)。
// TypeId のユニオンは手書きの複製だが、相性表と過不足なく一致することを ja.test.ts が検査して同期を保つ
// (コーディング規約 §2 の「独立した検証」)。

/** 相性表が持つ18タイプの ID(`testdata/golden/typechart.json` の `types` と同じ)。 */
export type TypeId =
  | "bug"
  | "dark"
  | "dragon"
  | "electric"
  | "fairy"
  | "fighting"
  | "fire"
  | "flying"
  | "ghost"
  | "grass"
  | "ground"
  | "ice"
  | "normal"
  | "poison"
  | "psychic"
  | "rock"
  | "steel"
  | "water";

/** タイプ ID → 日本語の表示名(docs/design.md 「タイプ色(自作パレット)」の表と同じ呼び名)。 */
export const typeNameJa: Record<TypeId, string> = {
  normal: "ノーマル",
  fire: "ほのお",
  water: "みず",
  electric: "でんき",
  grass: "くさ",
  ice: "こおり",
  fighting: "かくとう",
  poison: "どく",
  ground: "じめん",
  flying: "ひこう",
  psychic: "エスパー",
  bug: "むし",
  rock: "いわ",
  ghost: "ゴースト",
  dragon: "ドラゴン",
  dark: "あく",
  steel: "はがね",
  fairy: "フェアリー",
};

/**
 * 値がタイプ ID(TypeId)かどうかの型ガード。マスタ由来の文字列(種族の types など)を
 * typeNameJa に渡す前に安全性を確かめるために使い、`as TypeId` の型アサーションを避ける
 * (コーディング規約 §4 TypeScript「非 null 断言・キャストは原則禁止」と同じ精神)。
 */
export function isTypeId(value: string): value is TypeId {
  return Object.hasOwn(typeNameJa, value);
}

/**
 * 計算画面(P4-2)の文言。コーディング規約 §2「UI の文言は文言資源に置く」に従い、
 * 画面・書式のコードはここの語だけを組み合わせ、日本語の文字列リテラルを直接持たない。
 */
export const calcScreenText = {
  attackerPokemonLabel: "攻撃側のポケモン",
  defenderPokemonLabel: "防御側のポケモン",
  attackerItemLabel: "攻撃側の持ち物",
  defenderItemLabel: "防御側の持ち物",
  moveLabel: "技",
  attackerRegionLabel: "攻撃側",
  defenderRegionLabel: "防御側",
  noItemOption: "なし",
  noItemRowLabel: "持ち物なし",
  resultsListLabel: "計算結果",
  compareItemCandidatesLabel: "持ち物の候補も比較",
  swapButtonLabel: "攻守入れ替え",
  statusMoveNotice: "変化技はダメージを計算しません",
  /** 技セレクタの各行の区切り(「技名・分類・威力n」)。 */
  moveOptionSeparator: "・",
  /** 技セレクタの威力の前置き(「威力80」)。 */
  movePowerLabel: "威力",
  /** 入力が揃い calcBulk の応答待ちのときに出す文言(古い行を出さず、これに差し替える)。 */
  loadingNotice: "計算中",
} as const;

/** アプリ全体(App.tsx)の文言。 */
export const appText = {
  title: "ポケモン ダメージ計算",
  loading: "読み込み中…",
  masterLoadError: "マスタデータの読み込みに失敗しました",
} as const;

/** 計算結果の書式(domain/format.ts)で使う語。 */
export const resultText = {
  determinedPrefix: "確定",
  randomPrefix: "乱数",
  hitsSuffix: "発",
  cannotKO: "倒せない",
  effectivenessNone: "効果なし",
  effectivenessNotVery: "効果はいまひとつ",
  effectivenessNeutral: "等倍",
  effectivenessSuper: "効果はばつぐん",
  moveCategory: { physical: "物理", special: "特殊", status: "変化" },
} as const;
