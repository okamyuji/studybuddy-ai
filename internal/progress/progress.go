package progress

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"studybuddy-ai/internal/database"
)

// Manager 学習進捗管理システム
type Manager struct {
	db *database.DB
}

// LearningAnalysis 学習分析結果
type LearningAnalysis struct {
	UserID           string                    `json:"user_id"`
	OverallProgress  *OverallProgress          `json:"overall_progress"`
	SubjectProgress  map[string]*SubjectAnalysis `json:"subject_progress"`
	WeaknessAnalysis *WeaknessAnalysis         `json:"weakness_analysis"`
	StrengthAnalysis *StrengthAnalysis         `json:"strength_analysis"`
	Recommendations  []Recommendation          `json:"recommendations"`
	StudyStreak      *StudyStreakInfo          `json:"study_streak"`
	LastUpdated      time.Time                 `json:"last_updated"`
}

// OverallProgress 全体進捗
type OverallProgress struct {
	TotalStudyTime   int     `json:"total_study_time"`   // 秒
	TotalProblems    int     `json:"total_problems"`
	TotalCorrect     int     `json:"total_correct"`
	AccuracyRate     float64 `json:"accuracy_rate"`
	AverageSessionTime int   `json:"average_session_time"` // 秒
	StudyDaysCount   int     `json:"study_days_count"`
	CurrentLevel     int     `json:"current_level"`
	ExperiencePoints int     `json:"experience_points"`
}

// SubjectAnalysis 科目別分析
type SubjectAnalysis struct {
	Subject            string             `json:"subject"`
	AccuracyRate       float64            `json:"accuracy_rate"`
	TotalProblems      int                `json:"total_problems"`
	CorrectAnswers     int                `json:"correct_answers"`
	AverageTime        float64            `json:"average_time"`        // 秒
	ProgressLevel      int                `json:"progress_level"`      // 1-5
	StrengthAreas      []string           `json:"strength_areas"`
	WeaknessAreas      []string           `json:"weakness_areas"`
	RecentTrend        string             `json:"recent_trend"`        // "improving", "stable", "declining"
	DifficultyStats    map[int]DifficultyData `json:"difficulty_stats"`
	LastStudyDate      *time.Time         `json:"last_study_date"`
}

// DifficultyData 難易度別データ
type DifficultyData struct {
	Difficulty      int     `json:"difficulty"`
	ProblemsAttempted int   `json:"problems_attempted"`
	CorrectAnswers  int     `json:"correct_answers"`
	AccuracyRate    float64 `json:"accuracy_rate"`
	AverageTime     float64 `json:"average_time"`
}

// WeaknessAnalysis 弱点分析
type WeaknessAnalysis struct {
	TopWeaknesses    []WeaknessItem `json:"top_weaknesses"`
	ErrorPatterns    []ErrorPattern `json:"error_patterns"`
	RecommendedFocus []string       `json:"recommended_focus"`
}

// WeaknessItem 弱点項目
type WeaknessItem struct {
	Subject       string  `json:"subject"`
	ProblemType   string  `json:"problem_type"`
	AccuracyRate  float64 `json:"accuracy_rate"`
	ErrorCount    int     `json:"error_count"`
	Severity      string  `json:"severity"`      // "high", "medium", "low"
	Improvement   float64 `json:"improvement"`   // 改善度（%）
}

// ErrorPattern エラーパターン
type ErrorPattern struct {
	Type         string    `json:"type"`
	Description  string    `json:"description"`
	Frequency    int       `json:"frequency"`
	LastOccurred time.Time `json:"last_occurred"`
	IsActive     bool      `json:"is_active"`
}

// StrengthAnalysis 強み分析
type StrengthAnalysis struct {
	TopStrengths   []StrengthItem `json:"top_strengths"`
	ConsistentAreas []string      `json:"consistent_areas"`
	ImprovingAreas []string      `json:"improving_areas"`
}

// StrengthItem 強み項目
type StrengthItem struct {
	Subject      string  `json:"subject"`
	ProblemType  string  `json:"problem_type"`
	AccuracyRate float64 `json:"accuracy_rate"`
	Consistency  float64 `json:"consistency"`   // 一貫性スコア
	Growth       float64 `json:"growth"`        // 成長率
}

// Recommendation 学習推奨事項
type Recommendation struct {
	Type        string    `json:"type"`        // "focus_area", "difficulty_adjustment", "time_management", etc.
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Priority    string    `json:"priority"`    // "high", "medium", "low"
	Subject     string    `json:"subject"`
	Actions     []string  `json:"actions"`
	ExpectedEffect string `json:"expected_effect"`
}

// StudyStreakInfo 学習継続情報
type StudyStreakInfo struct {
	CurrentStreak    int       `json:"current_streak"`
	LongestStreak    int       `json:"longest_streak"`
	LastStudyDate    time.Time `json:"last_study_date"`
	StreakStartDate  time.Time `json:"streak_start_date"`
	StudyDaysThisWeek int      `json:"study_days_this_week"`
	StudyDaysThisMonth int     `json:"study_days_this_month"`
}

// SessionSummary セッション要約
type SessionSummary struct {
	SessionID        string    `json:"session_id"`
	Subject          string    `json:"subject"`
	StartTime        time.Time `json:"start_time"`
	Duration         int       `json:"duration"`         // 秒
	ProblemsAttempted int      `json:"problems_attempted"`
	CorrectAnswers   int       `json:"correct_answers"`
	AccuracyRate     float64   `json:"accuracy_rate"`
	AverageTime      float64   `json:"average_time"`     // 秒
	DominantEmotion  string    `json:"dominant_emotion"`
	Improvements     []string  `json:"improvements"`
	Challenges       []string  `json:"challenges"`
}

// NewManager プログレス管理システムを作成
func NewManager(db *database.DB) *Manager {
	return &Manager{db: db}
}

// UpdateProgress 学習セッション後の進捗更新
func (m *Manager) UpdateProgress(userID string, session *database.StudySession, results []database.ProblemResult) error {
	// 基本統計の更新
	if err := m.updateBasicStats(userID, session, results); err != nil {
		return fmt.Errorf("基本統計更新エラー: %w", err)
	}

	// 強み・弱み分析の更新
	if err := m.updateStrengthWeakness(userID, session.Subject, results); err != nil {
		return fmt.Errorf("強み・弱み分析更新エラー: %w", err)
	}

	// エラーパターンの更新
	if err := m.updateErrorPatterns(userID, session.Subject, results); err != nil {
		return fmt.Errorf("エラーパターン更新エラー: %w", err)
	}

	// 学習継続記録の更新
	if err := m.updateStudyStreak(userID); err != nil {
		return fmt.Errorf("学習継続記録更新エラー: %w", err)
	}

	return nil
}

// updateBasicStats 基本統計を更新
func (m *Manager) updateBasicStats(userID string, session *database.StudySession, results []database.ProblemResult) error {
	progress, err := m.db.GetLearningProgress(userID, session.Subject)
	if err != nil {
		return err
	}

	// セッション時間の計算
	sessionDuration := 0
	if session.EndTime != nil {
		sessionDuration = int(session.EndTime.Sub(session.StartTime).Seconds())
	}

	// 統計更新
	progress.TotalProblems += len(results)
	progress.TotalStudyTime += sessionDuration
	progress.LastStudyDate = &session.StartTime

	// 正解数更新
	correctCount := 0
	for _, result := range results {
		if result.IsCorrect {
			correctCount++
		}
	}
	progress.CorrectAnswers += correctCount

	// 学習継続日数の計算
	if progress.LastStudyDate != nil {
		yesterday := time.Now().AddDate(0, 0, -1)
		if progress.LastStudyDate.After(yesterday) {
			progress.StudyStreak++
		} else {
			progress.StudyStreak = 1 // リセット
		}
	} else {
		progress.StudyStreak = 1
	}

	return m.db.UpsertLearningProgress(progress)
}

// updateStrengthWeakness 強み・弱み分析を更新
func (m *Manager) updateStrengthWeakness(userID, subject string, results []database.ProblemResult) error {
	// 問題タイプ別の分析
	typeStats := make(map[string]struct {
		total   int
		correct int
	})

	for _, result := range results {
		stats := typeStats[result.ProblemType]
		stats.total++
		if result.IsCorrect {
			stats.correct++
		}
		typeStats[result.ProblemType] = stats
	}

	// 強み・弱みの識別
	var strengths, weaknesses []string
	for problemType, stats := range typeStats {
		accuracy := float64(stats.correct) / float64(stats.total)
		if accuracy >= 0.8 && stats.total >= 3 {
			strengths = append(strengths, problemType)
		} else if accuracy < 0.6 && stats.total >= 3 {
			weaknesses = append(weaknesses, problemType)
		}
	}

	// データベースに保存
	progress, err := m.db.GetLearningProgress(userID, subject)
	if err != nil {
		return err
	}

	strengthsJSON, _ := json.Marshal(strengths)
	weaknessesJSON, _ := json.Marshal(weaknesses)
	progress.StrengthAreas = string(strengthsJSON)
	progress.WeaknessAreas = string(weaknessesJSON)

	return m.db.UpsertLearningProgress(progress)
}

// updateErrorPatterns エラーパターンを更新
func (m *Manager) updateErrorPatterns(userID, subject string, results []database.ProblemResult) error {
	// TODO: エラーパターンテーブルの実装
	_ = userID    // 一時的に使用
	_ = subject   // 一時的に使用
	_ = results   // 一時的に使用
	return nil
}

// updateStudyStreak 学習継続記録を更新
func (m *Manager) updateStudyStreak(userID string) error {
	// 今日の学習記録があるかチェック
	today := time.Now().Format("2006-01-02")
	sessions, err := m.db.GetRecentStudySessions(userID, 30)
	if err != nil {
		return err
	}

	// 今日の学習があれば継続記録を更新
	for _, session := range sessions {
		if session.StartTime.Format("2006-01-02") == today {
			// 継続記録の処理
			break
		}
	}

	return nil
}

// AnalyzeProgress 総合的な学習進捗分析
func (m *Manager) AnalyzeProgress(userID string) (*LearningAnalysis, error) {
	analysis := &LearningAnalysis{
		UserID:          userID,
		SubjectProgress: make(map[string]*SubjectAnalysis),
		LastUpdated:     time.Now(),
	}

	// 全科目の学習進捗を取得
	subjects := []string{"数学", "英語", "国語", "理科", "社会"}
	
	// 全体進捗の計算
	overallProgress, err := m.calculateOverallProgress(userID, subjects)
	if err != nil {
		return nil, fmt.Errorf("全体進捗計算エラー: %w", err)
	}
	analysis.OverallProgress = overallProgress

	// 科目別分析
	for _, subject := range subjects {
		subjectAnalysis, err := m.analyzeSubjectProgress(userID, subject)
		if err != nil {
			continue // エラーがあってもスキップして続行
		}
		analysis.SubjectProgress[subject] = subjectAnalysis
	}

	// 弱点分析
	weaknessAnalysis, err := m.analyzeWeaknesses(userID)
	if err == nil {
		analysis.WeaknessAnalysis = weaknessAnalysis
	}

	// 強み分析
	strengthAnalysis, err := m.analyzeStrengths(userID)
	if err == nil {
		analysis.StrengthAnalysis = strengthAnalysis
	}

	// 推奨事項生成
	analysis.Recommendations = m.generateRecommendations(analysis)

	// 学習継続情報
	streakInfo, err := m.calculateStudyStreak(userID)
	if err == nil {
		analysis.StudyStreak = streakInfo
	}

	return analysis, nil
}

// calculateOverallProgress 全体進捗を計算
func (m *Manager) calculateOverallProgress(userID string, subjects []string) (*OverallProgress, error) {
	progress := &OverallProgress{}

	totalProblems := 0
	totalCorrect := 0
	totalStudyTime := 0
	studyDays := make(map[string]bool)

	for _, subject := range subjects {
		subjectProgress, err := m.db.GetLearningProgress(userID, subject)
		if err != nil {
			continue
		}

		totalProblems += subjectProgress.TotalProblems
		totalCorrect += subjectProgress.CorrectAnswers
		totalStudyTime += subjectProgress.TotalStudyTime

		if subjectProgress.LastStudyDate != nil {
			dateKey := subjectProgress.LastStudyDate.Format("2006-01-02")
			studyDays[dateKey] = true
		}
	}

	// 精度計算
	if totalProblems > 0 {
		progress.AccuracyRate = float64(totalCorrect) / float64(totalProblems)
	}

	progress.TotalProblems = totalProblems
	progress.TotalCorrect = totalCorrect
	progress.TotalStudyTime = totalStudyTime
	progress.StudyDaysCount = len(studyDays)

	// レベル計算（経験値ベース）
	progress.ExperiencePoints = totalCorrect * 10
	progress.CurrentLevel = 1 + (progress.ExperiencePoints / 1000)

	// 平均セッション時間
	if progress.StudyDaysCount > 0 {
		progress.AverageSessionTime = totalStudyTime / progress.StudyDaysCount
	}

	return progress, nil
}

// analyzeSubjectProgress 科目別進捗分析
func (m *Manager) analyzeSubjectProgress(userID, subject string) (*SubjectAnalysis, error) {
	progress, err := m.db.GetLearningProgress(userID, subject)
	if err != nil {
		return nil, err
	}

	analysis := &SubjectAnalysis{
		Subject:        subject,
		TotalProblems:  progress.TotalProblems,
		CorrectAnswers: progress.CorrectAnswers,
		LastStudyDate:  progress.LastStudyDate,
		DifficultyStats: make(map[int]DifficultyData),
	}

	// 精度計算
	if progress.TotalProblems > 0 {
		analysis.AccuracyRate = float64(progress.CorrectAnswers) / float64(progress.TotalProblems)
	}

	// 進捗レベル（1-5）
	analysis.ProgressLevel = m.calculateProgressLevel(analysis.AccuracyRate, progress.TotalProblems)

	// 強み・弱み領域の解析
	var strengths, weaknesses []string
	if progress.StrengthAreas != "" {
		_ = json.Unmarshal([]byte(progress.StrengthAreas), &strengths)
	}
	if progress.WeaknessAreas != "" {
		_ = json.Unmarshal([]byte(progress.WeaknessAreas), &weaknesses)
	}
	analysis.StrengthAreas = strengths
	analysis.WeaknessAreas = weaknesses

	// 最近のトレンド分析
	analysis.RecentTrend = m.calculateRecentTrend(userID, subject)

	return analysis, nil
}

// calculateProgressLevel 進捗レベルを計算
func (m *Manager) calculateProgressLevel(accuracyRate float64, totalProblems int) int {
	// 問題数と精度に基づくレベル計算
	level := 1

	if totalProblems >= 10 {
		level = 2
	}
	if totalProblems >= 50 && accuracyRate >= 0.7 {
		level = 3
	}
	if totalProblems >= 100 && accuracyRate >= 0.8 {
		level = 4
	}
	if totalProblems >= 200 && accuracyRate >= 0.85 {
		level = 5
	}

	return level
}

// calculateRecentTrend 最近のトレンドを計算
func (m *Manager) calculateRecentTrend(userID, subject string) string {
	// 最近のセッションを取得して傾向を分析
	sessions, err := m.db.GetRecentStudySessions(userID, 10)
	if err != nil || len(sessions) < 3 {
		return "stable"
	}

	// 科目に関連するセッションのみフィルタ
	var subjectSessions []database.StudySession
	for _, session := range sessions {
		if session.Subject == subject {
			subjectSessions = append(subjectSessions, session)
		}
	}

	if len(subjectSessions) < 3 {
		return "stable"
	}

	// 最近3セッションの精度を比較
	recentAccuracy := 0.0
	for _, session := range subjectSessions[:3] {
		if session.TotalProblems > 0 {
			accuracy := float64(session.CorrectAnswers) / float64(session.TotalProblems)
			recentAccuracy += accuracy
		}
	}
	recentAccuracy /= 3

	// 過去のセッションとの比較
	pastAccuracy := 0.0
	pastCount := 0
	for i := 3; i < len(subjectSessions) && i < 6; i++ {
		session := subjectSessions[i]
		if session.TotalProblems > 0 {
			accuracy := float64(session.CorrectAnswers) / float64(session.TotalProblems)
			pastAccuracy += accuracy
			pastCount++
		}
	}

	if pastCount > 0 {
		pastAccuracy /= float64(pastCount)
		
		if recentAccuracy > pastAccuracy+0.1 {
			return "improving"
		} else if recentAccuracy < pastAccuracy-0.1 {
			return "declining"
		}
	}

	return "stable"
}

// analyzeWeaknesses 弱点分析
func (m *Manager) analyzeWeaknesses(userID string) (*WeaknessAnalysis, error) {
	analysis := &WeaknessAnalysis{
		TopWeaknesses: []WeaknessItem{},
		ErrorPatterns: []ErrorPattern{},
	}

	subjects := []string{"数学", "英語", "国語", "理科", "社会"}
	
	for _, subject := range subjects {
		progress, err := m.db.GetLearningProgress(userID, subject)
		if err != nil {
			continue
		}

		if progress.TotalProblems > 0 {
			accuracy := float64(progress.CorrectAnswers) / float64(progress.TotalProblems)
			if accuracy < 0.7 {
				severity := "medium"
				if accuracy < 0.5 {
					severity = "high"
				} else if accuracy > 0.6 {
					severity = "low"
				}

				weakness := WeaknessItem{
					Subject:      subject,
					ProblemType:  subject + "_general",
					AccuracyRate: accuracy,
					ErrorCount:   progress.TotalProblems - progress.CorrectAnswers,
					Severity:     severity,
				}
				analysis.TopWeaknesses = append(analysis.TopWeaknesses, weakness)
			}
		}
	}

	// 弱点の重要度でソート
	sort.Slice(analysis.TopWeaknesses, func(i, j int) bool {
		return analysis.TopWeaknesses[i].AccuracyRate < analysis.TopWeaknesses[j].AccuracyRate
	})

	// 推奨フォーカス領域
	if len(analysis.TopWeaknesses) > 0 {
		analysis.RecommendedFocus = []string{analysis.TopWeaknesses[0].Subject}
	}

	return analysis, nil
}

// analyzeStrengths 強み分析
func (m *Manager) analyzeStrengths(userID string) (*StrengthAnalysis, error) {
	analysis := &StrengthAnalysis{
		TopStrengths:   []StrengthItem{},
		ConsistentAreas: []string{},
		ImprovingAreas: []string{},
	}

	subjects := []string{"数学", "英語", "国語", "理科", "社会"}
	
	for _, subject := range subjects {
		progress, err := m.db.GetLearningProgress(userID, subject)
		if err != nil {
			continue
		}

		if progress.TotalProblems >= 10 {
			accuracy := float64(progress.CorrectAnswers) / float64(progress.TotalProblems)
			if accuracy >= 0.8 {
				strength := StrengthItem{
					Subject:      subject,
					ProblemType:  subject + "_general",
					AccuracyRate: accuracy,
					Consistency:  m.calculateConsistency(userID, subject),
				}
				analysis.TopStrengths = append(analysis.TopStrengths, strength)
				
				if strength.Consistency >= 0.8 {
					analysis.ConsistentAreas = append(analysis.ConsistentAreas, subject)
				}
			}
		}
	}

	// 強みの精度でソート
	sort.Slice(analysis.TopStrengths, func(i, j int) bool {
		return analysis.TopStrengths[i].AccuracyRate > analysis.TopStrengths[j].AccuracyRate
	})

	return analysis, nil
}

// calculateConsistency 一貫性スコアを計算
func (m *Manager) calculateConsistency(userID, subject string) float64 {
	sessions, err := m.db.GetRecentStudySessions(userID, 10)
	if err != nil {
		return 0.5
	}

	var accuracies []float64
	for _, session := range sessions {
		if session.Subject == subject && session.TotalProblems > 0 {
			accuracy := float64(session.CorrectAnswers) / float64(session.TotalProblems)
			accuracies = append(accuracies, accuracy)
		}
	}

	if len(accuracies) < 3 {
		return 0.5
	}

	// 標準偏差を使用して一貫性を計算
	mean := 0.0
	for _, acc := range accuracies {
		mean += acc
	}
	mean /= float64(len(accuracies))

	variance := 0.0
	for _, acc := range accuracies {
		diff := acc - mean
		variance += diff * diff
	}
	variance /= float64(len(accuracies))
	stddev := math.Sqrt(variance)

	// 一貫性スコア（標準偏差が小さいほど高い）
	consistency := math.Max(0, 1.0-stddev*2)
	return consistency
}

// generateRecommendations 推奨事項を生成
func (m *Manager) generateRecommendations(analysis *LearningAnalysis) []Recommendation {
	var recommendations []Recommendation

	// 弱点に基づく推奨
	if analysis.WeaknessAnalysis != nil && len(analysis.WeaknessAnalysis.TopWeaknesses) > 0 {
		weakness := analysis.WeaknessAnalysis.TopWeaknesses[0]
		rec := Recommendation{
			Type:        "focus_area",
			Title:       fmt.Sprintf("%sの強化が必要です", weakness.Subject),
			Description: fmt.Sprintf("現在の正解率は%.1f%%です。集中的な学習で改善しましょう。", weakness.AccuracyRate*100),
			Priority:    weakness.Severity,
			Subject:     weakness.Subject,
			Actions: []string{
				"基礎問題から丁寧に復習する",
				"間違いやすいポイントをノートにまとめる",
				"毎日15分以上この科目に時間を割く",
			},
			ExpectedEffect: "2週間で正解率10%向上が期待できます",
		}
		recommendations = append(recommendations, rec)
	}

	// 学習時間に基づく推奨
	if analysis.OverallProgress != nil && analysis.OverallProgress.AverageSessionTime < 900 { // 15分未満
		rec := Recommendation{
			Type:        "time_management",
			Title:       "学習時間を増やしましょう",
			Description: "平均学習時間が短いようです。より長い集中時間で効果を高めましょう。",
			Priority:    "medium",
			Actions: []string{
				"1回の学習セッションを20分以上にする",
				"休憩を挟みながら集中時間を伸ばす",
				"タイマーを使って時間を意識する",
			},
			ExpectedEffect: "集中力と理解度の向上が期待できます",
		}
		recommendations = append(recommendations, rec)
	}

	// 学習継続に基づく推奨
	if analysis.StudyStreak != nil && analysis.StudyStreak.CurrentStreak < 3 {
		rec := Recommendation{
			Type:        "consistency",
			Title:       "学習習慣を作りましょう",
			Description: "継続的な学習が重要です。毎日少しずつでも続けることが大切です。",
			Priority:    "high",
			Actions: []string{
				"毎日決まった時間に学習する",
				"小さな目標から始める",
				"学習カレンダーで進捗を可視化する",
			},
			ExpectedEffect: "学習効果と記憶の定着が向上します",
		}
		recommendations = append(recommendations, rec)
	}

	return recommendations
}

// calculateStudyStreak 学習継続情報を計算
func (m *Manager) calculateStudyStreak(userID string) (*StudyStreakInfo, error) {
	sessions, err := m.db.GetRecentStudySessions(userID, 30)
	if err != nil {
		return nil, err
	}

	if len(sessions) == 0 {
		return &StudyStreakInfo{}, nil
	}

	// 日付別にセッションをグループ化
	studyDates := make(map[string]bool)
	for _, session := range sessions {
		dateKey := session.StartTime.Format("2006-01-02")
		studyDates[dateKey] = true
	}

	// 現在の継続日数を計算
	currentStreak := 0
	longestStreak := 0
	tempStreak := 0

	// 日付をソート
	var dates []string
	for date := range studyDates {
		dates = append(dates, date)
	}
	sort.Strings(dates)

	// 継続日数の計算
	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	if studyDates[today] || studyDates[yesterday] {
		// 今日または昨日学習している場合、継続中
		for i := len(dates) - 1; i >= 0; i-- {
			date, _ := time.Parse("2006-01-02", dates[i])
			expectedDate := time.Now().AddDate(0, 0, -currentStreak)
			if date.Format("2006-01-02") == expectedDate.Format("2006-01-02") {
				currentStreak++
			} else {
				break
			}
		}
	}

	// 最長継続日数の計算
	for i := 0; i < len(dates); i++ {
		tempStreak = 1
		if i > 0 {
			prevDate, _ := time.Parse("2006-01-02", dates[i-1])
			currentDate, _ := time.Parse("2006-01-02", dates[i])
			if currentDate.Sub(prevDate).Hours() == 24 {
				tempStreak++
			} else {
				tempStreak = 1
			}
		}
		if tempStreak > longestStreak {
			longestStreak = tempStreak
		}
	}

	// 週・月の学習日数
	weekStart := time.Now().AddDate(0, 0, -int(time.Now().Weekday()))
	monthStart := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.Now().Location())

	studyDaysThisWeek := 0
	studyDaysThisMonth := 0

	for dateStr := range studyDates {
		date, _ := time.Parse("2006-01-02", dateStr)
		if date.After(weekStart) {
			studyDaysThisWeek++
		}
		if date.After(monthStart) {
			studyDaysThisMonth++
		}
	}

	streakInfo := &StudyStreakInfo{
		CurrentStreak:      currentStreak,
		LongestStreak:      longestStreak,
		StudyDaysThisWeek:  studyDaysThisWeek,
		StudyDaysThisMonth: studyDaysThisMonth,
	}

	if len(sessions) > 0 {
		streakInfo.LastStudyDate = sessions[0].StartTime
		if currentStreak > 0 {
			streakInfo.StreakStartDate = time.Now().AddDate(0, 0, -currentStreak+1)
		}
	}

	return streakInfo, nil
}

// GenerateSessionSummary セッション要約を生成
func (m *Manager) GenerateSessionSummary(sessionID string) (*SessionSummary, error) {
	// セッション情報を取得
	// 実際の実装では、セッションIDを使用してデータベースから詳細情報を取得
	
	// プレースホルダーの実装
	summary := &SessionSummary{
		SessionID: sessionID,
		// 他のフィールドは実際のデータベースクエリで取得
	}

	return summary, nil
}

// GetProgressTrend 進捗トレンドを取得
func (m *Manager) GetProgressTrend(userID string, subject string, days int) ([]float64, error) {
	sessions, err := m.db.GetRecentStudySessions(userID, days)
	if err != nil {
		return nil, err
	}

	// 日付別精度の計算
	dailyAccuracy := make(map[string][]float64)
	
	for _, session := range sessions {
		if subject == "" || session.Subject == subject {
			if session.TotalProblems > 0 {
				accuracy := float64(session.CorrectAnswers) / float64(session.TotalProblems)
				dateKey := session.StartTime.Format("2006-01-02")
				dailyAccuracy[dateKey] = append(dailyAccuracy[dateKey], accuracy)
			}
		}
	}

	// 日付順にソートして平均を計算
	var trend []float64
	var dates []string
	for date := range dailyAccuracy {
		dates = append(dates, date)
	}
	sort.Strings(dates)

	for _, date := range dates {
		accuracies := dailyAccuracy[date]
		sum := 0.0
		for _, acc := range accuracies {
			sum += acc
		}
		avgAccuracy := sum / float64(len(accuracies))
		trend = append(trend, avgAccuracy)
	}

	return trend, nil
}

// Close プログレス管理システムをクリーンアップ
func (m *Manager) Close() error {
	// 特にクリーンアップすることはない
	return nil
}
