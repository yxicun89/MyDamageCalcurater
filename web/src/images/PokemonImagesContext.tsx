// ポケモン画像の manifest を配る Provider(ADR-0325)。取得は起動時に1回だけ。失敗は静かに「画像なし」にする。

import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { fetchPokemonImageManifest, type PokemonImageManifest } from "./pokemonImages";

const PokemonImagesContext = createContext<PokemonImageManifest | null>(null);

/** 現在の manifest(Provider が無い・未取得・失敗なら null = 画像なし)。 */
export function usePokemonImageManifest(): PokemonImageManifest | null {
  return useContext(PokemonImagesContext);
}

interface PokemonImagesProviderProps {
  /** 取得済みの manifest を直接渡す口(テスト用)。渡すと取得しない。 */
  readonly manifest?: PokemonImageManifest | null;
  /** manifest を1回だけ取得する fetch。 */
  readonly fetch?: typeof fetch;
  readonly children: ReactNode;
}

// StrictMode の二重 effect でも取得を1回にするため、fetch 実装ごとの取得 Promise を覚える。
const inflight = new WeakMap<typeof fetch, Promise<PokemonImageManifest | null>>();

function fetchOnce(fetchImpl: typeof fetch): Promise<PokemonImageManifest | null> {
  let promise = inflight.get(fetchImpl);
  if (promise === undefined) {
    promise = fetchPokemonImageManifest(fetchImpl);
    inflight.set(fetchImpl, promise);
  }
  return promise;
}

export function PokemonImagesProvider({
  manifest: given,
  fetch: fetchImpl,
  children,
}: PokemonImagesProviderProps) {
  const [fetched, setFetched] = useState<PokemonImageManifest | null>(null);

  useEffect(() => {
    if (given !== undefined || fetchImpl === undefined) {
      return;
    }
    let active = true;
    void fetchOnce(fetchImpl).then((result) => {
      if (active) {
        setFetched(result);
      }
    });
    return () => {
      active = false;
    };
  }, [given, fetchImpl]);

  return (
    <PokemonImagesContext.Provider value={given !== undefined ? given : fetched}>
      {children}
    </PokemonImagesContext.Provider>
  );
}
