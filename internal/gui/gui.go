package gui

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/google/uuid"

	"studybuddy-ai/internal/ai"
	"studybuddy-ai/internal/config"
	"studybuddy-ai/internal/database"
)

// MainApp メインアプリケーション
type MainApp struct {
	app      fyne.App
	window   fyne.Window
	db       *database.DB
	aiEngine *ai.Engine
	config   *config.Config

	// UI コンポーネント
	content      *container.AppTabs
	dashboard    *DashboardView
	studyView    *StudyView
	progressView *ProgressView
	settingsView *SettingsView

	// タブアイテム参照
	studyTab    *container.TabItem
	progressTab *container.TabItem

	// アプリケーション状態
	currentUser *database.User
}

// DashboardView ダッシュボード画面
type DashboardView struct {
	container   *fyne.Container
	welcomeCard *widget.Card
	statsCard   *widget.Card
	petCard     *widget.Card
	quickAction *fyne.Container
}

// StudyView 学習画面
type StudyView struct {
	container        *fyne.Container
	subjectSelect    *widget.Select
	problemCard      *widget.Card
	problemText      *widget.RichText // 問題文表示用（アクセシブル・高コントラスト）
	optionsContainer *fyne.Container
	feedbackCard     *widget.Card
	feedbackText     *widget.RichText // フィードバック表示用（アクセシブル・高コントラスト）

	// 学習状態
	currentSession *database.StudySession
	currentProblem *ai.Problem
	startTime      time.Time
	timerLabel     *widget.Label
	progressBar    *widget.ProgressBar
	isGenerating   bool // 問題生成中フラグ
}

// ProgressView 進捗画面
type ProgressView struct {
	container       *fyne.Container
	overallProgress *widget.Card
	subjectProgress *fyne.Container
	recentSessions  *widget.List
}

// SettingsView 設定画面
type SettingsView struct {
	container     *fyne.Container
	aiSettings    *widget.Card
	uiSettings    *widget.Card
	learnSettings *widget.Card
}

// NewMainApp メインアプリケーションを作成
func NewMainApp(app fyne.App, db *database.DB, aiEngine *ai.Engine, cfg *config.Config) *MainApp {
	w := app.NewWindow("StudyBuddy AI - パーソナル学習コンパニオン")
	w.Resize(fyne.NewSize(float32(cfg.UI.WindowWidth), float32(cfg.UI.WindowHeight)))
	w.CenterOnScreen()

	mainApp := &MainApp{
		app:      app,
		window:   w,
		db:       db,
		aiEngine: aiEngine,
		config:   cfg,
	}

	// ウィンドウクローズイベントハンドラー設定
	w.SetCloseIntercept(func() {
		log.Println("🪟 メインウィンドウ終了要求")

		// リソースクリーンアップ実行
		if err := mainApp.Close(); err != nil {
			log.Printf("GUI終了エラー: %v", err)
		}

		// アプリケーション全体の適切な終了処理
		mainApp.app.Quit()

		// プロセス確実終了（最後の手段）
		go func() {
			time.Sleep(3 * time.Second)
			log.Println("⚠️ 強制終了実行")
			os.Exit(0)
		}()
	})

	// ユーザー初期化
	mainApp.initializeUser()

	// UI初期化
	mainApp.createUI()

	return mainApp
}

// initializeUser ユーザーを初期化
func (m *MainApp) initializeUser() {
	userID := "default-user"
	user, err := m.db.GetUser(userID)

	if err != nil {
		// 新規ユーザー作成
		user = &database.User{
			ID:        userID,
			Name:      "学習者",
			Grade:     m.config.UserGrade,
			CreatedAt: time.Now(),
		}

		if err := m.db.CreateUser(user); err != nil {
			log.Printf("ユーザー作成エラー: %v", err)
		}

			// バーチャルペット機能を削除しました
	}

	m.currentUser = user

	// 最終ログイン更新
	if err := m.db.UpdateUserLastLogin(userID); err != nil {
		log.Printf("ログイン時刻更新エラー: %v", err)
	}
}

// createUI UIを作成
func (m *MainApp) createUI() {
	// 各画面を初期化
	m.dashboard = m.createDashboard()
	m.studyView = m.createStudyView()
	m.progressView = m.createProgressView()
	m.settingsView = m.createSettingsView()

	// タブ作成
	m.studyTab = container.NewTabItemWithIcon("学習", theme.DocumentIcon(), m.studyView.container)
	m.progressTab = container.NewTabItemWithIcon("進捗", theme.InfoIcon(), m.progressView.container)

	m.content = container.NewAppTabs(
		container.NewTabItemWithIcon("ホーム", theme.HomeIcon(), m.dashboard.container),
		m.studyTab,
		m.progressTab,
		container.NewTabItemWithIcon("設定", theme.SettingsIcon(), m.settingsView.container),
	)

	m.window.SetContent(m.content)
}

// createDashboard ダッシュボード画面を作成
func (m *MainApp) createDashboard() *DashboardView {
	dashboard := &DashboardView{}

	// ウェルカムカード
	dashboard.welcomeCard = widget.NewCard(
		fmt.Sprintf("こんにちは、%sさん！", m.currentUser.Name),
		"今日も一緒に学習しましょう",
		widget.NewLabel("StudyBuddy AIがあなたの学習をサポートします。\n好きな科目から始めてみませんか？"),
	)

	// 統計カード
	dashboard.statsCard = m.createStatsCard()

	// ペットカード（機能削除）
	dashboard.petCard = widget.NewCard("学習のこつ", "", 
		widget.NewLabel("毎日少しずつでも続けることが\n大切です。頑張りましょう！"))

	// クイックアクション
	dashboard.quickAction = container.NewGridWithColumns(2,
		widget.NewButton("学習開始", func() {
			m.content.Select(m.studyTab) // 学習タブに移動
		}),
		widget.NewButton("今日の進捗", func() {
			m.content.Select(m.progressTab) // 進捗タブに移動
		}),
	)

	// レイアウト
	dashboard.container = container.NewVBox(
		dashboard.welcomeCard,
		container.NewGridWithColumns(2,
			dashboard.statsCard,
			dashboard.petCard,
		),
		dashboard.quickAction,
	)

	return dashboard
}

// createStatsCard 統計カードを作成
func (m *MainApp) createStatsCard() *widget.Card {
	// 最近のセッション取得
	sessions, err := m.db.GetRecentStudySessions(m.currentUser.ID, 7)
	if err != nil {
		log.Printf("セッション取得エラー: %v", err)
		return widget.NewCard("今週の学習", "", widget.NewLabel("データを読み込み中..."))
	}

	if len(sessions) == 0 {
		return widget.NewCard("今週の学習", "",
			widget.NewLabel("まだ学習記録がありません。\n学習を始めてみましょう！"))
	}

	// 統計計算
	totalProblems := 0
	totalCorrect := 0
	for _, session := range sessions {
		totalProblems += session.TotalProblems
		totalCorrect += session.CorrectAnswers
	}

	accuracyRate := 0.0
	if totalProblems > 0 {
		accuracyRate = float64(totalCorrect) / float64(totalProblems) * 100
	}

	statsText := fmt.Sprintf(
		"学習セッション: %d回\n解答した問題: %d問\n正解率: %.1f%%",
		len(sessions), totalProblems, accuracyRate,
	)

	return widget.NewCard("今週の学習", "", widget.NewLabel(statsText))
}

// createPetCard機能を削除しました

// createStudyView 学習画面を作成
func (m *MainApp) createStudyView() *StudyView {
	study := &StudyView{}

	// 科目選択
	study.subjectSelect = widget.NewSelect(
		[]string{"数学", "英語", "国語", "理科", "社会"},
		func(subject string) {
			// 問題生成中は選択を無視
			if study.isGenerating {
				return
			}
			study.startStudySession(subject, m)
		},
	)
	study.subjectSelect.PlaceHolder = "学習する科目を選択してください"

	// 問題表示（アクセシブル・高コントラスト・ユニバーサルデザイン対応）
	study.problemText = widget.NewRichTextFromMarkdown("**AI接続中です。しばらくお待ちください...**\n\nOllamaモデルの読み込みには最大3分かかる場合があります。")
	study.problemText.Wrapping = fyne.TextWrapWord
	// 高コントラスト・読みやすさ重視の設定
	study.problemText.Resize(fyne.NewSize(500, 250))
	study.problemCard = widget.NewCard("📖 問題", "", study.problemText)

	// 選択肢コンテナ
	study.optionsContainer = container.NewVBox()

	// フィードバック（アクセシブル・高コントラスト表示）
	study.feedbackText = widget.NewRichTextFromMarkdown("解答後にフィードバックが表示されます")
	study.feedbackText.Wrapping = fyne.TextWrapWord
	// 横幅制限（画面見切れ対策）
	study.feedbackText.Resize(fyne.NewSize(350, 180))
	study.feedbackCard = widget.NewCard("💭 フィードバック", "", study.feedbackText)

	// ステータス表示（感情分析機能削除）
	study.timerLabel = widget.NewLabel("00:00")
	study.progressBar = widget.NewProgressBar()

	statusContainer := container.NewHBox(
		study.timerLabel,
		study.progressBar,
	)

	// 左側: 問題と選択肢
	leftPanel := container.NewVBox(
		study.problemCard,
		study.optionsContainer,
	)

	// 右側: フィードバック
	rightPanel := container.NewVBox(
		study.feedbackCard,
	)

	// 垂直レイアウトで重複完全回避
	mainContent := container.NewVBox(
		leftPanel,
		widget.NewSeparator(), // 視覚的区切り
		rightPanel,
	)

	// 全体レイアウト
	study.container = container.NewVBox(
		widget.NewCard("科目選択", "", study.subjectSelect),
		statusContainer,
		mainContent,
	)

	return study
}

// startStudySession 学習セッションを開始
func (s *StudyView) startStudySession(subject string, mainApp *MainApp) {
	// 新しいセッション作成
	session := &database.StudySession{
		ID:        uuid.New().String(),
		UserID:    mainApp.currentUser.ID,
		Subject:   subject,
		StartTime: time.Now(),
		CreatedAt: time.Now(),
	}

	if err := mainApp.db.CreateStudySession(session); err != nil {
		log.Printf("セッション作成エラー: %v", err)
		return
	}

	s.currentSession = session
	s.startTime = time.Now()

	// 学習進捗取得
	progress, err := mainApp.db.GetLearningProgress(mainApp.currentUser.ID, subject)
	if err != nil {
		log.Printf("進捗取得エラー: %v", err)
		progress = &database.LearningProgress{
			UserID:  mainApp.currentUser.ID,
			Subject: subject,
		}
	}

	// AI用の学習コンテキスト構築
	studyContext := ai.StudyContext{
		UserID:     mainApp.currentUser.ID,
		Subject:    subject,
		Grade:      mainApp.currentUser.Grade,
		Difficulty: mainApp.config.Learning.DifficultyLevel,
		Emotion:    "neutral",
		Progress:   calculateProgress(progress),
		Strengths:  []string{}, // TODO: 実際の強み分析
		Weaknesses: []string{}, // TODO: 実際の弱み分析
	}

	// 初期状態をAI準備完了状態に更新
	s.problemCard.SetTitle("📚 準備完了")
	s.problemText.ParseMarkdown("**科目を選択すると問題が表示されます**")
	s.problemText.Refresh()
	s.problemCard.Refresh()

	// 問題生成
	s.generateNewProblem(studyContext, mainApp)
}

// generateNewProblem 新しい問題を生成
func (s *StudyView) generateNewProblem(studyContext ai.StudyContext, mainApp *MainApp) {
	// 生成中フラグを設定（教科選択をブロック）
	s.isGenerating = true
	s.subjectSelect.Disable()
	
	// UI最初化（選択肢クリア）
	s.optionsContainer.RemoveAll()
	s.optionsContainer.Refresh()
	
	// フィードバッククリア
	s.feedbackCard.SetTitle("💭 フィードバック")
	s.feedbackText.ParseMarkdown("問題を生成中...")
	s.feedbackText.Refresh()
	s.feedbackCard.Refresh()
	
	// 問題生成状態表示
	s.problemCard.SetTitle("🔄 問題生成中")
	s.problemText.ParseMarkdown("**AI が問題を作成しています...**\n\n教科選択は生成完了までお待ちください。")

	go func() {
		// タイムアウトを8秒に大幅短縮（応答速度大幅改善）
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		problem, err := mainApp.aiEngine.GeneratePersonalizedProblem(ctx, studyContext)
		if err != nil {
			log.Printf("問題生成エラー: %v", err)
			// エラー時の確実な表示更新（メインスレッドで実行）
			fyne.Do(func() {
				// エラー時も教科選択を再有効化
				s.isGenerating = false
				s.subjectSelect.Enable()
				s.problemCard.SetTitle("⚠️ エラー")
				s.problemText.ParseMarkdown("**問題の生成に失敗しました。もう一度試してください。**")
				s.problemText.Refresh()
				s.problemCard.Refresh()
				s.container.Refresh() // コンテナ全体も更新
			})
			return
		}

		// UIを更新（メインスレッドで実行）
		fyne.Do(func() {
			// 生成完了、教科選択を再有効化
			s.isGenerating = false
			s.subjectSelect.Enable()
			s.displayProblem(problem, mainApp)
		})
	}()
}

// displayProblem 問題を表示
func (s *StudyView) displayProblem(problem *ai.Problem, mainApp *MainApp) {
	s.currentProblem = problem

	// 問題表示の確実な更新（数学記号対応・高コントラスト）
	s.problemCard.SetTitle(fmt.Sprintf("📚 %s", problem.Title))
	// 問題文をマークダウンで太字表示（アクセシブル）
	s.problemText.ParseMarkdown(fmt.Sprintf("## %s\n\n**%s**", problem.Title, problem.Description))
	// 複数回のRefreshで確実な更新
	s.problemText.Refresh()
	s.problemCard.Refresh()
	s.container.Refresh()
	// ログで確認
	descPreview := problem.Description
	if len(problem.Description) > 50 {
		descPreview = problem.Description[:50] + "..."
	}
	log.Printf("問題表示更新: タイトル=%s, 内容=%s", problem.Title, descPreview)

	// 選択肢ボタン（アクセシブル・色弱対応・ユニバーサルデザイン）
	s.optionsContainer.RemoveAll()
	for i, option := range problem.Options {
		optionIndex := i // クロージャ用にコピー
		// 色に依存しないボタンデザイン（アクセシブル）
		btn := widget.NewButton(fmt.Sprintf("%d. %s", i+1, option), func() {
			s.handleAnswer(optionIndex, mainApp)
		})
		// 色強調を使わず、テキストで区別（WCAG準拠）
		btn.Importance = widget.LowImportance // デフォルトのコントラストで読みやすく
		s.optionsContainer.Add(btn)
	}

	// フィードバックの確実なクリア
	s.feedbackCard.SetTitle("💭 フィードバック")
	s.feedbackText.ParseMarkdown("回答を選択してください")
	s.feedbackText.Refresh()
	s.feedbackCard.Refresh()

	s.optionsContainer.Refresh()
	log.Printf("問題表示完了: タイトル=%s, 説明文字数=%d", problem.Title, len(problem.Description))
}

// handleAnswer 回答処理
func (s *StudyView) handleAnswer(selectedIndex int, mainApp *MainApp) {
	if s.currentProblem == nil {
		return
	}

	endTime := time.Now()
	timeTaken := int(endTime.Sub(s.startTime).Seconds())
	isCorrect := selectedIndex == s.currentProblem.CorrectAnswer

	// 問題結果を保存
	result := &database.ProblemResult{
		ID:              uuid.New().String(),
		SessionID:       s.currentSession.ID,
		ProblemType:     s.currentProblem.ProblemType,
		Difficulty:      s.currentProblem.Difficulty,
		IsCorrect:       isCorrect,
		TimeTaken:       timeTaken,
		EmotionAtAnswer: "neutral", // 感情分析機能を削除
		ProblemContent:  s.currentProblem.Description,
		UserAnswer:      s.currentProblem.Options[selectedIndex],
		CorrectAnswer:   s.currentProblem.Options[s.currentProblem.CorrectAnswer],
		CreatedAt:       time.Now(),
	}

	if err := mainApp.db.CreateProblemResult(result); err != nil {
		log.Printf("結果保存エラー: %v", err)
	}

	// セッション統計更新
	s.currentSession.TotalProblems++
	if isCorrect {
		s.currentSession.CorrectAnswers++
	}

	if err := mainApp.db.UpdateStudySession(s.currentSession); err != nil {
		log.Printf("セッション更新エラー: %v", err)
	}

	// フィードバック表示
	s.showFeedback(result, mainApp)
}

// showFeedback フィードバックを表示
func (s *StudyView) showFeedback(result *database.ProblemResult, mainApp *MainApp) {
	// AI フィードバック生成
	feedbackReq := ai.FeedbackRequest{
		Problem:    *s.currentProblem,
		UserAnswer: result.UserAnswer,
		IsCorrect:  result.IsCorrect,
		TimeTaken:  result.TimeTaken,
		Emotion:    result.EmotionAtAnswer,
		StudyContext: ai.StudyContext{
			UserID:  mainApp.currentUser.ID,
			Subject: s.currentSession.Subject,
			Grade:   mainApp.currentUser.Grade,
		},
	}

	go func() {
		// フィードバック生成のタイムアウトを5秒に大幅短縮
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		feedback, err := mainApp.aiEngine.GenerateFeedback(ctx, feedbackReq)
		if err != nil {
			log.Printf("フィードバック生成エラー: %v", err)
			fyne.Do(func() {
				s.showSimpleFeedback(result)
			})
			return
		}

		// UIを更新（メインスレッドで実行）
		fyne.Do(func() {
			// 次の問題ボタン追加
			nextBtn := widget.NewButton("次の問題", func() {
				s.generateNewProblem(ai.StudyContext{
					UserID:     mainApp.currentUser.ID,
					Subject:    s.currentSession.Subject,
					Grade:      mainApp.currentUser.Grade,
					Difficulty: mainApp.config.Learning.DifficultyLevel,
					Emotion:    "neutral",
				}, mainApp)
			})
			nextBtn.Importance = widget.HighImportance

			// フィードバック表示（幅制限付き）
			s.feedbackCard.SetTitle("フィードバック")
			feedbackContent := container.NewVBox(
				widget.NewRichTextFromMarkdown(fmt.Sprintf("**結果:** %s\n\n**説明:** %s", feedback.Message, feedback.Explanation)),
				nextBtn,
			)
			s.feedbackCard.SetContent(feedbackContent)
		})
	}()
}

// showSimpleFeedback シンプルなフィードバックを表示
func (s *StudyView) showSimpleFeedback(result *database.ProblemResult) {
	message := "❌ 不正解です。"
	if result.IsCorrect {
		message = "✅ 正解です！"
	}

	// 次の問題ボタン追加
	nextBtn := widget.NewButton("次の問題", func() {
		// 次の問題を生成（簡易版）
		log.Println("次の問題を生成中...")
	})
	nextBtn.Importance = widget.HighImportance

	s.feedbackCard.SetTitle("フィードバック")
	feedbackContent := container.NewVBox(
		widget.NewLabel(message),
		widget.NewLabel(fmt.Sprintf("正解: %s", result.CorrectAnswer)),
		nextBtn,
	)
	s.feedbackCard.SetContent(feedbackContent)
}

// createProgressView 進捗画面を作成
func (m *MainApp) createProgressView() *ProgressView {
	progress := &ProgressView{}

	// 全体進捗カード - 実際のデータを表示
	overallStats := m.calculateOverallProgress()
	progress.overallProgress = widget.NewCard("全体の進捗", "", 
		widget.NewLabel(overallStats))

	// 科目別進捗 - 実際のデータを表示
	progress.subjectProgress = m.createSubjectProgress()

	// 最近のセッション
	sessions, _ := m.db.GetRecentStudySessions(m.currentUser.ID, 10)
	sessionNames := make([]string, len(sessions))
	for i, session := range sessions {
		sessionNames[i] = fmt.Sprintf("%s - %s",
			session.Subject, session.StartTime.Format("01/02 15:04"))
	}

	progress.recentSessions = widget.NewList(
		func() int { return len(sessionNames) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			obj.(*widget.Label).SetText(sessionNames[id])
		},
	)

	progress.container = container.NewVBox(
		progress.overallProgress,
		widget.NewCard("最近の学習セッション", "", progress.recentSessions),
	)

	return progress
}

// createSettingsView 設定画面を作成
func (m *MainApp) createSettingsView() *SettingsView {
	settings := &SettingsView{}

	// AI設定
	aiModelSelect := widget.NewSelect(
		[]string{"dsasai/llama3-elyza-jp-8b", "7shi/ezo-gemma-2-jpn:2b-instruct-q8_0 ", "hf.co/mmnga/cyberagent-DeepSeek-R1-Distill-Qwen-14B-Japanese-gguf"},
		func(model string) {
			m.config.AI.Model = model
			_ = config.Save(m.config)
		},
	)
	aiModelSelect.SetSelected(m.config.AI.Model)

	settings.aiSettings = widget.NewCard("AI設定", "",
		container.NewVBox(
			widget.NewLabel("使用するAIモデル:"),
			aiModelSelect,
		),
	)

	// UI設定（ダークモード削除）
	settings.uiSettings = widget.NewCard("表示設定", "",
		widget.NewLabel("現在利用可能な表示設定はありません。"))

	// 学習設定
	difficultySlider := widget.NewSlider(1, 5)
	difficultySlider.SetValue(float64(m.config.Learning.DifficultyLevel))
	difficultySlider.OnChanged = func(value float64) {
		m.config.Learning.DifficultyLevel = int(value)
		_ = config.Save(m.config)
	}

	settings.learnSettings = widget.NewCard("学習設定", "",
		container.NewVBox(
			widget.NewLabel("難易度レベル:"),
			difficultySlider,
		),
	)

	settings.container = container.NewVBox(
		settings.aiSettings,
		settings.uiSettings,
		settings.learnSettings,
	)

	return settings
}

// Show アプリケーションを表示
func (m *MainApp) Show() {
	m.window.ShowAndRun()
}

// Close GUIシステムを適切にクローズ
func (m *MainApp) Close() error {
	log.Println("🪟 GUIリソースのクリーンアップ開始")

	// 進行中の学習セッションを終了
	if m.studyView != nil && m.studyView.currentSession != nil {
		endTime := time.Now()
		m.studyView.currentSession.EndTime = &endTime
		if err := m.db.UpdateStudySession(m.studyView.currentSession); err != nil {
			log.Printf("セッション終了処理エラー: %v", err)
		}
	}

	// 設定保存
	if err := config.Save(m.config); err != nil {
		log.Printf("設定保存エラー: %v", err)
	}

	// ウィンドウを隠す
	if m.window != nil {
		m.window.Hide()
	}

	log.Println("✅ GUIリソースのクリーンアップ完了")
	return nil
}

// calculateProgress 進捗率を計算
func calculateProgress(progress *database.LearningProgress) float64 {
	if progress.TotalProblems == 0 {
		return 0.0
	}
	return float64(progress.CorrectAnswers) / float64(progress.TotalProblems)
}

// calculateOverallProgress 全体進捗を計算
func (m *MainApp) calculateOverallProgress() string {
	// 全科目のセッションを取得
	sessions, err := m.db.GetRecentStudySessions(m.currentUser.ID, 30) // 過去30回
	if err != nil {
		return "データ読み込みエラー"
	}

	if len(sessions) == 0 {
		return "まだ学習記録がありません。\n学習を始めてみましょう！"
	}

	// 科目別統計を集計
	subjectStats := make(map[string]struct {
		totalProblems int
		correctAnswers int
		sessions int
	})

	totalAllProblems := 0
	totalAllCorrect := 0
	totalSessions := len(sessions)

	for _, session := range sessions {
		stats := subjectStats[session.Subject]
		stats.totalProblems += session.TotalProblems
		stats.correctAnswers += session.CorrectAnswers
		stats.sessions++
		subjectStats[session.Subject] = stats

		totalAllProblems += session.TotalProblems
		totalAllCorrect += session.CorrectAnswers
	}

	overallAccuracy := 0.0
	if totalAllProblems > 0 {
		overallAccuracy = float64(totalAllCorrect) / float64(totalAllProblems) * 100
	}

	return fmt.Sprintf(
		"学習セッション: %d回\n解答した問題: %d問\n全体正解率: %.1f%%\n学習科目数: %d科目",
		totalSessions, totalAllProblems, overallAccuracy, len(subjectStats),
	)
}

// createSubjectProgress 科目別進捗を作成
func (m *MainApp) createSubjectProgress() *fyne.Container {
	// 全科目のセッションを取得
	sessions, err := m.db.GetRecentStudySessions(m.currentUser.ID, 50)
	if err != nil {
		return container.NewVBox(widget.NewLabel("データ読み込みエラー"))
	}

	if len(sessions) == 0 {
		return container.NewVBox(widget.NewLabel("まだ学習記録がありません。"))
	}

	// 科目別統計を集計
	subjectStats := make(map[string]struct {
		totalProblems int
		correctAnswers int
		sessions int
		lastStudied time.Time
	})

	for _, session := range sessions {
		stats := subjectStats[session.Subject]
		stats.totalProblems += session.TotalProblems
		stats.correctAnswers += session.CorrectAnswers
		stats.sessions++
		if session.StartTime.After(stats.lastStudied) {
			stats.lastStudied = session.StartTime
		}
		subjectStats[session.Subject] = stats
	}

	// 科目別カードを作成
	subjectCards := container.NewVBox()
	for subject, stats := range subjectStats {
		accuracy := 0.0
		if stats.totalProblems > 0 {
			accuracy = float64(stats.correctAnswers) / float64(stats.totalProblems) * 100
		}

		subjectInfo := fmt.Sprintf(
			"セッション: %d回\n問題数: %d問\n正解率: %.1f%%\n最終学習: %s",
			stats.sessions, stats.totalProblems, accuracy,
			stats.lastStudied.Format("01/02 15:04"),
		)

		card := widget.NewCard(subject, "", widget.NewLabel(subjectInfo))
		subjectCards.Add(card)
	}

	return subjectCards
}

// ShowErrorDialog エラーダイアログを表示
func (m *MainApp) ShowErrorDialog(title, message string) {
	dialog.ShowError(
		fmt.Errorf("%s", message),
		m.window,
	)
}

// ShowInfoDialog 情報ダイアログを表示
func (m *MainApp) ShowInfoDialog(title, message string) {
	dialog.ShowInformation(title, message, m.window)
}
