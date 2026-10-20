// タイプバランスの節(team/TeamBalanceSection)を、メンバーの入力欄つきでテストするための共通の足場(テスト専用)。
// 実アプリではメンバーは構築編集(TeamMemberEditor)が下書きとして持つ。ここでは MemberFields でその下書きを作り、
// TeamBalanceSection には本物と同じ props で渡す(仮想敵・API 呼び出し・結果表示は本物をそのまま描画する)。
// アプリ本体からは import しない。

import { act, fireEvent, render, screen, type RenderResult } from "@testing-library/react";
import type { ReactNode } from "react";
import { useState } from "react";
import type { BalanceClient } from "../api/balanceClient";
import {
  emptyMember,
  MAX_MEMBERS,
  MemberFields,
  MOVE_SLOTS,
  toBalanceInput,
  useMemberListActions,
  type MemberState,
} from "../balance/memberInputs";
import { BalancePanel } from "../balance/BalancePanel";
import { isBalanceAvailable, useTeamBalance } from "../balance/useTeamBalance";
import { balanceScreenText } from "../i18n/ja";
import { masterCapabilities } from "../master/capabilities";
import type { MasterData, MasterSpeciesSearch } from "../master/types";
import { useSpeciesResolutions, type SpeciesResolutions } from "../screens/speciesResolution";
import { blankDraft, type MemberDraft } from "../team/teamMember";
import { TeamBalanceSection } from "../team/TeamBalanceSection";
import { Tile } from "../ui/Tile";

export interface BalanceTestBedProps {
  readonly master: MasterData;
  readonly client: BalanceClient;
  readonly masterSearch?: MasterSpeciesSearch;
  /** おすすめのポケモンを押したときの口(既定は何もしない)。 */
  readonly onPickPokemon?: (speciesKey: string) => void;
  /**
   * true なら節(TeamBalanceSection)を使わず、BalancePanel と useTeamBalance を直接描画する
   * (節が足す設定=おすすめの件数 5 件・ポケモンを選ぶタイルを含まない素の挙動を確かめるテスト用。仮想敵は無い)。
   */
  readonly directPanel?: boolean;
}

/** 入力中のメンバーの枠 → 構築の下書き(空の枠は speciesKey: null。技の空欄は null)。 */
function toDraft(member: MemberState): MemberDraft {
  return {
    ...blankDraft(""),
    speciesKey: member.speciesKey === "" ? null : member.speciesKey,
    abilityId: member.abilityId === "" ? null : member.abilityId,
    moves: Array.from({ length: MOVE_SLOTS }, (_, slot) => {
      const moveId = member.moveIds[slot] ?? "";
      return moveId === "" ? null : moveId;
    }),
  };
}

export function BalanceTestBed({
  master,
  client,
  masterSearch,
  onPickPokemon,
  directPanel = false,
}: BalanceTestBedProps): ReactNode {
  const capabilities = masterCapabilities(master);
  const available = isBalanceAvailable(master, masterSearch);
  const resolutions = useSpeciesResolutions();
  const [members, setMembers] = useState<MemberState[]>(() => [emptyMember(0)]);
  const memberActions = useMemberListActions(setMembers);

  return (
    <div>
      <div>
        {members.map((member, index) => (
          <MemberFields
            key={member.id}
            groupLabel={balanceScreenText.memberGroupLabel(index + 1)}
            removeLabel={balanceScreenText.removeMemberLabel(index + 1)}
            member={member}
            master={master}
            removable={members.length > 1}
            disabled={!available}
            movesListAvailable={capabilities.moves}
            speciesListAvailable={capabilities.speciesList}
            masterSearch={masterSearch}
            speciesFor={resolutions.speciesFor}
            abilitiesFor={resolutions.abilitiesFor}
            movesFor={resolutions.movesFor}
            onSelectSpecies={(speciesKey) => {
              memberActions.selectSpecies(master, index, speciesKey);
            }}
            onSpeciesResolved={(resolution) => {
              resolutions.register(resolution);
              memberActions.resolveSpecies(member.id, resolution);
            }}
            onSelectAbility={(abilityId) => {
              memberActions.selectAbility(index, abilityId);
            }}
            onSelectMove={(slot, moveId) => {
              memberActions.selectMove(index, slot, moveId);
            }}
            onRemove={() => {
              memberActions.remove(index);
            }}
          />
        ))}
      </div>
      <Tile
        empty
        label={balanceScreenText.addMemberLabel}
        onClick={memberActions.add}
        disabled={!available || members.length >= MAX_MEMBERS}
      />
      {directPanel ? (
        <DirectPanel
          members={members}
          client={client}
          master={master}
          masterSearch={masterSearch}
          resolutions={resolutions}
        />
      ) : (
        <TeamBalanceSection
          drafts={members.map(toDraft)}
          client={client}
          master={master}
          masterSearch={masterSearch}
          resolutions={resolutions}
          onPickPokemon={onPickPokemon ?? ((): void => undefined)}
          canPick
          debounceMs={0}
        />
      )}
    </div>
  );
}

function DirectPanel({
  members,
  client,
  master,
  masterSearch,
  resolutions,
}: {
  readonly members: readonly MemberState[];
  readonly client: BalanceClient;
  readonly master: MasterData;
  readonly masterSearch: MasterSpeciesSearch | undefined;
  readonly resolutions: SpeciesResolutions;
}): ReactNode {
  const state = useTeamBalance({
    members: members.map(toBalanceInput),
    threats: [],
    client,
    master,
    masterSearch,
    movesFor: resolutions.movesFor,
  });
  return (
    <BalancePanel
      state={state}
      master={master}
      speciesFor={resolutions.speciesFor}
      abilitiesFor={resolutions.abilitiesFor}
    />
  );
}

/**
 * 描画し、仮想敵の最初の 1 枠を開く(本物の節は仮想敵が 0 体から始まるため。「仮想敵1」の入力欄がすぐ使える)。
 * 使えないマスタ(仮想敵の入力が出ない)では何もしない。
 */
export function renderBalanceTestBed(props: BalanceTestBedProps): RenderResult {
  const rendered = render(<BalanceTestBed {...props} />);
  openFirstThreat();
  return rendered;
}

export function openFirstThreat(): void {
  const add = screen.queryByRole("button", { name: balanceScreenText.addThreatLabel });
  if (add !== null) {
    act(() => {
      fireEvent.click(add);
    });
  }
}
