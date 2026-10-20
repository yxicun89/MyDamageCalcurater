// 防御相性の表・チームの集計・攻撃範囲の表(ADR-0303 §7。倍率・人数は応答の値をそのまま出す)。

import { coverageMultiplierLabel, defenseMultiplierLabel } from "../domain/balanceLabels";
import { balanceScreenText } from "../i18n/ja";
import type { MasterData } from "../master/types";
import { TypeBadge } from "../ui/TypeBadge";
import { coverageLook, defenseLook, ResultCell } from "./balanceCells";
import type { SpeciesResolutions } from "../ui/speciesResolution";
import { ColumnHeading, MemberHeading, TableScroll } from "./balanceParts";
import type { Schemas } from "./types";

export interface DefenseTableProps {
  readonly master: MasterData;
  readonly speciesFor: SpeciesResolutions["speciesFor"];
  readonly response: Schemas["AnalyzeResponse"];
}

/** 防御相性の表(メンバー × 18タイプ)。列の並びは応答の順に従う(タイプの順をコードに持たない)。 */
export function DefenseTable({ master, speciesFor, response }: DefenseTableProps) {
  const headerTypes = response.teamSummary.map((entry) => entry.attackType);
  return (
    <TableScroll label={balanceScreenText.defenseTableLabel}>
      <table className="ui-table" aria-label={balanceScreenText.defenseTableLabel}>
        <thead>
          <tr>
            <th scope="col">{balanceScreenText.memberColumnLabel}</th>
            {headerTypes.map((type) => (
              <th key={type} scope="col">
                <TypeBadge type={type} />
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {response.members.map((member, index) => (
            <tr key={`${member.pokemonId}-${String(index)}`}>
              <th scope="row">
                <MemberHeading master={master} speciesFor={speciesFor} pokemonId={member.pokemonId} />
              </th>
              {headerTypes.map((type) => {
                const entry = member.defense.find((candidate) => candidate.attackType === type);
                return entry === undefined ? (
                  <td key={type} />
                ) : (
                  <ResultCell
                    key={type}
                    look={defenseLook(entry.category)}
                    text={defenseMultiplierLabel(entry.multiplier, entry.category)}
                  />
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>
    </TableScroll>
  );
}

/** チームの集計(攻撃タイプごとの 弱点・うち×4・耐性・無効・等倍 の人数)。 */
export function TeamSummaryTable({ response }: { readonly response: Schemas["AnalyzeResponse"] }) {
  return (
    <TableScroll label={balanceScreenText.teamSummaryTableLabel}>
      <table className="ui-table" aria-label={balanceScreenText.teamSummaryTableLabel}>
        <thead>
          <tr>
            <th scope="col">{balanceScreenText.attackTypeColumnLabel}</th>
            <th scope="col">
              <ColumnHeading icon="up" text={balanceScreenText.weakColumnLabel} />
            </th>
            <th scope="col">{balanceScreenText.quadWeakColumnLabel}</th>
            <th scope="col">
              <ColumnHeading icon="down" text={balanceScreenText.resistColumnLabel} />
            </th>
            <th scope="col">
              <ColumnHeading icon="close" text={balanceScreenText.immuneColumnLabel} />
            </th>
            <th scope="col">
              <ColumnHeading icon="minus" text={balanceScreenText.neutralColumnLabel} />
            </th>
          </tr>
        </thead>
        <tbody>
          {response.teamSummary.map((entry) => (
            <tr key={entry.attackType}>
              <th scope="row" style={{ borderLeftColor: `var(--type-${entry.attackType})` }}>
                <TypeBadge type={entry.attackType} />
              </th>
              <td className="balance-count-cell">{entry.weak}</td>
              <td className="balance-count-cell">{entry.quadWeak}</td>
              <td className="balance-count-cell">{entry.resist}</td>
              <td className="balance-count-cell">{entry.immune}</td>
              <td className="balance-count-cell">{entry.neutral}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </TableScroll>
  );
}

/** 攻撃範囲(防御タイプごとの 最大倍率・有効・抜群 の人数)。 */
export function CoverageTable({ response }: { readonly response: Schemas["CoverageResponse"] }) {
  return (
    <TableScroll label={balanceScreenText.coverageTableLabel}>
      <table className="ui-table" aria-label={balanceScreenText.coverageTableLabel}>
        <thead>
          <tr>
            <th scope="col">{balanceScreenText.defenseTypeColumnLabel}</th>
            <th scope="col">{balanceScreenText.bestMultiplierColumnLabel}</th>
            <th scope="col">{balanceScreenText.effectiveColumnLabel}</th>
            <th scope="col">{balanceScreenText.superEffectiveColumnLabel}</th>
          </tr>
        </thead>
        <tbody>
          {response.teamCoverage.map((entry) => (
            <tr key={entry.defenseType}>
              <th scope="row" style={{ borderLeftColor: `var(--type-${entry.defenseType})` }}>
                <TypeBadge type={entry.defenseType} />
              </th>
              <ResultCell
                look={coverageLook(entry.bestMultiplier)}
                text={coverageMultiplierLabel(entry.bestMultiplier)}
              />
              <td className="balance-count-cell">{entry.effectiveMembers}</td>
              <td className="balance-count-cell">{entry.superEffectiveMembers}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </TableScroll>
  );
}
