// P5-5d: 情報ページの「データの扱い」節(ADR-0318 §3・§7)。説明・削除ボタン・確認ダイアログ・結果表示。
// 手順は deviceData/deleteDeviceData.ts の純粋関数に任せ、ここは表示と操作(フォーカス・キー操作)だけを持つ。
// ローカルの状態(端末 ID・計算モード等)は触らない(ADR-0318 §4)。文言は i18n/ja.ts の deviceDataText。

import { useEffect, useId, useRef, useState, type KeyboardEvent } from "react";
import { deviceDataText } from "../i18n/ja";
import { Icon } from "../ui/Icon";
import {
  describeDeviceDataDeletion,
  runDeviceDataDeletion,
  type DeviceDataDeleter,
  type DeviceDataDeletionResult,
  type DeviceDataTarget,
} from "./deleteDeviceData";

export interface DeviceDataSectionProps {
  readonly recordClient: DeviceDataDeleter;
  readonly teamClient: DeviceDataDeleter;
  /** team が completed になったときの合図(App が開いたままの構築一覧を取り直す)。 */
  readonly onTeamDataDeleted: () => void;
}

/** completed でない対象(再試行で呼ぶ対象)。 */
function remainingTargets(result: DeviceDataDeletionResult): DeviceDataTarget[] {
  const targets: DeviceDataTarget[] = [];
  if (result.record.kind !== "completed") {
    targets.push("record");
  }
  if (result.team.kind !== "completed") {
    targets.push("team");
  }
  return targets;
}

export function DeviceDataSection({ recordClient, teamClient, onTeamDataDeleted }: DeviceDataSectionProps) {
  const [dialogOpen, setDialogOpen] = useState(false);
  const [running, setRunning] = useState(false);
  const [lastPartial, setLastPartial] = useState(false);
  const [result, setResult] = useState<DeviceDataDeletionResult | null>(null);
  const deleteButtonRef = useRef<HTMLButtonElement>(null);
  const cancelButtonRef = useRef<HTMLButtonElement>(null);
  const confirmButtonRef = useRef<HTMLButtonElement>(null);
  const abortRef = useRef<AbortController | null>(null);
  // state の running は次の描画まで古いので、同一フレームの2回押しを防ぐ同期のガード。
  const runningRef = useRef(false);
  const titleId = useId();
  const descriptionId = useId();

  // 画面を離れたら以後の要求を送らない。
  useEffect(
    () => () => {
      abortRef.current?.abort();
    },
    [],
  );

  // ダイアログを開いたら初期フォーカスはキャンセル(破壊的な「削除する」にしない)。
  useEffect(() => {
    if (dialogOpen) {
      cancelButtonRef.current?.focus();
    }
  }, [dialogOpen]);

  function closeDialog(): void {
    setDialogOpen(false);
    deleteButtonRef.current?.focus();
  }

  function onDialogKeyDown(event: KeyboardEvent<HTMLDivElement>): void {
    if (event.key === "Escape") {
      event.preventDefault();
      closeDialog();
      return;
    }
    if (event.key !== "Tab") {
      return;
    }
    // フォーカスはダイアログの2つのボタンの間で循環させる。
    const first = cancelButtonRef.current;
    const last = confirmButtonRef.current;
    if (first === null || last === null) {
      return;
    }
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  }

  async function run(
    targets: DeviceDataTarget[] | undefined,
    previous: DeviceDataDeletionResult | undefined,
  ) {
    if (runningRef.current) {
      return;
    }
    runningRef.current = true;
    const controller = new AbortController();
    abortRef.current = controller;
    setRunning(true);
    setResult(null);
    setLastPartial(false);
    const next = await runDeviceDataDeletion({
      record: recordClient,
      team: teamClient,
      targets,
      previous,
      signal: controller.signal,
      onProgress: (progress) => {
        if (!controller.signal.aborted) {
          setLastPartial(progress.lastResponsePartial);
        }
      },
    });
    runningRef.current = false;
    // team が消えていれば、画面を離れて中断した後でも合図する(setState だけなので unmount 後でも無害)。
    if (next.team.kind === "completed" && (targets === undefined || targets.includes("team"))) {
      onTeamDataDeleted();
    }
    if (controller.signal.aborted) {
      return;
    }
    setRunning(false);
    setLastPartial(false);
    setResult(next);
  }

  function confirmDeletion(): void {
    setDialogOpen(false);
    void run(undefined, undefined);
  }

  const message = result === null ? null : describeDeviceDataDeletion(result);
  const retryTargets = result === null ? [] : remainingTargets(result);

  return (
    <section className="ui-card about__section about__data">
      <h3 className="about__section-heading">
        <Icon name="about" size={24} />
        {deviceDataText.sectionHeading}
      </h3>
      <ul aria-label={deviceDataText.sectionHeading} className="ui-rows about__points">
        {deviceDataText.explanation.map((sentence) => (
          <li key={sentence}>{sentence}</li>
        ))}
      </ul>
      <div className="about__actions">
        <button
          ref={deleteButtonRef}
          type="button"
          className="ui-button ui-button--danger about__button about__button--danger"
          disabled={running}
          onClick={() => {
            setDialogOpen(true);
          }}
        >
          <Icon name="trash" size={20} />
          {deviceDataText.deleteButton}
        </button>
      </div>
      {running && (
        <p role="status" className="ui-notice ui-notice--loading about__status">
          {lastPartial ? deviceDataText.partialNotice : deviceDataText.deleting}
        </p>
      )}
      {!running && message !== null && message.tone !== "failure" && (
        <p role="status" className="ui-notice ui-notice--info about__status">
          {message.lines.join("")}
        </p>
      )}
      {!running && message !== null && message.tone === "failure" && (
        <div role="alert" className="ui-notice ui-notice--error about__status about__status--error">
          {message.lines.map((line) => (
            <p key={line}>{line}</p>
          ))}
        </div>
      )}
      {!running && message !== null && message.tone !== "success" && (
        <button
          type="button"
          className="ui-button ui-button--secondary about__button"
          onClick={() => {
            if (result !== null) {
              void run(retryTargets, result);
            }
          }}
        >
          {deviceDataText.retryButton}
        </button>
      )}
      {dialogOpen && (
        <div className="about__scrim">
          <div
            role="alertdialog"
            aria-modal="true"
            aria-labelledby={titleId}
            aria-describedby={descriptionId}
            className="ui-card about__dialog"
            onKeyDown={onDialogKeyDown}
          >
            <h4 id={titleId} className="about__dialog-title">
              <Icon name="alert" size={24} />
              {deviceDataText.deleteButton}
            </h4>
            <p id={descriptionId}>{deviceDataText.confirmMessage}</p>
            <div className="about__actions">
              <button
                ref={cancelButtonRef}
                type="button"
                className="ui-button ui-button--secondary about__button"
                onClick={closeDialog}
              >
                {deviceDataText.cancelAction}
              </button>
              <button
                ref={confirmButtonRef}
                type="button"
                className="ui-button ui-button--danger about__button about__button--danger"
                onClick={confirmDeletion}
              >
                <Icon name="trash" size={20} />
                {deviceDataText.confirmAction}
              </button>
            </div>
          </div>
        </div>
      )}
    </section>
  );
}
