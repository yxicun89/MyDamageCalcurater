// 仮想敵(threats)・おすすめタイプ(recommendations)の表示(ADR-0400・ADR-0401)。

import {
  matchupMultiplierLabel,
  multiplierLabel,
  safeLabel,
  superEffectiveLabel,
} from "../domain/balanceLabels";
import { balanceScreenText } from "../i18n/ja";
import type { MasterData } from "../master/types";
import { PokemonCard } from "../ui/PokemonCard";
import { TypeBadge } from "../ui/TypeBadge";
import { ResultCell, safeLook, superEffectiveLook, TypeList } from "../screens/balanceCells";
import type { SpeciesResolutions } from "../screens/speciesResolution";
import {
  CountLine,
  findAbilityName,
  findSpeciesName,
  HolesLine,
  MemberHeading,
  TableScroll,
} from "./balanceParts";
import type { Schemas } from "./types";

interface ThreatSectionProps {
  readonly n: number;
  readonly master: MasterData;
  readonly speciesFor: SpeciesResolutions["speciesFor"];
  readonly threat: Schemas["ThreatResult"];
}

/**
 * 仮想敵1体分の結果(region)。行=自分のメンバー、列=受ける倍率・与える倍率・安全・抜群(ADR-0400 §2)。
 * safe/superEffective は応答の真偽値をそのまま語にする(倍率から判定し直さない)。
 */
export function ThreatSection({ n, master, speciesFor, threat }: ThreatSectionProps) {
  const species = speciesFor(master.species, threat.pokemonId);
  return (
    <section
      className="ui-card balance-screen__threat"
      aria-label={balanceScreenText.threatRegionLabel(
        n,
        findSpeciesName(speciesFor, master, threat.pokemonId),
      )}
    >
      <PokemonCard
        small
        className="balance-screen__threat-card"
        speciesKey={species?.key ?? threat.pokemonId}
        name={findSpeciesName(speciesFor, master, threat.pokemonId)}
        types={species?.types ?? []}
      />
      <TableScroll label={balanceScreenText.threatMatchupTableLabel}>
        <table className="ui-table" aria-label={balanceScreenText.threatMatchupTableLabel}>
          <thead>
            <tr>
              <th scope="col">{balanceScreenText.memberColumnLabel}</th>
              <th scope="col">{balanceScreenText.incomingColumnLabel}</th>
              <th scope="col">{balanceScreenText.outgoingColumnLabel}</th>
              <th scope="col">{balanceScreenText.safeColumnLabel}</th>
              <th scope="col">{balanceScreenText.superEffectiveColumnLabel}</th>
            </tr>
          </thead>
          <tbody>
            {threat.matchups.map((matchup, index) => (
              <tr key={`${matchup.pokemonId}-${String(index)}`}>
                <th scope="row">
                  <MemberHeading master={master} speciesFor={speciesFor} pokemonId={matchup.pokemonId} />
                </th>
                <td>{matchupMultiplierLabel(matchup.incoming)}</td>
                <td>{matchupMultiplierLabel(matchup.outgoing)}</td>
                <ResultCell look={safeLook(matchup.safe)} text={safeLabel(matchup.safe)} />
                <ResultCell
                  look={superEffectiveLook(matchup.superEffective)}
                  text={superEffectiveLabel(matchup.superEffective)}
                />
              </tr>
            ))}
          </tbody>
        </table>
      </TableScroll>
      <div className="balance-screen__counts">
        <CountLine icon="check" prefix={balanceScreenText.safeMembersPrefix} count={threat.safeMembers} />
        <CountLine
          icon="up"
          prefix={balanceScreenText.superEffectiveMembersPrefix}
          count={threat.superEffectiveMembers}
        />
      </div>
    </section>
  );
}

interface RecommendationsSectionProps {
  readonly master: MasterData;
  readonly abilitiesFor: SpeciesResolutions["abilitiesFor"];
  readonly response: Schemas["RecommendationsResponse"];
}

/** おすすめタイプの結果(region)。防御・攻撃範囲の穴、候補の表、特性で補える表を出す(ADR-0401)。 */
export function RecommendationsSection({ master, abilitiesFor, response }: RecommendationsSectionProps) {
  return (
    <section
      className="ui-card balance-screen__recommendations"
      aria-label={balanceScreenText.recommendationsRegionLabel}
    >
      <HolesLine label={balanceScreenText.defenseHolesLabel} types={response.defenseHoles} />
      <HolesLine label={balanceScreenText.offenseHolesLabel} types={response.offenseHoles} />
      <CandidatesTable candidates={response.candidates} />
      <AbilityOptionsTable
        master={master}
        abilitiesFor={abilitiesFor}
        abilityOptions={response.abilityOptions}
      />
    </section>
  );
}

/** おすすめタイプの候補の表(タイプ・ふさぐ防御の穴・ふさぐ攻撃範囲の穴・該当ポケモン。応答の順)。 */
function CandidatesTable({ candidates }: { readonly candidates: readonly Schemas["TypeCandidate"][] }) {
  return (
    <TableScroll label={balanceScreenText.candidatesTableLabel}>
      <table className="ui-table" aria-label={balanceScreenText.candidatesTableLabel}>
        <thead>
          <tr>
            <th scope="col">{balanceScreenText.typesColumnLabel}</th>
            <th scope="col">{balanceScreenText.defenseCoveredColumnLabel}</th>
            <th scope="col">{balanceScreenText.offenseCoveredColumnLabel}</th>
            <th scope="col">{balanceScreenText.pokemonColumnLabel}</th>
          </tr>
        </thead>
        <tbody>
          {candidates.map((candidate, index) => {
            const pokemonText =
              candidate.pokemon.length === 0
                ? balanceScreenText.noneLabel
                : candidate.pokemon
                    .map((pokemon) => pokemon.nameJa ?? pokemon.pokemonId)
                    .join(balanceScreenText.listSeparator);
            return (
              <tr key={`${candidate.types.join("-")}-${String(index)}`}>
                <th scope="row">
                  <TypeList types={candidate.types} />
                </th>
                <td>
                  <TypeList types={candidate.defenseCovered} />
                </td>
                <td>
                  <TypeList types={candidate.offenseCovered} />
                </td>
                <td>{pokemonText}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </TableScroll>
  );
}

interface AbilityOptionsTableProps {
  readonly master: MasterData;
  readonly abilitiesFor: SpeciesResolutions["abilitiesFor"];
  readonly abilityOptions: readonly Schemas["AbilityOption"][];
}

/** 特性で補えるポケモンの表(防御の穴ごとに 攻撃タイプ・該当ポケモン。ADR-0401 §4)。 */
function AbilityOptionsTable({ master, abilitiesFor, abilityOptions }: AbilityOptionsTableProps) {
  return (
    <TableScroll label={balanceScreenText.abilityOptionsTableLabel}>
      <table className="ui-table" aria-label={balanceScreenText.abilityOptionsTableLabel}>
        <thead>
          <tr>
            <th scope="col">{balanceScreenText.attackTypeColumnLabel}</th>
            <th scope="col">{balanceScreenText.pokemonColumnLabel}</th>
          </tr>
        </thead>
        <tbody>
          {abilityOptions.map((option) => {
            const pokemonText =
              option.pokemon.length === 0
                ? balanceScreenText.noneLabel
                : option.pokemon
                    .map((pokemon) =>
                      balanceScreenText.abilityOptionEntryLabel(
                        pokemon.nameJa ?? pokemon.pokemonId,
                        findAbilityName(abilitiesFor, master, pokemon.pokemonId, pokemon.abilityId),
                        multiplierLabel(pokemon.multiplier),
                      ),
                    )
                    .join(balanceScreenText.listSeparator);
            return (
              <tr key={option.attackType}>
                <th scope="row">
                  <TypeBadge type={option.attackType} />
                </th>
                <td>{pokemonText}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </TableScroll>
  );
}
