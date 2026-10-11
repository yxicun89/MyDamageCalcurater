// タイプバランスの結果の表示(ADR-0421 §3)。useTeamBalance の状態を、防御相性・集計・攻撃範囲・
// 仮想敵・おすすめタイプの順に出す。倍率・集計・穴の判定は応答をそのまま出し、Web で計算し直さない(ADR-0303 §1・§7)。
// 記号+文字は API 値だけから作る(balanceCells・domain/balanceLabels。ADR-0303 §7)。

import { balanceScreenText } from "../i18n/ja";
import type { MasterData } from "../master/types";
import type { SpeciesResolutions } from "../screens/speciesResolution";
import { balanceErrorMessage } from "./balanceParts";
import { RecommendationsSection, ThreatSection } from "./balanceSections";
import { CoverageTable, DefenseTable, TeamSummaryTable } from "./balanceTables";
import type { TeamBalanceState } from "./types";
import "../screens/BalanceScreen.css";

export interface BalancePanelProps {
  readonly state: TeamBalanceState;
  readonly master: MasterData;
  readonly speciesFor: SpeciesResolutions["speciesFor"];
  readonly abilitiesFor: SpeciesResolutions["abilitiesFor"];
}

/** 結果の節(Fragment。置かれた側のコンテナの子としてそのまま並ぶ)。 */
export function BalancePanel({ state, master, speciesFor, abilitiesFor }: BalancePanelProps) {
  const { analyze, coverage, threats, recommendations, threatSlotNumbers } = state;
  const loading = analyze.status === "loading" || coverage.status === "loading";

  return (
    <>
      {loading && (
        <p className="ui-notice ui-notice--loading balance-screen__notice">
          {balanceScreenText.loadingNotice}
        </p>
      )}
      {analyze.status === "error" && (
        <p role="alert" className="ui-notice ui-notice--error balance-screen__error">
          {balanceErrorMessage(analyze.error.code)}
        </p>
      )}
      {coverage.status === "error" && (
        <p role="alert" className="ui-notice ui-notice--error balance-screen__error">
          {balanceErrorMessage(coverage.error.code)}
        </p>
      )}

      {analyze.status === "success" && (
        <>
          <DefenseTable master={master} speciesFor={speciesFor} response={analyze.value} />
          <TeamSummaryTable response={analyze.value} />
        </>
      )}
      {coverage.status === "success" && <CoverageTable response={coverage.value} />}

      {threats.status === "loading" && (
        <p className="ui-notice ui-notice--loading balance-screen__notice">
          {balanceScreenText.threatsLoadingNotice}
        </p>
      )}
      {threats.status === "error" && (
        <p role="alert" className="ui-notice ui-notice--error balance-screen__error">
          {balanceErrorMessage(threats.error.code)}
        </p>
      )}
      {threats.status === "success" &&
        threats.value.threats.map((threat, index) => (
          <ThreatSection
            key={`${threat.pokemonId}-${String(index)}`}
            n={threatSlotNumbers[index] ?? index + 1}
            master={master}
            speciesFor={speciesFor}
            threat={threat}
          />
        ))}

      {recommendations.status === "loading" && (
        <p className="ui-notice ui-notice--loading balance-screen__notice">
          {balanceScreenText.recommendationsLoadingNotice}
        </p>
      )}
      {recommendations.status === "error" && (
        <p role="alert" className="ui-notice ui-notice--error balance-screen__error">
          {balanceErrorMessage(recommendations.error.code)}
        </p>
      )}
      {recommendations.status === "success" && (
        <RecommendationsSection
          master={master}
          abilitiesFor={abilitiesFor}
          response={recommendations.value}
        />
      )}
    </>
  );
}
