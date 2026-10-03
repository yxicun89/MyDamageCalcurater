// ポケモン画像(ADR-0325)。manifest にあれば <img>(装飾。alt 空)、無い・未取得・読み込み失敗なら fallback(タイプ色エンブレム)。

import { useState, type ReactNode } from "react";
import { usePokemonImageManifest } from "./PokemonImagesContext";
import { pokemonImageUrl, type PokemonImageSize } from "./pokemonImages";

// CLS 対策の既定寸法(実際の寸法は className の CSS が決める)。
const DEFAULT_PIXELS: Record<PokemonImageSize, number> = { thumb: 32, detail: 128 };

interface PokemonImageProps {
  readonly speciesKey: string;
  readonly size: PokemonImageSize;
  /** 画像が無いときに出す既存のエンブレム。 */
  readonly fallback: ReactNode;
  readonly className?: string;
}

export function PokemonImage({ speciesKey, size, fallback, className }: PokemonImageProps) {
  const manifest = usePokemonImageManifest();
  const url = pokemonImageUrl(manifest, speciesKey, size);
  // 読み込みに失敗した URL を覚える。URL が変われば(別のキー)失敗を引きずらない。
  const [failedUrl, setFailedUrl] = useState<string | null>(null);

  if (url === null || failedUrl === url) {
    return <>{fallback}</>;
  }
  return (
    <img
      src={url}
      alt=""
      loading="lazy"
      width={DEFAULT_PIXELS[size]}
      height={DEFAULT_PIXELS[size]}
      className={className}
      onError={() => {
        setFailedUrl(url);
      }}
    />
  );
}
