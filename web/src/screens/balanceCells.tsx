// タイプバランス画面の結果の升目の部品(I-web-13f。G-05。ADR-0352)。
// 升目は色だけで伝えず、記号(アイコン)と文字の両方を出す。記号は aria-hidden で、読み上げ・textContent は文字だけ
// (既存のテストと e2e が文字を「×2 弱点」のとおりに見る)。どの語・倍率になるかは応答の値で決め、ここは見た目だけを引く。

import { Fragment } from "react";
import type { ReactElement } from "react";
import type { components } from "../api/balance.gen";
import { balanceScreenText } from "../i18n/ja";
import { Icon, type IconName } from "../ui/Icon";
import { TypeBadge } from "../ui/TypeBadge";

type Schemas = components["schemas"];

/** 升目の色分け。背景は *-soft、記号は状態色、文字は text.primary(popPalette.test.ts で検査済みの組)。 */
export type CellTone = "good" | "caution" | "bad" | "info" | "resist" | "immune" | "plain";

interface CellLook {
  readonly tone: CellTone;
  readonly icon: IconName;
}

/** 防御の分類 → 見た目。弱点=上向き(赤)、耐性=下向き(水色系)、無効=×(灰)、等倍=横線(ADR-0422 §3。緑は相性に使わない)。 */
export function defenseLook(category: Schemas["DefenseCategory"]): CellLook {
  switch (category) {
    case "quad_weak":
    case "weak":
      return { tone: "bad", icon: "up" };
    case "resist":
    case "quad_resist":
      return { tone: "resist", icon: "down" };
    case "immune":
      return { tone: "immune", icon: "close" };
    case "neutral":
      return { tone: "plain", icon: "minus" };
  }
}

/** 攻撃範囲の最大倍率 → 見た目。抜群=上向き(赤)、いまひとつ=下向き(水色系)、無効=×(灰)、等倍・攻撃技なし=横線(ADR-0422 §3)。 */
export function coverageLook(multiplier: Schemas["CoverageMultiplier"]): CellLook {
  switch (multiplier) {
    case "2":
      return { tone: "bad", icon: "up" };
    case "1/2":
      return { tone: "resist", icon: "down" };
    case "0":
      return { tone: "immune", icon: "close" };
    case "1":
    case null:
      return { tone: "plain", icon: "minus" };
  }
}

/** 仮想敵の「安全に受けられるか」。安全=チェック(緑系)、注意=三角(黄系)。 */
export function safeLook(safe: boolean): CellLook {
  return safe ? { tone: "good", icon: "check" } : { tone: "caution", icon: "alert" };
}

/** 仮想敵への「抜群が取れるか」。取れる=上向き(緑系)、取れない=横線。 */
export function superEffectiveLook(superEffective: boolean): CellLook {
  return superEffective ? { tone: "good", icon: "up" } : { tone: "plain", icon: "minus" };
}

/** 記号 + 文字の升目(td)。textContent は文字だけ。 */
export function ResultCell({ look, text }: { readonly look: CellLook; readonly text: string }): ReactElement {
  return (
    <td className={`balance-cell balance-cell--${look.tone}`}>
      <span className="balance-cell__inner">
        <Icon name={look.icon} size={16} />
        <span className="balance-cell__text">{text}</span>
      </span>
    </td>
  );
}

/** タイプの一覧をバッジで並べる(区切りの「・」は文字として残す。空なら「なし」)。 */
export function TypeList({ types }: { readonly types: readonly Schemas["TypeId"][] }): ReactElement {
  if (types.length === 0) {
    return <>{balanceScreenText.noneLabel}</>;
  }
  return (
    <span className="balance-types">
      {types.map((type, index) => (
        <Fragment key={type}>
          {index > 0 && <span className="balance-types__separator">{balanceScreenText.listSeparator}</span>}
          <TypeBadge type={type} />
        </Fragment>
      ))}
    </span>
  );
}
