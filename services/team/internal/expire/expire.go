// Package expire は team-svc の失効ジョブ(ADR-0209 §4・ADR-0220 §3・§4)。
//
// 全端末を横断して期限切れの行を消すので、「全メソッドが deviceID を取る」internal/store の Store には
// 足さず、失効専用の Store(このパッケージ)に置く。HTTP・NATS・他サービスには依存しない。
package expire

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// JetStreamMaxAge は calc-svc が発行する CALC_EVENTS ストリームの max_age(固定7日。ADR-0212 §4)。
// DeviceRowExpiry はこれより大きいこと(墓石の猶予。ADR-0209 §7)。
const JetStreamMaxAge = 7 * 24 * time.Hour

// Policy は失効の保持期間と、1回の実行で消す構築数の上限。
type Policy struct {
	Retention             time.Duration
	DeviceRowExpiry       time.Duration
	PurgeJournalRetention time.Duration
	BatchLimit            int
}

// Validate は Policy が失効に使える値かを返す。
func (p Policy) Validate() error {
	if p.Retention <= 0 {
		return fmt.Errorf("expire: Retention は正でなければならない(%s)", p.Retention)
	}
	if p.PurgeJournalRetention <= 0 {
		return fmt.Errorf("expire: PurgeJournalRetention は正でなければならない(%s)", p.PurgeJournalRetention)
	}
	if p.DeviceRowExpiry <= JetStreamMaxAge {
		return fmt.Errorf("expire: DeviceRowExpiry(%s)は JetStream の max_age(%s)より大きくなければならない",
			p.DeviceRowExpiry, JetStreamMaxAge)
	}
	if p.BatchLimit <= 0 {
		return fmt.Errorf("expire: BatchLimit は正でなければならない(%d)", p.BatchLimit)
	}
	return nil
}

// Result は1回の実行で消した件数。Remaining は上限に達して残りがありうることを表す(保守的に倒す)。
type Result struct {
	Teams, TeamMembers, PurgeJournal, Devices int
	OrphansMarked, OrphansCleared             int
	Remaining                                 bool
}

// Store は失効専用の操作。Delete* は「判定時刻 < cutoff」の行を最大 limit 件消す(ちょうど cutoff は消さない)。
type Store interface {
	// DeleteInactiveTeams は max(devices.last_seen_at, teams.updated_at) < cutoff の構築を最大 limit 件、
	// メンバーを先に同じトランザクションで消す。消した構築数とメンバー数を返す。
	DeleteInactiveTeams(ctx context.Context, cutoff time.Time, limit int) (teams, members int, err error)
	DeletePurgeJournal(ctx context.Context, cutoff time.Time, limit int) (int, error)
	MarkOrphans(ctx context.Context, now time.Time, limit int) (marked, cleared int, err error)
	DeleteOrphanDevices(ctx context.Context, cutoff time.Time, limit int) (int, error)
}

// Run は ADR-0220 §4 の順に1巡だけ実行し、終わりに件数をログに1行出す。
// 削除の上限(Policy.BatchLimit。構築の件数で数える)は段で共有する。失敗した段より後は呼ばず、
// それまでの件数を返す。
func Run(ctx context.Context, st Store, p Policy, now time.Time, log *slog.Logger) (Result, error) {
	if err := p.Validate(); err != nil {
		return Result{}, err
	}
	start := time.Now()
	var res Result
	budget := p.BatchLimit

	// step は上限が残っている間だけ fn を呼び、消した件数を budget から引く。
	step := func(name string, fn func(limit int) (int, error)) error {
		if budget <= 0 {
			res.Remaining = true
			return nil
		}
		limit := budget
		n, err := fn(limit)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		budget -= n
		if n >= limit {
			res.Remaining = true
		}
		return nil
	}

	steps := []func() error{
		func() error {
			return step("DeleteInactiveTeams", func(l int) (int, error) {
				teams, members, err := st.DeleteInactiveTeams(ctx, now.Add(-p.Retention), l)
				res.Teams, res.TeamMembers = teams, members
				return teams, err
			})
		},
		func() error {
			return step("DeletePurgeJournal", func(l int) (int, error) {
				n, err := st.DeletePurgeJournal(ctx, now.Add(-p.PurgeJournalRetention), l)
				res.PurgeJournal = n
				return n, err
			})
		},
		func() error {
			// 印付けは削除の上限とは別枠(同じ値を上限にする)。
			marked, cleared, err := st.MarkOrphans(ctx, now, p.BatchLimit)
			if err != nil {
				return fmt.Errorf("MarkOrphans: %w", err)
			}
			res.OrphansMarked, res.OrphansCleared = marked, cleared
			if marked >= p.BatchLimit || cleared >= p.BatchLimit {
				res.Remaining = true
			}
			return nil
		},
		func() error {
			return step("DeleteOrphanDevices", func(l int) (int, error) {
				n, err := st.DeleteOrphanDevices(ctx, now.Add(-p.DeviceRowExpiry), l)
				res.Devices = n
				return n, err
			})
		},
	}
	for _, s := range steps {
		if err := s(); err != nil {
			// 途中まで消した件数を残す(件数だけ。端末 ID・行の中身は出さない)。
			log.Error("expire failed", append(resultAttrs(res), "duration", time.Since(start))...)
			return res, err
		}
	}

	log.Info("expire done", append(resultAttrs(res), "duration", time.Since(start))...)
	return res, nil
}

// resultAttrs は件数のログ属性(端末 ID・行の中身は含めない)。
func resultAttrs(res Result) []any {
	return []any{
		"teams", res.Teams, "team_members", res.TeamMembers,
		"purge_journal", res.PurgeJournal, "devices", res.Devices,
		"orphans_marked", res.OrphansMarked, "orphans_cleared", res.OrphansCleared,
		"remaining", res.Remaining,
	}
}
