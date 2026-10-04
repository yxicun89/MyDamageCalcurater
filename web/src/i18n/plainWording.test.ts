// F-13(I-web-11、ADR-0337): 文言のやさしい言い換えの代表の文言。正は docs/glossary.md と ADR-0337 の対応表(旧 → 新)。
// キー名は変えず値(文言)だけを変える(参照するコードを壊さない)。ここは各画面の代表の文言が新しい語になったことと、
// 対戦の標準の用語(確定・乱数・特化・無振り など)を変えていないことを確かめる。
// 禁止語が残っていないことの網羅は glossary.test.ts が受け持つ。

import { describe, expect, test } from "vitest";
import { deviceDataText } from "./about";
import { adjustErrorText, adjustScreenText } from "./adjust";
import { balanceErrorText } from "./balance";
import { favoritesScreenText } from "./favorites";
import { itemRoleText } from "./items";
import {
  apiEngineText,
  appText,
  attackerPresetText,
  calcScreenText,
  defenderPresetText,
  megaItemText,
  requestLimitText,
  resultText,
  reverseResultText,
  reverseScreenText,
} from "./ja";
import { speedPresetText, speedScreenText } from "./speed";
import { teamClientText } from "./team";

describe("逆算: 「観測」をやめて「ダメージ」と言う", () => {
  test("ダメージの行・追加・削除・単位", () => {
    expect(reverseScreenText.observationLabel(1)).toBe("ダメージ1");
    expect(reverseScreenText.observationUnitGroupLabel(2)).toBe("ダメージ2の単位");
    expect(reverseScreenText.removeObservationLabel(3)).toBe("ダメージ3を削除");
    expect(reverseScreenText.addObservationLabel).toBe("ダメージを追加");
    expect(reverseScreenText.sideGroupLabel).toBe("どちらのダメージ");
  });

  test("与えた・受けたの選択肢は変えない", () => {
    expect(reverseScreenText.sideDefenderLabel).toBe("与えたダメージ");
    expect(reverseScreenText.sideAttackerLabel).toBe("受けたダメージ");
  });

  test("結果の一覧・ほぼ合う候補・見つからないときの案内", () => {
    expect(reverseScreenText.resultsListLabel).toBe("考えられる振り方");
    expect(reverseResultText.closeCandidateLabel).toBe("ほぼ合う候補");
    expect(reverseResultText.noExactCandidateNotice).toBe(
      "入力したダメージにぴったり合う振り方が見つかりません(技・持ち物・入力した値を確かめてください)",
    );
  });

  test("件数の上限の案内", () => {
    expect(requestLimitText.observationLimitReached(8)).toBe(
      "ダメージは8件まで入力できます。追加するには、どれかの行を削除してください",
    );
  });
});

describe("計算・ヘッダー: 内部の仕組みの語を出さない", () => {
  test("計算する場所の切り替え", () => {
    expect(appText.calcModeGroupLabel).toBe("計算する場所");
    expect(appText.calcModeOfflineLabel).toBe("この端末(オフライン)");
    expect(appText.calcModeOnlineLabel).toBe("サーバー(オンライン)");
  });

  test("データを読み込めないときの案内", () => {
    expect(appText.masterLoadError).toBe("ポケモンのデータを読み込めませんでした");
    expect(appText.onlineMasterLoadError).toBe(
      "サーバーからポケモンのデータを読み込めませんでした。接続を確かめて、もう一度お試しください",
    );
    expect(appText.masterCacheEmptyError).toBe(
      "オフラインで使うには、一度オンラインで開いてポケモンのデータを取り込んでください",
    );
  });

  test("特性のおまかせ", () => {
    expect(calcScreenText.anyAbilityOption).toBe("おまかせ(すべての特性で計算)");
  });

  test("サーバーに接続できないとき", () => {
    expect(apiEngineText.unavailable).toBe("サーバーに接続できません");
    expect(apiEngineText.unknownNature).toBe("この性格がデータに見つかりません");
  });
});

describe("メガシンカ: 見出し+コロンの書き方をやめ、ふつうの文にする", () => {
  test("持ち物欄が固定される理由", () => {
    expect(megaItemText.lockedReason).toBe("メガシンカするので、持ち物はメガストーンに決まっています");
    expect(megaItemText.missingReason).toBe("メガシンカに使うメガストーンが、データに見つかりません");
    expect(megaItemText.compareDisabledReason).toBe(
      "防御側はメガシンカするので持ち物がメガストーンに決まっています。持ち物の候補は比べません",
    );
    expect(megaItemText.clearedNotice).toBe(
      "メガシンカに使うメガストーンがデータに無いため、持ち物を空にしました。保存すると反映されます",
    );
  });

  test("持ち物欄の固定のメガストーンの表示(「〇〇のメガストーン」をやめる)", () => {
    expect(itemRoleText.megaStoneOf("テストドラゴン")).toBe("テストドラゴン専用のメガストーン");
    // 基本種名が分からないときの表示は変えない(名前は推測しない。ADR-0326)。
    expect(itemRoleText.megaStoneUnnamed).toBe("メガストーン");
  });
});

describe("調整: 「指数」「16n」「最小の振り方」を言い換える(ec レーンの依頼)", () => {
  test("モードの名前", () => {
    expect(adjustScreenText.modeLabel.indices).toBe("今の耐久・火力と HP を見る");
    expect(adjustScreenText.modeLabel.minKo).toBe("倒せるいちばん少ない振り方");
    expect(adjustScreenText.modeLabel.minSurvive).toBe("耐えられるいちばん少ない振り方");
    // 言い換えの対象でないモードは変えない。
    expect(adjustScreenText.modeLabel.goals).toBe("目標から振り方を決める");
  });

  test("指数 → 目安", () => {
    expect(adjustScreenText.indicesHeading).toBe("今の振り方の強さ(目安)");
    expect(adjustScreenText.firepowerIndexLabel).toBe("火力の目安");
    expect(adjustScreenText.physicalBulkLabel).toBe("物理耐久の目安");
    expect(adjustScreenText.specialBulkLabel).toBe("特殊耐久の目安");
    expect(adjustScreenText.indexNote).toBe(
      "火力の目安には、タイプ一致だけを含めます(持ち物・特性・テラスタルは含めません)",
    );
  });

  test("16n → 16の倍数", () => {
    expect(adjustScreenText.hpLineHeading).toBe("HP と 16 の倍数");
    expect(adjustScreenText.hpLineKindLabel).toEqual({
      none: "16の倍数でも、16の倍数-1でもない",
      "16n": "16の倍数",
      "16n-1": "16の倍数-1",
    });
    expect(adjustScreenText.next16nLabel).toBe("次の16の倍数");
    expect(adjustScreenText.prev16nLabel).toBe("前の16の倍数");
    expect(adjustScreenText.next16nMinus1Label).toBe("次の16の倍数-1");
    expect(adjustScreenText.prev16nMinus1Label).toBe("前の16の倍数-1");
  });

  test("振り方の見出し", () => {
    expect(adjustScreenText.maxIndexHeading).toBe("いちばん強くなる振り方");
    expect(adjustScreenText.minSpHeading).toBe("目標に届くいちばん少ない振り方");
    expect(adjustScreenText.minSpNotRequested).toBe(
      "目標を指定すると、目標に届くいちばん少ない振り方も出します",
    );
  });

  test("エラーの言い方", () => {
    expect(adjustErrorText.unknown_species).toBe("このポケモンはデータにありません");
    expect(adjustErrorText.missing_header).toBe(
      "端末の情報を送れませんでした。ページを読み込み直してください",
    );
    expect(adjustErrorText.adjust_unavailable).toBe("調整のサーバーに接続できません");
    expect(adjustScreenText.natureNotFoundMessage).toBe("相手の調整に合う性格がデータにありません");
  });
});

describe("素早さ・タイプバランス・構築・お気に入り・このアプリについて", () => {
  test("素早さの入力の方法(プリセット → 定番の振り方)", () => {
    expect(speedScreenText.modeLabel.preset).toBe("定番の振り方");
    expect(speedScreenText.errorByCode.speed_unavailable).toBe("素早さのサーバーに接続できません");
  });

  test("タイプバランスのエラー", () => {
    expect(balanceErrorText.invalid_request).toBe("入力の内容が正しくありません。見直してください");
    expect(balanceErrorText.unknown_pokemon).toBe(
      "選んだポケモンがサーバーのデータにありません。選び直してください",
    );
  });

  test("構築のエラー", () => {
    expect(teamClientText.unavailable).toBe("構築のサーバーに接続できません");
  });

  test("お気に入りの案内は、ヘッダーの見出しと同じ語(計算する場所)を使う", () => {
    expect(favoritesScreenText.offlineNotice).toContain(appText.calcModeGroupLabel);
    expect(favoritesScreenText.offlineNotice).toContain(appText.calcModeOnlineLabel);
  });

  test("端末のデータの説明(ID → 番号)", () => {
    expect(deviceDataText.explanation[0]).toBe(
      "アカウントはありません。履歴・お気に入り・構築は、この端末に割り当てた番号でサーバーに保存しています。",
    );
    expect(deviceDataText.explanation[1]).toBe(
      "番号が変わると(ブラウザのサイトデータを消したとき)、前のデータは開けなくなります。元に戻す方法はありません。",
    );
  });
});

describe("対戦の標準の用語は変えない(言い換えすぎない)", () => {
  test("確定数・乱数・相性", () => {
    expect(resultText.determinedPrefix).toBe("確定");
    expect(resultText.randomPrefix).toBe("乱数");
    expect(resultText.hitsSuffix).toBe("発");
    expect(resultText.effectivenessSuper(2)).toBe("ばつぐん(×2)");
  });

  test("振り方の名前", () => {
    expect(attackerPresetText.none).toBe("無振り");
    expect(attackerPresetText.fullSuffix).toBe("特化");
    expect(defenderPresetText.hb_full).toBe("HB特化");
    expect(speedPresetText["neutral-max"]).toBe("準速");
    expect(speedPresetText.max).toBe("最速");
  });

  test("画面の名前(逆算・調整)", () => {
    expect(appText.reverseTabLabel).toBe("逆算");
    expect(appText.adjustTabLabel).toBe("調整");
  });
});
