// 仮想敵(threats)・おすすめタイプ(recommendations)の表示(ADR-0400・ADR-0401)。

import {
  matchupMultiplierLabel,
  multiplierLabel,
  safeLabel,
  superEffectiveLabel,
} from "../domain/balanceLabels";
import { useState } from "react";
import { balanceScreenText, typeNameJa } from "../i18n/ja";
import type { MasterData } from "../master/types";
import { PokemonIcon } from "../images/PokemonIcon";
import { PokemonCard } from "../ui/PokemonCard";
import { Tile } from "../ui/Tile";
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
  readonly speciesFor: SpeciesResolutions["speciesFor"];
  readonly response: Schemas["RecommendationsResponse"];
  readonly onPickPokemon?: (speciesKey: string) => void;
  readonly canPick: boolean;
}

/** おすすめタイプの結果(region)。防御・攻撃範囲の穴、候補の表、特性で補える表を出す(ADR-0401)。 */
export function RecommendationsSection({
  master,
  abilitiesFor,
  speciesFor,
  response,
  onPickPokemon,
  canPick,
}: RecommendationsSectionProps) {
  return (
    <section
      className="ui-card balance-screen__recommendations"
      aria-label={balanceScreenText.recommendationsRegionLabel}
    >
      <HolesLine label={balanceScreenText.defenseHolesLabel} types={response.defenseHoles} />
      <HolesLine label={balanceScreenText.offenseHolesLabel} types={response.offenseHoles} />
      <CandidatesTable
        candidates={response.candidates}
        master={master}
        speciesFor={speciesFor}
        onPickPokemon={onPickPokemon}
        canPick={canPick}
      />
      <AbilityOptionsTable
        master={master}
        abilitiesFor={abilitiesFor}
        abilityOptions={response.abilityOptions}
      />
    </section>
  );
}

/** おすすめタイプの候補の表(タイプ・ふさぐ防御の穴・ふさぐ攻撃範囲の穴・該当ポケモン。応答の順)。 */
function CandidatesTable({
  candidates,
  master,
  speciesFor,
  onPickPokemon,
  canPick,
}: {
  readonly candidates: readonly Schemas["TypeCandidate"][];
  readonly master: MasterData;
  readonly speciesFor: SpeciesResolutions["speciesFor"];
  readonly onPickPokemon?: (speciesKey: string) => void;
  readonly canPick: boolean;
}) {
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
                <td>
                  {onPickPokemon === undefined ? (
                    pokemonText
                  ) : (
                    <CandidatePokemonTiles
                      candidate={candidate}
                      master={master}
                      speciesFor={speciesFor}
                      onPick={onPickPokemon}
                      canPick={canPick}
                    />
                  )}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </TableScroll>
  );
}

/** 1 候補で最初に見せるタイルの数(残りは「もっと見る」。ADR-0422 §2)。 */
export const PICK_TILES_VISIBLE = 8;

/** 候補の該当ポケモンのタイル(画像 + 名前。押すと構築の最初の空き枠に入る。満杯なら押せない)。 */
function CandidatePokemonTiles({
  candidate,
  master,
  speciesFor,
  onPick,
  canPick,
}: {
  readonly candidate: Schemas["TypeCandidate"];
  readonly master: MasterData;
  readonly speciesFor: SpeciesResolutions["speciesFor"];
  readonly onPick: (speciesKey: string) => void;
  readonly canPick: boolean;
}) {
  const [expanded, setExpanded] = useState(false);
  if (candidate.pokemon.length === 0) {
    return <>{balanceScreenText.noneLabel}</>;
  }
  const shown = expanded ? candidate.pokemon : candidate.pokemon.slice(0, PICK_TILES_VISIBLE);
  const hiddenCount = candidate.pokemon.length - PICK_TILES_VISIBLE;
  const typesText = candidate.types.map((type) => typeNameJa[type]).join(balanceScreenText.listSeparator);
  return (
    <div className="balance-pick" role="group" aria-label={balanceScreenText.pickGroupLabel(typesText)}>
      <div className="balance-pick__tiles">
        {shown.map((pokemon) => {
          const species = speciesFor(master.species, pokemon.pokemonId);
          return (
            <Tile
              key={pokemon.pokemonId}
              className="balance-pick__tile"
              disabled={!canPick}
              onClick={() => {
                onPick(pokemon.pokemonId);
              }}
            >
              <PokemonIcon speciesKey={pokemon.pokemonId} typeId={species?.types[0] ?? pokemon.types[0]} />
              {pokemon.nameJa ?? pokemon.pokemonId}
            </Tile>
          );
        })}
      </div>
      {hiddenCount > 0 && (
        <button
          type="button"
          className="ui-button ui-button--secondary"
          aria-expanded={expanded}
          onClick={() => {
            setExpanded((current) => !current);
          }}
        >
          {expanded ? balanceScreenText.pickShowLess : balanceScreenText.pickShowMore}
        </button>
      )}
      {!canPick && <p className="balance-pick__reason">{balanceScreenText.pickNoEmptySlot}</p>}
    </div>
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
