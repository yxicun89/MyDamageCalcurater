// P4-16: pokedex-svc の公開 API からマスタを読む MasterSource(ADR-0304)。
// 公開 API(api/openapi.yaml)の制約:
//   - searchSpecies / searchMoves / searchItems は limit の上限が 200 でページングが無い。
//     実データは種族349件・技515件なので一括取得できず、持ち物166件・性格25件だけが1回で全件取れる(ADR-0304 §1)。
//   - getSpecies.learnset は技の ID 配列だけで、ID から技の実体を引く公開 API が無い(ADR-0304 §3。API レーン待ち)。
// なので load() は「持ち物・性格・相性表」だけを返し、種族は MasterSpeciesSearch で都度引き、技は使えないことを
// capabilities で伝える(ADR-0304 §4 の段階的導入)。
// 応答の型は openapi-typescript の生成型(api/openapi.gen.ts)を正とし、手で複製しない(ADR-0301 §6)。

import type { ClientIds } from "../api/clientIds";
import type { MasterCapabilities, SearchableMasterSource } from "./types";

/**
 * 持ち物を1回で全件取るときの limit。公開 API の maximum(200)そのもので、実データ166件を上回る
 * (ADR-0304 §1 の実測)。これを超えて持ち物が増えたら1回では取り切れないので、load() は黙って
 * 打ち切らずに失敗する。
 */
export const ITEMS_FETCH_LIMIT = 200;

/**
 * 種族の検索で1回に受け取る件数の上限。349件を一覧にはできないので、入力補完の候補として出せる数に絞る
 * (ADR-0304 §1 の「既定の limit(50)」)。
 */
export const SPECIES_SEARCH_LIMIT = 50;

/** 種族の検索を始める最小の文字数。1文字目から候補を出す(日本語名の前方一致なので1文字で十分絞れる)。 */
export const SPECIES_SEARCH_MIN_LENGTH = 1;

/**
 * 種族の検索入力のデバウンス(ミリ秒)。入力1文字ごとに検索を投げない(ADR-0304 §1)。
 * 検索 UI(P4-16b)がこの値を使う。
 */
export const SPECIES_SEARCH_DEBOUNCE_MS = 250;

/** オンラインのマスタが使える機能(ADR-0304 §4)。種族は検索、技は未対応、効果データは公開 API に無い。 */
export const ONLINE_MASTER_CAPABILITIES: MasterCapabilities = {
  speciesList: false,
  moves: false,
  effects: false,
};

/** createOnlineMasterSource の引数(createApiEngine と同じ形。ADR-0301 §3・§4)。 */
export interface CreateOnlineMasterSourceInput {
  /** API の基点 URL(末尾はスラッシュ1つ。api/config.ts の apiBaseUrl() で作る)。 */
  readonly baseUrl: string;
  /** 注入する fetch(テストは fake、実行時は globalThis.fetch)。 */
  readonly fetch: typeof fetch;
  /** 全リクエストに付ける端末 ID・セッション ID(CLAUDE.md 技術規約)。 */
  readonly ids: ClientIds;
}

/** pokedex-svc の公開 API から読むマスタの取得口(P4-16、ADR-0304)。 */
// eslint-disable-next-line @typescript-eslint/no-unused-vars -- P4-16 spec-writer のスタブ(引数は実装で使う)
export function createOnlineMasterSource(input: CreateOnlineMasterSourceInput): SearchableMasterSource {
  // P4-16 spec-writer のスタブ。implementer が実装する(master/onlineSource.test.ts が仕様)。
  throw new Error("createOnlineMasterSource は未実装(P4-16)");
}
