// 構築名の扱い(ADR-0332 §1)。構築名の入力は廃止したので、画面は name を送らず、サーバーが既定名を入れる(ADR-0229)。
// ここは応答の Team.name から画面に出す表示名を導く。

import type { components } from "../api/openapi.gen";
import { teamScreenText } from "../i18n/team";

type Schemas = components["schemas"];

/** TeamInput.name を省略したときにサーバーが入れる既定名(api/openapi.yaml・ADR-0229 の契約の値)。画面には出さない。 */
export const SERVER_DEFAULT_TEAM_NAME = "名称未設定";

function createdTime(team: Schemas["Team"]): number {
  const time = Date.parse(team.createdAt);
  return Number.isNaN(time) ? 0 : time;
}

/**
 * 構築 id → 画面に出す名前。既定名(完全一致)の構築は「構築 N」(N は既定名の構築だけを作成の古い順に数えた 1 始まり。
 * 同じ作成日時は id の昇順)。それ以外の名前(古い構築に付けた名前)はそのまま。
 * 一覧の位置で数えると、新しい構築を足すたびに番号がずれるので作成順にしている。
 */
export function teamDisplayNames(teams: readonly Schemas["Team"][]): ReadonlyMap<string, string> {
  const names = new Map<string, string>();
  const untitled = teams
    .filter((team) => team.name === SERVER_DEFAULT_TEAM_NAME)
    .sort((a, b) => createdTime(a) - createdTime(b) || (a.id < b.id ? -1 : a.id > b.id ? 1 : 0));
  untitled.forEach((team, index) => {
    names.set(team.id, teamScreenText.untitledTeamName(index + 1));
  });
  for (const team of teams) {
    if (!names.has(team.id)) {
      names.set(team.id, team.name);
    }
  }
  return names;
}
