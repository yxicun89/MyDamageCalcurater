package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

// MaxMembersPerTeam / MaxMovesPerMember は構築の形の上限(契約の maxItems と同じ値。
// requirements.md §2・ADR-0213 §3。iOS の TeamLimits と同じ)。
const (
	MaxMembersPerTeam = 6
	MaxMovesPerMember = 4
)

// 構築名・ニックネームの文字数、SP の範囲と合計(契約の maxItems/maxLength と
// 同じ値。requirements.md §2・ADR-0213 §2・§3)。
const (
	MaxTeamNameRunes = 50
	// UnsetTeamName は名前を省略・空にした構築に補う既定名(ADR-0229)。
	UnsetTeamName    = "名称未設定"
	MaxNicknameRunes = 24
	MaxSPPerStat     = 32
	MaxSPTotal       = 66

	// MaxSpeciesKeyRunes・MaxIDRunes・MaxTeraTypeRunes は
	// db/migrations/000004_create_team_members.up.sql の列幅とちょうど対応する上限
	// (critic 指摘 重要2)。ここで弾かないと、超過した入力が INSERT の "Data too long" で失敗し、
	// store の全 SQL エラーを ErrUnavailable に包む wrapUnavailable(adapters/store/tidb.go)を経由して
	// 503 store_unavailable(可用性シグナルの誤発火)になってしまう。マスタに実在するかは見ない
	// (ADR-0213 §3)ので、ここは長さだけを見る。
	MaxSpeciesKeyRunes = 16 // species_key VARCHAR(16)
	MaxIDRunes         = 64 // item_id / ability_id / nature_id VARCHAR(64)。moveIds の各要素にも適用する
	MaxTeraTypeRunes   = 16 // tera_type VARCHAR(16)
)

// StatBlock は能力ポイント(SP)の6ステータス(api.StatBlock と同じ形。ADR-0213 §3)。
// 値の範囲(各 0..32・合計 66 以下)の検証は ValidateTeamInput の責務で、store は受けた値をそのまま保存する。
type StatBlock struct {
	HP  int
	Atk int
	Def int
	Spa int
	Spd int
	Spe int
}

// Member は構築の1体。ID(種族・技・持ち物・特性・性格)はマスタに実在するか検証せずそのまま持つ
// (ADR-0213 §3)。パーティの何番目かは Team.Members の並び順そのもので、slot は持たない
// (DB の team_members.slot は保存時に添字から決める)。
type Member struct {
	SpeciesKey string
	// Nickname は未設定なら nil(空文字は ValidateTeamInput が nil に正規化する)。利用者の自由入力なので
	// **ログに出さない**(ADR-0209 §3)。
	Nickname *string
	// MoveIDs は最大 MaxMovesPerMember 個で、同一メンバー内で重複しない(検証は ValidateTeamInput)。
	MoveIDs   []string
	ItemID    *string
	AbilityID *string
	NatureID  string
	SP        StatBlock
	// TeraType は api.PokeType の文字列(未設定なら nil)。
	TeraType *string
}

// Team は保存済みの構築1件。ID と時刻はサーバー(store)が決める。
type Team struct {
	ID        string
	Name      string
	Members   []Member
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TeamInput は構築の作成・置換の受け口(契約の TeamInput と同じ形。省略は nil)。
// adapters/httpapi が契約の型から写す。
type TeamInput struct {
	Name    *string
	Members *[]MemberInput
}

// MemberInput は TeamInput のメンバー1体(契約の TeamMember と同じ形。省略は nil)。
type MemberInput struct {
	SpeciesKey string
	Nickname   *string
	MoveIDs    *[]string
	ItemID     *string
	AbilityID  *string
	NatureID   string
	SP         StatBlock
	TeraType   *string
}

// ValidateTeamInput は「マスタを引かなくても判断できる」検証だけを行う(ADR-0213 §3)。
// 通れば Member のスライスと正規化した名前(前後の空白を除いたもの。空なら既定名)を返す。
// 失敗は *Error(CodeInvalidInput)。
func ValidateTeamInput(in TeamInput) (name string, members []Member, err error) {
	if in.Name != nil {
		name = strings.TrimSpace(*in.Name)
	}
	if name == "" {
		name = UnsetTeamName
	}
	if n := utf8.RuneCountInString(name); n > MaxTeamNameRunes {
		return "", nil, Errorf(CodeInvalidInput, "構築名は%d文字まで(%d文字)", MaxTeamNameRunes, n)
	}

	var raw []MemberInput
	if in.Members != nil {
		raw = *in.Members
	}
	if len(raw) > MaxMembersPerTeam {
		return "", nil, Errorf(CodeInvalidInput, "メンバーは最大%d体まで(%d体)", MaxMembersPerTeam, len(raw))
	}

	members = make([]Member, 0, len(raw))
	for i, m := range raw {
		sm, err := validateMember(i, m)
		if err != nil {
			return "", nil, err
		}
		members = append(members, sm)
	}
	return name, members, nil
}

func validateMember(index int, m MemberInput) (Member, error) {
	speciesKey := m.SpeciesKey
	if strings.TrimSpace(speciesKey) == "" {
		return Member{}, Errorf(CodeInvalidInput, "メンバー%d: speciesKey が空", index+1)
	}
	if n := utf8.RuneCountInString(speciesKey); n > MaxSpeciesKeyRunes {
		return Member{}, Errorf(CodeInvalidInput, "メンバー%d: speciesKey は%d文字まで(%d文字)", index+1, MaxSpeciesKeyRunes, n)
	}
	natureID := m.NatureID
	if strings.TrimSpace(natureID) == "" {
		return Member{}, Errorf(CodeInvalidInput, "メンバー%d: natureId が空", index+1)
	}
	if n := utf8.RuneCountInString(natureID); n > MaxIDRunes {
		return Member{}, Errorf(CodeInvalidInput, "メンバー%d: natureId は%d文字まで(%d文字)", index+1, MaxIDRunes, n)
	}

	var moveIDs []string
	if m.MoveIDs != nil {
		moveIDs = *m.MoveIDs
	}
	if len(moveIDs) > MaxMovesPerMember {
		return Member{}, Errorf(CodeInvalidInput, "メンバー%d: 技は最大%d個まで(%d個)", index+1, MaxMovesPerMember, len(moveIDs))
	}
	seen := make(map[string]bool, len(moveIDs))
	for _, id := range moveIDs {
		if seen[id] {
			return Member{}, Errorf(CodeInvalidInput, "メンバー%d: 技が重複している(%q)", index+1, id)
		}
		seen[id] = true
		if n := utf8.RuneCountInString(id); n > MaxIDRunes {
			return Member{}, Errorf(CodeInvalidInput, "メンバー%d: moveIds の要素は%d文字まで(%q)", index+1, MaxIDRunes, id)
		}
	}

	var nickname *string
	if m.Nickname != nil && *m.Nickname != "" {
		if n := utf8.RuneCountInString(*m.Nickname); n > MaxNicknameRunes {
			return Member{}, Errorf(CodeInvalidInput, "メンバー%d: ニックネームは%d文字まで", index+1, MaxNicknameRunes)
		}
		nickname = m.Nickname
	}

	if m.ItemID != nil {
		if n := utf8.RuneCountInString(*m.ItemID); n > MaxIDRunes {
			return Member{}, Errorf(CodeInvalidInput, "メンバー%d: itemId は%d文字まで(%d文字)", index+1, MaxIDRunes, n)
		}
	}
	if m.AbilityID != nil {
		if n := utf8.RuneCountInString(*m.AbilityID); n > MaxIDRunes {
			return Member{}, Errorf(CodeInvalidInput, "メンバー%d: abilityId は%d文字まで(%d文字)", index+1, MaxIDRunes, n)
		}
	}

	if err := validateSP(index, m.SP); err != nil {
		return Member{}, err
	}

	var tera *string
	if m.TeraType != nil {
		v := *m.TeraType
		if n := utf8.RuneCountInString(v); n > MaxTeraTypeRunes {
			return Member{}, Errorf(CodeInvalidInput, "メンバー%d: teraType は%d文字まで(%d文字)", index+1, MaxTeraTypeRunes, n)
		}
		tera = &v
	}

	return Member{
		SpeciesKey: speciesKey,
		Nickname:   nickname,
		MoveIDs:    moveIDs,
		ItemID:     m.ItemID,
		AbilityID:  m.AbilityID,
		NatureID:   natureID,
		SP:         m.SP,
		TeraType:   tera,
	}, nil
}

// validateSP は Individual.sp と同じ規則(各 0..32、合計 <= 66。requirements.md・ADR-0213 §3)。
func validateSP(index int, sp StatBlock) error {
	stats := []struct {
		name string
		val  int
	}{
		{"hp", sp.HP}, {"atk", sp.Atk}, {"def", sp.Def}, {"spa", sp.Spa}, {"spd", sp.Spd}, {"spe", sp.Spe},
	}
	total := 0
	for _, s := range stats {
		if s.val < 0 || s.val > MaxSPPerStat {
			return Errorf(CodeInvalidInput, "メンバー%d: sp.%s は0〜%dの範囲であること(%d)", index+1, s.name, MaxSPPerStat, s.val)
		}
		total += s.val
	}
	if total > MaxSPTotal {
		return Errorf(CodeInvalidInput, "メンバー%d: sp の合計は%d以下であること(%d)", index+1, MaxSPTotal, total)
	}
	return nil
}
