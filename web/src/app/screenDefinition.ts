// 画面の登録ファイル(`*.screen.tsx`)の型と、その定義を作る defineScreen(ADR-0323)。
// 画面を足すレーンは、自分のディレクトリに `<id>.screen.tsx` を1つ置き、`export default defineScreen({...})` を書く。
// App.tsx・app/screens.tsx・app/routes.ts・i18n/ja.ts は触らない(app/screens.tsx が import.meta.glob で集める)。

import type { ReactNode } from "react";
import type { ClientIds } from "../api/clientIds";
import type { CalcEngine } from "../engine/types";
import type { MasterData, MasterSource, MasterSpeciesSearch } from "../master/types";
import type { components } from "../api/openapi.gen";
import type { FavoriteRestoreRequest } from "../favorites/favoriteCalc";
import type { RecordClient } from "../record/recordClient";

type Favorite = components["schemas"]["Favorite"];

/**
 * 画面のクライアントを作る材料(App がマウント時に1回だけ用意する)。基点 URL・fetch・端末 ID/セッション ID は
 * どのクライアントも同じものを使う(ADR-0301 §3)。各レーンの `createXxxClient` の入力にそのまま渡せる形。
 */
export interface ScreenClientDeps {
  readonly baseUrl: string;
  readonly fetch: typeof fetch;
  readonly ids: ClientIds;
}

/**
 * App がどの画面にも渡す、アプリ全体の値(ADR-0323 §2)。画面ごとのクライアントはここに入れない
 * (クライアントは登録ファイルの createClient が作り、render の第2引数で受け取る)。
 */
export interface ScreenEnvironment {
  /** 計算の差し替え口(選択中の計算モードの engine。ADR-0301 §4)。 */
  readonly engine: CalcEngine;
  readonly master: MasterData;
  /**
   * P4-16b(ADR-0304 A-10): 種族を都度引く口。App は今選ばれているマスタの取得口が検索付きのときだけ渡す。
   */
  readonly masterSearch?: MasterSpeciesSearch;
  /** P5-5c(ADR-0317 §2): 記録 API の口。計算モードがオンラインのときだけ渡す。 */
  readonly recordClient?: RecordClient;
  /** P5-5d(ADR-0318 §6): 構築一覧の取り直しの合図(端末データの削除後に App が進める)。 */
  readonly reloadToken?: number;
  /** P5-3c(ADR-0327 §5): お気に入り一覧の取り直しの合図(計算画面での追加・端末データの削除の後に App が進める)。 */
  readonly favoritesReloadToken?: number;
  /** P5-3c(ADR-0327 §5): お気に入りを追加できたことを App に知らせる(お気に入りタブの一覧を古いままにしない)。 */
  readonly onFavoriteAdded?: () => void;
  /** I-web-8(ADR-0333 §3): 計算画面に渡す、お気に入りから入力を戻す要求(token が変わったときだけ適用する)。 */
  readonly favoriteRestore?: FavoriteRestoreRequest;
  /** I-web-8(ADR-0333 §3): お気に入りの「計算に使う」。App が要求の token を進め、計算タブへ移す。 */
  readonly onUseFavorite?: (favorite: Favorite) => void;
  /** issue 276(ADR-0411): オンラインのマスタの取得口(API 専用の画面は計算モードに関係なくこれを使う)。 */
  readonly onlineMasterSource: MasterSource;
}

/**
 * issue 308(ADR-0304 追記6): マスタを使わない画面に渡す値。master と engine を持たない
 * (マスタの読み込みに失敗している間も描画するため。engine を除く理由は ADR-0301 §4 の自動フォールバックの禁止)。
 */
export type MasterlessScreenEnvironment = Omit<ScreenEnvironment, "master" | "engine">;

interface ScreenDefinitionBase<C> {
  /** 画面 ID(重複不可)。 */
  readonly id: string;
  /** base からの相対パスの1セグメント目(先頭・末尾の "/" を含まない。小文字英数字とハイフン。重複不可)。 */
  readonly segment: string;
  /** タブの表示名(文言資源の語をそのまま使う)。 */
  readonly label: string;
  /** タブの並び順(昇順。重複不可)。既存は 100 刻み(ADR-0323 §1)。間に挿入するときは間の値を使う。 */
  readonly order: number;
  /** 画面のクライアントを作る(App のマウント時に1回だけ呼ぶ。ここで通信しない)。 */
  readonly createClient: (deps: ScreenClientDeps) => C;
  /**
   * 真ならタブ・URL・マウントのすべてから外す(登録と検証だけ受ける。ADR-0330)。既定は false。
   * 再表示は、この行を消すだけでよい。
   */
  readonly hidden?: boolean;
}

/** マスタを使う画面の定義。 */
export interface MasterScreenDefinition<C> extends ScreenDefinitionBase<C> {
  readonly usesMaster: true;
  /** 画面を描く(既存の画面の Props への組み立てはここで行う)。 */
  readonly render: (env: ScreenEnvironment, client: C) => ReactNode;
}

/** マスタを使わない画面の定義(マスタの読み込みに失敗していても選んで使える)。 */
export interface MasterlessScreenDefinition<C> extends ScreenDefinitionBase<C> {
  readonly usesMaster: false;
  readonly render: (env: MasterlessScreenEnvironment, client: C) => ReactNode;
}

/** 生成済みのクライアントを閉じ込めた、描画の口(App が画面ごとに1つ持つ)。 */
export type ScreenInstance =
  | {
      readonly id: string;
      readonly usesMaster: true;
      readonly render: (env: ScreenEnvironment) => ReactNode;
    }
  | {
      readonly id: string;
      readonly usesMaster: false;
      readonly render: (env: MasterlessScreenEnvironment) => ReactNode;
    };

/** 登録ファイルの既定エクスポート(クライアントの型を閉じ込めたもの)。 */
export interface RegisteredScreen {
  readonly id: string;
  readonly segment: string;
  readonly label: string;
  readonly order: number;
  readonly usesMaster: boolean;
  /** 非表示の画面か(ADR-0330)。真の画面はタブにも URL にも出ず、クライアントも作らない。 */
  readonly hidden: boolean;
  /** クライアントを作り、描画の口を返す。 */
  readonly instantiate: (deps: ScreenClientDeps) => ScreenInstance;
}

/** 画面の登録ファイルが既定エクスポートにする定義を作る(クライアントの型 C を閉じ込める)。 */
export function defineScreen<C>(
  definition: MasterScreenDefinition<C> | MasterlessScreenDefinition<C>,
): RegisteredScreen {
  const { id, segment, label, order, usesMaster, hidden = false } = definition;
  return {
    id,
    segment,
    label,
    order,
    usesMaster,
    hidden,
    instantiate: (deps) => {
      const client = definition.createClient(deps);
      if (definition.usesMaster) {
        const { render } = definition;
        return { id, usesMaster: true, render: (env) => render(env, client) };
      }
      const { render } = definition;
      return { id, usesMaster: false, render: (env) => render(env, client) };
    },
  };
}

/** クライアントを使わない画面(計算・逆算など)の createClient。 */
export function noClient(): undefined {
  return undefined;
}
