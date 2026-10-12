// ADR-0358(案 A)段 1: オフラインのとき、画面の上に「保存済み: ○匹。使える範囲」の 1 行を出す。
// 保存済みの数は App から渡された口(MasterCacheStore の species のキー数)で、表示するたびに(mount ごとに)読む。
// 読めない・0 は「まだありません」の案内にする(失敗させない)。

import { useEffect, useState } from "react";
import { offlineScopeText } from "../i18n/offlineScope";

interface OfflineScopeNoticeProps {
  /** 保存済みのポケモンの数を読む口。省くと 0 匹。 */
  readonly loadCount?: () => Promise<number>;
}

export function OfflineScopeNotice({ loadCount }: OfflineScopeNoticeProps) {
  const [count, setCount] = useState(0);

  useEffect(() => {
    let cancelled = false;
    if (loadCount === undefined) {
      return undefined;
    }
    loadCount().then(
      (value) => {
        if (!cancelled) {
          setCount(value);
        }
      },
      () => {
        if (!cancelled) {
          setCount(0);
        }
      },
    );
    return () => {
      cancelled = true;
    };
  }, [loadCount]);

  return (
    <p
      role="note"
      aria-label={offlineScopeText.regionLabel}
      className="ui-notice ui-notice--info app-offline-scope"
    >
      {count > 0 ? offlineScopeText.summary(count) : offlineScopeText.empty}
    </p>
  );
}
