package pet

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"studybuddy-ai/internal/database"
)

// Manager バーチャルペット管理システム
type Manager struct {
	db *database.DB
}

// StudyResult 学習結果
type StudyResult struct {
	IsCorrect          bool    `json:"is_correct"`
	Difficulty         int     `json:"difficulty"`
	TimeTaken          int     `json:"time_taken"`         // 秒
	ConsecutiveCorrect int     `json:"consecutive_correct"`
	SubjectProgress    float64 `json:"subject_progress"`
	SessionDuration    int     `json:"session_duration"`   // 秒
}

// PetAction ペットのアクション
type PetAction struct {
	Type        string `json:"type"`        // "level_up", "evolution", "happy", "sad", etc.
	Message     string `json:"message"`     // ペットからのメッセージ
	Emoji       string `json:"emoji"`       // 表示する絵文字
	Sound       string `json:"sound"`       // 効果音（オプション）
	Animation   string `json:"animation"`   // アニメーション（オプション）
}

// EvolutionInfo 進化情報
type EvolutionInfo struct {
	RequiredLevel int    `json:"required_level"`
	FromStage     string `json:"from_stage"`
	ToStage       string `json:"to_stage"`
	Description   string `json:"description"`
}

// PetStats ペットの詳細ステータス
type PetStats struct {
	Pet              *database.VirtualPet `json:"pet"`
	NextLevelExp     int                  `json:"next_level_exp"`
	ExpToNextLevel   int                  `json:"exp_to_next_level"`
	DaysOld          int                  `json:"days_old"`
	HealthStatus     string               `json:"health_status"`
	HappinessStatus  string               `json:"happiness_status"`
	IntelligenceRank string               `json:"intelligence_rank"`
	NextEvolution    *EvolutionInfo       `json:"next_evolution"`
}

// NewManager ペット管理システムを作成
func NewManager(db *database.DB) *Manager {
	return &Manager{db: db}
}

// FeedPet 学習結果に基づいてペットに経験値を与える
func (m *Manager) FeedPet(userID string, result StudyResult) (*PetAction, error) {
	pet, err := m.db.GetVirtualPet(userID)
	if err != nil {
		return nil, fmt.Errorf("ペット取得エラー: %w", err)
	}

	// 経験値と幸福度の計算
	expGain := m.calculateExperience(result)
	happinessGain := m.calculateHappiness(result)
	healthChange := m.calculateHealthChange(result)

	// ステータス更新
	pet.Experience += expGain
	pet.Happiness = clamp(pet.Happiness+happinessGain, 0, 100)
	pet.Health = clamp(pet.Health+healthChange, 0, 100)
	pet.LastFed = &[]time.Time{time.Now()}[0]

	// レベルアップ判定
	levelUpAction := m.checkLevelUp(pet)
	
	// 進化判定
	evolutionAction := m.checkEvolution(pet)

	// ペット情報を更新
	if err := m.db.UpdateVirtualPet(pet); err != nil {
		return nil, fmt.Errorf("ペット更新エラー: %w", err)
	}

	// アクションの決定（優先度：進化 > レベルアップ > 通常フィードバック）
	if evolutionAction != nil {
		return evolutionAction, nil
	}
	if levelUpAction != nil {
		return levelUpAction, nil
	}

	// 通常のフィードバック
	return m.generateFeedbackAction(pet, result), nil
}

// calculateExperience 経験値を計算
func (m *Manager) calculateExperience(result StudyResult) int {
	baseExp := 10

	// 正解ボーナス
	if result.IsCorrect {
		baseExp += 15
	} else {
		baseExp += 5 // 間違いでも少し経験値
	}

	// 難易度ボーナス
	baseExp += result.Difficulty * 3

	// 連続正解ボーナス
	if result.ConsecutiveCorrect > 1 {
		baseExp += int(math.Min(float64(result.ConsecutiveCorrect*2), 20))
	}

	// 時間ボーナス（適切な時間で回答した場合）
	if result.TimeTaken > 30 && result.TimeTaken < 300 { // 30秒～5分
		baseExp += 5
	}

	// 学習継続ボーナス（長時間の学習セッション）
	if result.SessionDuration > 600 { // 10分以上
		baseExp += 10
	}

	return baseExp
}

// calculateHappiness 幸福度の変化を計算
func (m *Manager) calculateHappiness(result StudyResult) int {
	happiness := 0

	if result.IsCorrect {
		happiness += 5
		// 連続正解でさらにボーナス
		if result.ConsecutiveCorrect > 2 {
			happiness += 3
		}
	} else {
		happiness -= 2 // 間違いでも大きく下がらない
	}

	// 難易度に挑戦した場合の幸福度
	if result.Difficulty >= 4 {
		happiness += 2 // 挑戦する姿勢を評価
	}

	return happiness
}

// calculateHealthChange 健康度の変化を計算
func (m *Manager) calculateHealthChange(result StudyResult) int {
	health := 1 // 基本的に学習すると健康度が上がる

	// 長時間学習での疲労
	if result.SessionDuration > 1800 { // 30分以上
		health -= 3
	} else if result.SessionDuration > 3600 { // 1時間以上
		health -= 5
	}

	// 適度な学習時間でのボーナス
	if result.SessionDuration > 300 && result.SessionDuration <= 1800 {
		health += 2
	}

	return health
}

// checkLevelUp レベルアップをチェック
func (m *Manager) checkLevelUp(pet *database.VirtualPet) *PetAction {
	requiredExp := m.getRequiredExp(pet.Level)
	
	if pet.Experience >= requiredExp {
		pet.Level++
		pet.Experience = 0 // 経験値リセット
		pet.Intelligence += 5 // レベルアップで知性も上昇

		return &PetAction{
			Type:      "level_up",
			Message:   fmt.Sprintf("🎉 %sがレベル%dに上がりました！", pet.Name, pet.Level),
			Emoji:     "✨",
			Animation: "level_up",
		}
	}

	return nil
}

// checkEvolution 進化をチェック
func (m *Manager) checkEvolution(pet *database.VirtualPet) *PetAction {
	evolutionInfo := m.getEvolutionRequirements(pet.Species, pet.Evolution)
	
	if evolutionInfo != nil && pet.Level >= evolutionInfo.RequiredLevel {
		pet.Evolution = evolutionInfo.ToStage
		
		// 進化時のステータスボーナス
		pet.Health = 100
		pet.Happiness = 100
		pet.Intelligence += 10

		return &PetAction{
			Type:      "evolution",
			Message:   fmt.Sprintf("🌟 すごい！%sが%sに進化しました！", pet.Name, evolutionInfo.Description),
			Emoji:     "🌟",
			Animation: "evolution",
		}
	}

	return nil
}

// generateFeedbackAction 通常のフィードバックアクションを生成
func (m *Manager) generateFeedbackAction(pet *database.VirtualPet, result StudyResult) *PetAction {
	messages := m.getPetMessages(pet.Species, result.IsCorrect)
	message := messages[rand.Intn(len(messages))]

	emoji := m.getPetEmoji(pet.Species)
	if result.IsCorrect {
		emoji += "💖"
	}

	actionType := "happy"
	if !result.IsCorrect {
		actionType = "encouraging"
	}

	return &PetAction{
		Type:    actionType,
		Message: fmt.Sprintf("%s: %s", pet.Name, message),
		Emoji:   emoji,
	}
}

// getRequiredExp レベルアップに必要な経験値を計算
func (m *Manager) getRequiredExp(level int) int {
	// レベルが上がるほど必要経験値が増加
	return 100 + (level-1)*50
}

// getEvolutionRequirements 進化の要件を取得
func (m *Manager) getEvolutionRequirements(species, currentStage string) *EvolutionInfo {
	evolutionMap := map[string]map[string]*EvolutionInfo{
		"cat": {
			"basic": {
				RequiredLevel: 5,
				FromStage:     "basic",
				ToStage:       "intermediate",
				Description:   "賢いネコ",
			},
			"intermediate": {
				RequiredLevel: 15,
				FromStage:     "intermediate",
				ToStage:       "advanced",
				Description:   "学者ネコ",
			},
		},
		"dog": {
			"basic": {
				RequiredLevel: 5,
				FromStage:     "basic",
				ToStage:       "intermediate",
				Description:   "忠実なワンコ",
			},
			"intermediate": {
				RequiredLevel: 15,
				FromStage:     "intermediate",
				ToStage:       "advanced",
				Description:   "博士ワンコ",
			},
		},
		"dragon": {
			"basic": {
				RequiredLevel: 8,
				FromStage:     "basic",
				ToStage:       "intermediate",
				Description:   "知恵のドラゴン",
			},
			"intermediate": {
				RequiredLevel: 20,
				FromStage:     "intermediate",
				ToStage:       "advanced",
				Description:   "古代ドラゴン",
			},
		},
		"unicorn": {
			"basic": {
				RequiredLevel: 10,
				FromStage:     "basic",
				ToStage:       "intermediate",
				Description:   "魔法のユニコーン",
			},
			"intermediate": {
				RequiredLevel: 25,
				FromStage:     "intermediate",
				ToStage:       "advanced",
				Description:   "伝説のユニコーン",
			},
		},
	}

	if speciesEvolutions, exists := evolutionMap[species]; exists {
		return speciesEvolutions[currentStage]
	}
	return nil
}

// getPetMessages ペットの種類に応じたメッセージを取得
func (m *Manager) getPetMessages(species string, isCorrect bool) []string {
	messageMap := map[string]map[bool][]string{
		"cat": {
			true: {
				"にゃ〜ん！すごいじゃない！",
				"完璧な回答だニャ！",
				"君は天才だニャ〜",
				"その調子で頑張るニャ！",
			},
			false: {
				"大丈夫ニャ、次は一緒に頑張ろう",
				"間違いは成長のチャンスだニャ",
				"ゆっくり考えてみるニャ",
				"君ならできるニャ〜",
			},
		},
		"dog": {
			true: {
				"ワンワン！素晴らしいワン！",
				"君は僕の誇りだワン！",
				"一緒に喜ぼうワン！",
				"最高の相棒だワン！",
			},
			false: {
				"大丈夫ワン、僕がついてるワン",
				"次は一緒にがんばろうワン",
				"君を信じてるワン！",
				"失敗なんて気にしないワン",
			},
		},
		"dragon": {
			true: {
				"我が友よ、見事な知恵の働きじゃ",
				"真の学者の資質を見せたな",
				"その探究心、実に素晴らしい",
				"知識の炎が燃え上がっておるな",
			},
			false: {
				"心配無用じゃ、学びは続く",
				"失敗こそが真の知恵への道",
				"次の挑戦で実力を示すがよい",
				"我が友の可能性は無限大じゃ",
			},
		},
		"unicorn": {
			true: {
				"魔法のような回答でした✨",
				"あなたの心の美しさが現れています",
				"希望の光が輝いていますね",
				"純粋な心で学ぶ姿が美しいです",
			},
			false: {
				"大丈夫、あなたの心は美しいままです",
				"希望を失わずに進みましょう",
				"困難を乗り越える力があります",
				"信じる心が奇跡を起こします",
			},
		},
	}

	if messages, exists := messageMap[species]; exists {
		return messages[isCorrect]
	}

	// デフォルトメッセージ
	if isCorrect {
		return []string{"素晴らしい回答です！", "その調子で頑張りましょう！"}
	}
	return []string{"大丈夫、一緒に頑張りましょう", "次はきっとできますよ"}
}

// getPetEmoji ペットの絵文字を取得
func (m *Manager) getPetEmoji(species string) string {
	emojiMap := map[string]string{
		"cat":     "🐱",
		"dog":     "🐶",
		"dragon":  "🐉",
		"unicorn": "🦄",
	}

	if emoji, exists := emojiMap[species]; exists {
		return emoji
	}
	return "🐾"
}

// GetPetStats ペットの詳細ステータスを取得
func (m *Manager) GetPetStats(userID string) (*PetStats, error) {
	pet, err := m.db.GetVirtualPet(userID)
	if err != nil {
		return nil, fmt.Errorf("ペット取得エラー: %w", err)
	}

	nextLevelExp := m.getRequiredExp(pet.Level)
	expToNext := nextLevelExp - pet.Experience

	// ペットの年齢（日数）
	daysOld := int(time.Since(pet.CreatedAt).Hours() / 24)

	// ステータス評価
	healthStatus := m.getStatusDescription(pet.Health, "health")
	happinessStatus := m.getStatusDescription(pet.Happiness, "happiness")
	intelligenceRank := m.getIntelligenceRank(pet.Intelligence)

	// 次の進化情報
	nextEvolution := m.getEvolutionRequirements(pet.Species, pet.Evolution)

	return &PetStats{
		Pet:              pet,
		NextLevelExp:     nextLevelExp,
		ExpToNextLevel:   expToNext,
		DaysOld:          daysOld,
		HealthStatus:     healthStatus,
		HappinessStatus:  happinessStatus,
		IntelligenceRank: intelligenceRank,
		NextEvolution:    nextEvolution,
	}, nil
}

// getStatusDescription ステータスの説明を取得
func (m *Manager) getStatusDescription(value int, statType string) string {
	descriptions := map[string]map[string]string{
		"health": {
			"excellent": "絶好調",
			"good":      "元気",
			"fair":      "普通",
			"poor":      "疲れ気味",
			"bad":       "要注意",
		},
		"happiness": {
			"excellent": "大喜び",
			"good":      "ご機嫌",
			"fair":      "普通",
			"poor":      "少し不機嫌",
			"bad":       "落ち込み中",
		},
	}

	var level string
	switch {
	case value >= 90:
		level = "excellent"
	case value >= 70:
		level = "good"
	case value >= 50:
		level = "fair"
	case value >= 30:
		level = "poor"
	default:
		level = "bad"
	}

	if desc, exists := descriptions[statType][level]; exists {
		return desc
	}
	return "普通"
}

// getIntelligenceRank 知性ランクを取得
func (m *Manager) getIntelligenceRank(intelligence int) string {
	switch {
	case intelligence >= 90:
		return "天才級"
	case intelligence >= 80:
		return "優秀"
	case intelligence >= 70:
		return "賢い"
	case intelligence >= 60:
		return "普通+"
	case intelligence >= 50:
		return "普通"
	default:
		return "成長中"
	}
}

// PlayWithPet ペットと遊ぶ（幸福度上昇）
func (m *Manager) PlayWithPet(userID string) (*PetAction, error) {
	pet, err := m.db.GetVirtualPet(userID)
	if err != nil {
		return nil, fmt.Errorf("ペット取得エラー: %w", err)
	}

	// 遊び時間の制限チェック
	if pet.LastPlayed != nil {
		timeSinceLastPlay := time.Since(*pet.LastPlayed)
		if timeSinceLastPlay < 30*time.Minute {
			return &PetAction{
				Type:    "wait",
				Message: fmt.Sprintf("%sはまだ疲れています。もう少し待ってから遊びましょう", pet.Name),
				Emoji:   "😴",
			}, nil
		}
	}

	// 幸福度と健康度をアップ
	pet.Happiness = clamp(pet.Happiness+10, 0, 100)
	pet.Health = clamp(pet.Health+5, 0, 100)
	pet.LastPlayed = &[]time.Time{time.Now()}[0]

	if err := m.db.UpdateVirtualPet(pet); err != nil {
		return nil, fmt.Errorf("ペット更新エラー: %w", err)
	}

	playMessages := []string{
		"楽しい時間を過ごしました！",
		"一緒に遊べて幸せです！",
		"とても楽しかったです！",
		"もっと遊びたいな〜",
	}

	return &PetAction{
		Type:    "play",
		Message: fmt.Sprintf("%s: %s", pet.Name, playMessages[rand.Intn(len(playMessages))]),
		Emoji:   m.getPetEmoji(pet.Species) + "✨",
	}, nil
}

// HealPet ペットの健康度を回復
func (m *Manager) HealPet(userID string) error {
	pet, err := m.db.GetVirtualPet(userID)
	if err != nil {
		return fmt.Errorf("ペット取得エラー: %w", err)
	}

	// 自動回復（時間経過）
	if pet.LastFed != nil {
		hoursSinceLastFed := time.Since(*pet.LastFed).Hours()
		healAmount := int(hoursSinceLastFed / 4) // 4時間ごとに1ポイント回復
		pet.Health = clamp(pet.Health+healAmount, 0, 100)
	}

	return m.db.UpdateVirtualPet(pet)
}

// RenamePet ペットの名前を変更
func (m *Manager) RenamePet(userID, newName string) error {
	if len(newName) == 0 || len(newName) > 20 {
		return fmt.Errorf("ペットの名前は1〜20文字で入力してください")
	}

	pet, err := m.db.GetVirtualPet(userID)
	if err != nil {
		return fmt.Errorf("ペット取得エラー: %w", err)
	}

	pet.Name = newName
	return m.db.UpdateVirtualPet(pet)
}

// GetDailyMessage 日替わりメッセージを取得
func (m *Manager) GetDailyMessage(userID string) (string, error) {
	pet, err := m.db.GetVirtualPet(userID)
	if err != nil {
		return "", fmt.Errorf("ペット取得エラー: %w", err)
	}

	// 日付ベースのランダムソース
	today := time.Now().Format("2006-01-02")
	source := rand.NewSource(int64(hashString(today + userID)))
	rng := rand.New(source)

	messages := []string{
		"今日も一緒に頑張りましょう！",
		"新しいことを学ぶ準備はできていますか？",
		"今日はどの科目から始めますか？",
		"一歩ずつ成長していきましょう",
		"今日も素敵な一日にしましょう！",
		"学習する時間ですね！",
		"一緒に知識の旅に出かけましょう",
		"今日の学習目標を決めましょう",
	}

	selectedMessage := messages[rng.Intn(len(messages))]
	return fmt.Sprintf("%s %s: %s", m.getPetEmoji(pet.Species), pet.Name, selectedMessage), nil
}

// clamp 値を範囲内に制限
func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// hashString 文字列の簡単なハッシュ値を計算
func hashString(s string) int {
	hash := 0
	for _, c := range s {
		hash = hash*31 + int(c)
	}
	return hash
}

// Close ペット管理システムをクリーンアップ
func (m *Manager) Close() error {
	// 特にクリーンアップすることはない
	return nil
}
