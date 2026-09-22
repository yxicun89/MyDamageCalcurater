// P4-6: Playwright の設定(playwright.config.ts・playwright.online.config.ts)が共有する値と前提の検査。
// E2E は本番ビルド(vite build → vite preview)を相手にする。engine.wasm・wasm_exec.js は vite build が
// web/public/ から dist/ へ写すので、`make wasm` で先に作っておく必要がある。

import { existsSync } from "node:fs";
import { fileURLToPath } from "node:url";

/** web/ の絶対パス。 */
export const webRoot = fileURLToPath(new URL("../..", import.meta.url));

/** リポジトリルートの絶対パス。 */
export const repoRoot = fileURLToPath(new URL("../../..", import.meta.url));

/** オフライン(WASM)用の preview サーバーのポート(他の開発サーバーと衝突しにくい値)。 */
export const OFFLINE_PORT = 4317;

/** オンライン(API)用の preview サーバーのポート。 */
export const ONLINE_PORT = 4318;

/** オンライン用に起動する calc-svc のポート。 */
export const CALC_SVC_PORT = 18317;

/** P4-12a: タイプバランス(balance API)用の preview サーバーのポート。 */
export const BALANCE_PORT = 4320;

/** P4-12a: タイプバランスの E2E で起動する balance-svc のポート。 */
export const BALANCE_SVC_PORT = 18318;

/** 本番ビルドを作って preview で配るコマンド(ポートは呼び出し側が決める)。 */
export function previewCommand(port: number): string {
  return `npx vite build && npx vite preview --host 127.0.0.1 --port ${port} --strictPort`;
}

/** make wasm が web/public/ に出す成果物。無ければ E2E は意味をなさない。 */
const REQUIRED_PUBLIC_FILES = ["public/engine.wasm", "public/wasm_exec.js"] as const;

/**
 * engine.wasm・wasm_exec.js が無ければ例外を投げ、設定の読み込みの時点で E2E を失敗させる
 * (スキップして成功扱いにしない。docs/development-workflow.md「未実装ターゲットの正常終了を成功と数えない」)。
 */
export function assertWasmArtifacts(): void {
  const missing = REQUIRED_PUBLIC_FILES.filter((path) => !existsSync(`${webRoot}${path}`));
  if (missing.length > 0) {
    throw new Error(
      `E2E の前提が足りない: ${missing.join(", ")} が無い。リポジトリルートで make wasm を実行してから再実行する`,
    );
  }
}

/** P4-11: コンテナ(pokecalc/web:local)の E2E で、ホスト側に出すポート。 */
export const CONTAINER_PORT = 4319;

/** P4-11: コンテナの E2E が起動するイメージ(`make web-docker-build` が作る)。 */
export const CONTAINER_IMAGE = "pokecalc/web:local";

/** P4-11: コンテナの E2E が使うコンテナ名(前回の残りを消すのに使う)。 */
export const CONTAINER_NAME = "pokecalc-web-e2e";

/**
 * 作り済みのイメージを起動するコマンド。イメージが無ければ Docker Hub から取りに行かず失敗させる(--pull never)。
 * 前回の実行で残ったコンテナがあれば先に消す。コンテナ内の nginx は 8080 で待ち受ける(web/Dockerfile)。
 * k8s の Deployment(deploy/k8s/base/web/deployment.yaml)と同じ制約(読み取り専用のルート・/tmp だけ書ける・
 * 非 root の UID 101・capability なし・権限昇格なし)で起動し、k3d に載せる前に同じ条件で壊れないことを確かめる。
 */
export function containerCommand(port: number): string {
  return (
    `docker rm -f ${CONTAINER_NAME} >/dev/null 2>&1; ` +
    `docker run --rm --pull never --read-only --tmpfs /tmp --user 101:101 --cap-drop ALL ` +
    `--security-opt no-new-privileges -p 127.0.0.1:${port}:8080 --name ${CONTAINER_NAME} ${CONTAINER_IMAGE}`
  );
}
