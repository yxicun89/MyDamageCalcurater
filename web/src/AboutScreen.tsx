// 「このアプリについて」情報ページ(issue 328 / P6-18、ADR-0314)。非公式の注記とデータの出典を出す。
// マスタ・engine は使わない(読み込み中・失敗中でも出す)。文言は i18n/ja.ts の aboutText が正。

import { useEffect, useRef } from "react";
import { OfflinePreloadSection, type OfflinePreloadDeps } from "./app/OfflinePreloadSection";
import { DeviceDataSection } from "./deviceData/DeviceDataSection";
import type { DeviceDataDeleter } from "./deviceData/deleteDeviceData";
import { aboutText } from "./i18n/ja";
import { Icon } from "./ui/Icon";

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
  /** ADR-0358 段 3: 「オフライン用にポケモンのデータを保存」節。渡されたときだけ出す。 */
  readonly offlinePreload?: OfflinePreloadDeps;
  /** 計算する場所がサーバーか(オフラインでは保存ボタンを押せなくする)。 */
  readonly online?: boolean;
  readonly onPreloadSaved?: () => void;
}

export function AboutScreen({
  backHref,
  onBack,
  focusOnMount,
  recordClient,
  teamClient,
  onTeamDataDeleted,
  offlinePreload,
  online = true,
  onPreloadSaved,
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
      <section className="ui-card about__section">
        <h3 className="about__section-heading">
          <Icon name="alert" size={24} />
          {aboutText.unofficialHeading}
        </h3>
        <p>{aboutText.unofficialNotice}</p>
      </section>
      <section className="ui-card about__section">
        <h3 className="about__section-heading">
          <Icon name="info" size={24} />
          {aboutText.dataSourcesHeading}
        </h3>
        <ul aria-label={aboutText.dataSourcesHeading} className="ui-rows about__sources">
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
      {offlinePreload !== undefined && (
        <OfflinePreloadSection online={online} preload={offlinePreload} onSaved={onPreloadSaved} />
      )}
      <a
        href={backHref}
        className="ui-button ui-button--secondary about__back"
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
