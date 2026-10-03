// 「このアプリについて」情報ページ(issue 328 / P6-18、ADR-0314)。非公式の注記とデータの出典を出す。
// マスタ・engine は使わない(読み込み中・失敗中でも出す)。文言は i18n/ja.ts の aboutText が正。

import { useEffect, useRef } from "react";
import { DeviceDataSection } from "./deviceData/DeviceDataSection";
import type { DeviceDataDeleter } from "./deviceData/deleteDeviceData";
import { aboutText } from "./i18n/ja";

interface AboutScreenProps {
  /** 「計算に戻る」のリンク先(base 付きのパス)。 */
  readonly backHref: string;
  readonly onBack: () => void;
  /** 開いた直後に h2 へフォーカスを移すか(リンクで開いたときだけ。直接開いたときは移さない)。 */
  readonly focusOnMount: boolean;
  /**
   * 「データの扱い」節(P5-5d。ADR-0318 §3)に渡す record / team のクライアントと、team 削除後の合図。
   * 3つとも渡されたときだけ節を出す。
   */
  readonly recordClient?: DeviceDataDeleter;
  readonly teamClient?: DeviceDataDeleter;
  readonly onTeamDataDeleted?: () => void;
}

export function AboutScreen({
  backHref,
  onBack,
  focusOnMount,
  recordClient,
  teamClient,
  onTeamDataDeleted,
}: AboutScreenProps) {
  const headingRef = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    if (focusOnMount) {
      headingRef.current?.focus();
    }
    // 開いた時点の1回だけ(以後の再描画でフォーカスを奪わない)。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <section className="about">
      <h2 ref={headingRef} tabIndex={-1} className="about__heading">
        {aboutText.pageHeading}
      </h2>
      <section className="about__section">
        <h3>{aboutText.unofficialHeading}</h3>
        <p>{aboutText.unofficialNotice}</p>
      </section>
      <section className="about__section">
        <h3>{aboutText.dataSourcesHeading}</h3>
        <ul aria-label={aboutText.dataSourcesHeading} className="about__sources">
          {aboutText.dataSources.map((source) => (
            <li key={source.title}>
              <span className="about__source-title">{source.title}</span> {source.detail}
            </li>
          ))}
        </ul>
      </section>
      {recordClient !== undefined && teamClient !== undefined && onTeamDataDeleted !== undefined && (
        <DeviceDataSection
          recordClient={recordClient}
          teamClient={teamClient}
          onTeamDataDeleted={onTeamDataDeleted}
        />
      )}
      <a
        href={backHref}
        className="about__back"
        onClick={(event) => {
          if (
            event.defaultPrevented ||
            event.button !== 0 ||
            event.metaKey ||
            event.ctrlKey ||
            event.shiftKey
          ) {
            return;
          }
          event.preventDefault();
          onBack();
        }}
      >
        {aboutText.backLabel}
      </a>
    </section>
  );
}
