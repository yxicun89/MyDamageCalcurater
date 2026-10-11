// タイプバランスの結果表示で共有する小さな部品(表のスクロール枠・見出し・名前の引き当て)。
// 文言は i18n/balance.ts の balanceScreenText から出す。

import type { ReactNode } from "react";
import { balanceErrorText, balanceScreenText } from "../i18n/ja";
import type { MasterData } from "../master/types";
import { Icon, type IconName } from "../ui/Icon";
import { PokemonIcon } from "../images/PokemonIcon";
import { TypeList } from "./balanceCells";
import type { SpeciesResolutions } from "../screens/speciesResolution";
import type { Schemas } from "./types";

/** エラーコードを日本語の文言にする(応答の message は英語の内部メッセージなので出さない。ADR-0411)。 */
export function balanceErrorMessage(code: string): string {
  return Object.hasOwn(balanceErrorText, code)
    ? balanceErrorText[code as keyof typeof balanceErrorText]
    : balanceErrorText.fallback;
}

/**
 * P4-17b(ADR-0304 A-14.2): ID→名前解決も speciesFor を通す(オンラインで master.species が空でも、
 * 検索で解決したメンバー・仮想敵の種族名が出るように)。
 */
export function findSpeciesName(
  speciesFor: SpeciesResolutions["speciesFor"],
  master: MasterData,
  pokemonId: string,
): string {
  return speciesFor(master.species, pokemonId)?.nameJa ?? pokemonId;
}

/** P4-17b(ADR-0304 A-14.2): abilitiesFor を通す(speciesKey はその特性の持ち主の種族 key)。 */
export function findAbilityName(
  abilitiesFor: SpeciesResolutions["abilitiesFor"],
  master: MasterData,
  speciesKey: string,
  abilityId: string,
): string {
  return (
    abilitiesFor(master.abilities, speciesKey).find((candidate) => candidate.id === abilityId)?.nameJa ??
    abilityId
  );
}

/**
 * 表を横スクロールできる領域で包む。防御相性の表は 18 列で、狭い画面ではページ全体が横に溢れるため、
 * 溢れは包みの中で受ける。キーボードでもスクロールできるよう tabIndex=0 の region にする。
 */
export function TableScroll({ label, children }: { readonly label: string; readonly children: ReactNode }) {
  return (
    <div
      role="region"
      aria-label={balanceScreenText.tableScrollLabel(label)}
      tabIndex={0}
      className="balance-screen__table-scroll"
    >
      {children}
    </div>
  );
}

/** 表の行見出し: ポケモンの画像かエンブレム + 名前(読み上げ名は名前だけ。画像は装飾)。 */
export function MemberHeading({
  master,
  speciesFor,
  pokemonId,
}: {
  readonly master: MasterData;
  readonly speciesFor: SpeciesResolutions["speciesFor"];
  readonly pokemonId: string;
}) {
  const species = speciesFor(master.species, pokemonId);
  return (
    <span className="balance-heading">
      <PokemonIcon speciesKey={species?.key ?? pokemonId} typeId={species?.types[0]} />
      {findSpeciesName(speciesFor, master, pokemonId)}
    </span>
  );
}

/** 集計の列見出し: 記号 + 語(語は列の名前のまま。記号は装飾)。 */
export function ColumnHeading({ icon, text }: { readonly icon: IconName; readonly text: string }) {
  return (
    <span className="balance-heading">
      <Icon name={icon} size={16} />
      {text}
    </span>
  );
}

/** 仮想敵の人数の行: 記号 + 前の語 + 大きい数字 + 「人」(つなげた文字は balanceScreenText.*MembersLabel と同じ)。 */
export function CountLine({
  icon,
  prefix,
  count,
}: {
  readonly icon: IconName;
  readonly prefix: string;
  readonly count: number;
}) {
  return (
    <p className="balance-count">
      <Icon name={icon} size={20} />
      {prefix}
      <strong className="balance-count__number">{count}</strong>
      {balanceScreenText.memberCountSuffix}
    </p>
  );
}

/** 穴の行: 「防御の穴: 」+ タイプのバッジ(無ければ「なし」の文字)。 */
export function HolesLine({
  label,
  types,
}: {
  readonly label: (list: string) => string;
  readonly types: readonly Schemas["TypeId"][];
}) {
  // label は「見出し: 一覧」の形。一覧の位置で前後に分け、一覧のところだけバッジにする。
  const marker = "\u0000";
  const [head = "", tail = ""] = label(marker).split(marker);
  return (
    <p className="balance-holes">
      {head}
      <TypeList types={types} />
      {tail}
    </p>
  );
}
