// P4-16b(ADR-0304 A-10): 検索で解決した種族・特性を覚える(取得結果を覚えるのは呼び出し側の責務。
// MasterSpeciesSearch の実装自体はキャッシュを持たない)。CalcScreen・ReverseScreen で共通に使う。
//
// capabilities.speciesList が true のマスタ(ドロップダウン、既存の動き)では、この覚え書きは常に空のままで、
// speciesFor・abilitiesFor は master.species・master.abilities をそのまま返す(今までどおり)。
// capabilities.speciesList が false のマスタ(検索)では、master.species・master.abilities が空なので、
// resolveSpecies で返った実体をここに足していく。

import { useCallback, useState } from "react";
import type { Ability } from "../engine/types";
import type { MasterSpecies, MasterSpeciesResolution } from "../master/types";

/** 検索で解決した種族・特性の覚え書き(CalcScreen.tsx・ReverseScreen.tsx で共有する形)。 */
export interface SpeciesResolutions {
  /** 種族を key で引く。master.species(全件)に無ければ、検索で解決した種族から探す。 */
  readonly speciesFor: (masterSpeciesList: readonly MasterSpecies[], key: string) => MasterSpecies | null;
  /** 特性の一覧(master.abilities に、検索で解決した特性を足したもの)。defaultAbility に渡す。 */
  readonly abilitiesFor: (masterAbilities: readonly Ability[], key: string) => readonly Ability[];
  /** resolveSpecies の結果を覚える。 */
  readonly register: (resolution: MasterSpeciesResolution) => void;
}

/** speciesResolution.ts の唯一のフック(コーディング規約 §2: 同じ定義を複数箇所に書かない)。 */
export function useSpeciesResolutions(): SpeciesResolutions {
  const [resolved, setResolved] = useState<ReadonlyMap<string, MasterSpeciesResolution>>(new Map());

  const register = useCallback((resolution: MasterSpeciesResolution): void => {
    setResolved((prev) => {
      const next = new Map(prev);
      next.set(resolution.species.key, resolution);
      return next;
    });
  }, []);

  const speciesFor = useCallback(
    (masterSpeciesList: readonly MasterSpecies[], key: string): MasterSpecies | null =>
      masterSpeciesList.find((species) => species.key === key) ?? resolved.get(key)?.species ?? null,
    [resolved],
  );

  const abilitiesFor = useCallback(
    (masterAbilities: readonly Ability[], key: string): readonly Ability[] => {
      const extra = resolved.get(key)?.abilities;
      return extra === undefined ? masterAbilities : [...masterAbilities, ...extra];
    },
    [resolved],
  );

  return { speciesFor, abilitiesFor, register };
}
