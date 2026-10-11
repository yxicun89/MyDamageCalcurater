// G-04(Web): 「自分の周り」パネル(ADR-0609)。スクロールなし・折りたたみなしで、自分の上下を見せる。
// 並びの切り出しは speedNeighborhood.ts(純粋)。ここは描画だけ。
//
// 契約(テストが見るもの):
//   - role=region・名前 speedScreenText.neighborhoodRegionLabel。表のスクロール領域(speed-viewport)の外に置く
//   - 先に動く側/後に動く側は ul(aria-label = 通常は速い側/遅い側、トリックルームは先に動く側/後に動く側)
//   - 近傍の段: data-testid="neighbor-before" / "neighbor-after"(data-speed を持つ)、
//     自分: data-testid="neighbor-self"(「自分」バッジの文字を含む)
//   - G-05(I-web-13e): 近傍の段・自分は小さなカード(.speed-neighbor__card。画像かエンブレム + 実数値の大きい数字 + 名前)。
//     自分のカードは「自分」のバッジ(アイコン付き)
//   - 端の合計行: data-testid="neighbor-total-before" / "neighbor-total-after"

import { speedScreenText } from "../i18n/ja";
import { PokemonIcon } from "../images/PokemonIcon";
import { Icon } from "../ui/Icon";
import { buildNeighborhood, type NeighborTier, type NeighborTierInput } from "./speedNeighborhood";

export interface SpeedNeighborhoodProps {
  /** 表の並びのままの tiers(通常は降順、トリックルームは昇順)。 */
  readonly tiers: readonly NeighborTierInput[];
  /** 自分の実数値。未決定なら null。 */
  readonly ownSpeed: number | null;
  readonly trickRoom: boolean;
}

/** 段の先頭のポケモン(画像かエンブレム用)。 */
function leadOf(tiers: readonly NeighborTierInput[], speed: number) {
  return tiers.find((t) => t.speed === speed)?.entries[0];
}

function CardIcon({ lead }: { readonly lead: ReturnType<typeof leadOf> }) {
  if (lead?.pokemonId === undefined) {
    return null;
  }
  return <PokemonIcon speciesKey={lead.pokemonId} typeId={lead.types?.[0]} />;
}

function TierItem({
  tier,
  testId,
  lead,
}: {
  readonly tier: NeighborTier;
  readonly testId: string;
  readonly lead: ReturnType<typeof leadOf>;
}) {
  return (
    <li className="speed-neighbor__tier speed-neighbor__card" data-testid={testId} data-speed={tier.speed}>
      <CardIcon lead={lead} />
      <span className="speed-neighbor__speed">{tier.speed}</span>
      <span className="speed-neighbor__names">
        {tier.names.join(speedScreenText.entrySeparator)}
        {tier.moreCount > 0 && ` ${speedScreenText.neighborhoodMore(tier.moreCount)}`}
      </span>
    </li>
  );
}

export function SpeedNeighborhood({ tiers, ownSpeed, trickRoom }: SpeedNeighborhoodProps) {
  const n = buildNeighborhood(tiers, ownSpeed, trickRoom);
  const t = speedScreenText;
  return (
    <section className="speed-neighbor" aria-label={t.neighborhoodRegionLabel}>
      <h3 className="speed-neighbor__heading">{t.neighborhoodHeading}</h3>
      {n === null ? (
        <p className="speed-neighbor__empty">{t.neighborhoodEmpty}</p>
      ) : (
        <>
          <p className="speed-neighbor__total" data-testid="neighbor-total-before">
            {t.neighborhoodUpArrow}{" "}
            {trickRoom ? t.neighborhoodBeforeTotal(n.beforeTotal) : t.neighborhoodFasterTotal(n.beforeTotal)}
          </p>
          {n.before.length === 0 ? (
            <p className="speed-neighbor__none">{t.neighborhoodNone}</p>
          ) : (
            <ul
              className="speed-neighbor__list"
              aria-label={trickRoom ? t.neighborhoodBeforeListLabel : t.neighborhoodFasterListLabel}
            >
              {n.before.map((tier) => (
                <TierItem
                  key={tier.speed}
                  tier={tier}
                  testId="neighbor-before"
                  lead={leadOf(tiers, tier.speed)}
                />
              ))}
            </ul>
          )}
          <div
            className="speed-neighbor__self speed-neighbor__card speed-neighbor__card--self"
            data-testid="neighbor-self"
          >
            <span className="speed-neighbor__badge speed-badge speed-badge--self ui-badge">
              <Icon name="speed" size={12} />
              {t.neighborhoodSelfBadge}
            </span>
            <span className="speed-neighbor__speed">{n.ownSpeed}</span>
            {n.tie === null ? (
              <span className="speed-neighbor__names">{t.neighborhoodBoundaryLabel}</span>
            ) : (
              <span className="speed-neighbor__names">
                {t.neighborhoodTieLabel}: {n.tie.names.join(t.entrySeparator)}
                {n.tie.moreCount > 0 && ` ${t.neighborhoodMore(n.tie.moreCount)}`}
              </span>
            )}
          </div>
          {n.after.length === 0 ? (
            <p className="speed-neighbor__none">{t.neighborhoodNone}</p>
          ) : (
            <ul
              className="speed-neighbor__list"
              aria-label={trickRoom ? t.neighborhoodAfterListLabel : t.neighborhoodSlowerListLabel}
            >
              {n.after.map((tier) => (
                <TierItem
                  key={tier.speed}
                  tier={tier}
                  testId="neighbor-after"
                  lead={leadOf(tiers, tier.speed)}
                />
              ))}
            </ul>
          )}
          <p className="speed-neighbor__total" data-testid="neighbor-total-after">
            {t.neighborhoodDownArrow}{" "}
            {trickRoom ? t.neighborhoodAfterTotal(n.afterTotal) : t.neighborhoodSlowerTotal(n.afterTotal)}
          </p>
        </>
      )}
    </section>
  );
}
