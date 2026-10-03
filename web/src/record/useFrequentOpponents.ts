// P5-5c: 「よく計算する相手」の取得と名前解決(ADR-0317 §2〜§4)。
// 取得は recordClient が渡されたときのマウント時に1回。失敗・0件・引けない key は黙って落とす
// (計算には影響させない。絶対ルール5)。

import { useEffect, useMemo, useState } from "react";
import { masterCapabilities } from "../master/capabilities";
import type {
  MasterData,
  MasterSpecies,
  MasterSpeciesResolution,
  MasterSpeciesSearch,
} from "../master/types";
import type { RecordClient } from "./recordClient";

export interface FrequentOpponentChip {
  readonly key: string;
  readonly nameJa: string;
  /** 検索マスタで解決した結果(一覧マスタでは undefined)。押したときに覚え書きへ登録するのに使う。 */
  readonly resolution: MasterSpeciesResolution | undefined;
}

export function useFrequentOpponents(
  recordClient: RecordClient | undefined,
  master: MasterData,
  masterSearch: MasterSpeciesSearch | undefined,
): readonly FrequentOpponentChip[] {
  const [keys, setKeys] = useState<readonly string[]>([]);
  const [resolutions, setResolutions] = useState<ReadonlyMap<string, MasterSpeciesResolution>>(new Map());
  const speciesListAvailable = masterCapabilities(master).speciesList;

  useEffect(() => {
    if (recordClient === undefined) {
      return;
    }
    const controller = new AbortController();
    recordClient.listFrequentOpponents(controller.signal).then(
      (result) => {
        if (!controller.signal.aborted && result.ok) {
          setKeys(result.value.map((opponent) => opponent.speciesKey));
        }
      },
      () => undefined,
    );
    return () => {
      controller.abort();
    };
  }, [recordClient]);

  useEffect(() => {
    if (speciesListAvailable || masterSearch === undefined || keys.length === 0) {
      return;
    }
    const controller = new AbortController();
    void Promise.allSettled(keys.map((key) => masterSearch.resolveSpecies(key, controller.signal))).then(
      (settled) => {
        if (controller.signal.aborted) {
          return;
        }
        const next = new Map<string, MasterSpeciesResolution>();
        for (const item of settled) {
          if (item.status === "fulfilled") {
            next.set(item.value.species.key, item.value);
          }
        }
        setResolutions(next);
      },
    );
    return () => {
      controller.abort();
    };
  }, [keys, masterSearch, speciesListAvailable]);

  return useMemo(() => {
    const chips: FrequentOpponentChip[] = [];
    for (const key of keys) {
      const resolution = resolutions.get(key);
      const species: MasterSpecies | undefined =
        master.species.find((candidate) => candidate.key === key) ?? resolution?.species;
      if (species !== undefined) {
        chips.push({ key, nameJa: species.nameJa, resolution });
      }
    }
    return chips;
  }, [keys, resolutions, master.species]);
}
