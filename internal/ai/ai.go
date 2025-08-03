package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"studybuddy-ai/internal/config"
)

// Engine AI推論エンジン
type Engine struct {
	config       config.AIConfig
	httpClient   *http.Client
	isOnline     bool
	lastCheck    time.Time
	failureCount int
	mu           sync.RWMutex
	problemIndex map[string]int // 教科別の問題インデックス
}

// Problem 問題構造体
type Problem struct {
	Title         string
	Description   string
	Options       []string
	CorrectAnswer int
	Explanation   string
	Difficulty    int
	EstimatedTime int // 秒
	Encouragement string
	ProblemType   string
}

// StudyContext 学習コンテキスト
type StudyContext struct {
	UserID         string
	Subject        string
	Grade          int
	Difficulty     int
	Emotion        string
	Progress       float64
	Strengths      []string
	Weaknesses     []string
	PreviousErrors []ErrorPattern
	SessionHistory []SessionInfo
}

// ErrorPattern エラーパターン
type ErrorPattern struct {
	ProblemType  string
	ErrorType    string
	Frequency    int
	LastOccurred time.Time
}

// SessionInfo セッション情報
type SessionInfo struct {
	Subject       string
	AccuracyRate  float64
	AverageTime   float64
	Emotion       string
	ProblemsCount int
	StudyTime     int
}

// FeedbackRequest フィードバック要求
type FeedbackRequest struct {
	Problem      Problem
	UserAnswer   string
	IsCorrect    bool
	TimeTaken    int
	Emotion      string
	StudyContext StudyContext
}

// FeedbackResponse フィードバック応答
type FeedbackResponse struct {
	Message       string
	Explanation   string
	Encouragement string
	NextSteps     string
	TipOfDay      string
}

// OllamaRequest Ollama API リクエスト
type OllamaRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	Stream  bool                   `json:"stream"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// OllamaResponse Ollama API レスポンス
type OllamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
	Error    string `json:"error,omitempty"`
}

// NewEngine AI エンジンを作成
func NewEngine(config config.AIConfig) (*Engine, error) {
	engine := &Engine{
		config: config,
		httpClient: &http.Client{
			Timeout: 300 * time.Second, // Ollamaモデルロード用5分タイムアウト
		},
		isOnline:     true, // 初期状態でAIを試行
		lastCheck:    time.Time{},
		failureCount: 0, // 失敗カウント初期化
		problemIndex: make(map[string]int),
	}

	// 初期状態をオンラインに設定（実際の接続は初回利用時にテスト）
	engine.setOnline()

	return engine, nil
}

// setOnline AIオンライン状態を設定
func (e *Engine) setOnline() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.isOnline = true
	e.failureCount = 0
	e.lastCheck = time.Now()
}

// shouldTryAI AI接続を試行すべきか判定（常に試行）
func (e *Engine) shouldTryAI() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// 常にAI接続を試行（学習アプリとしてAI生成が最優先）
	return true
}

// recordFailure AI失敗を記録
func (e *Engine) recordFailure() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.isOnline = false
	e.failureCount++
	e.lastCheck = time.Now()
}

// recordSuccess AI成功を記録
func (e *Engine) recordSuccess() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.isOnline = true
	e.failureCount = 0
	e.lastCheck = time.Now()
}

// testConnection Ollamaサーバーとの接続をテスト
func (e *Engine) testConnection(ctx context.Context) error {
	// シンプルなテストプロンプト
	testPrompt := "こんにちは"

	response, err := e.generate(ctx, testPrompt)
	if err != nil {
		return fmt.Errorf("接続テストエラー: %w", err)
	}

	// 日本語応答の確認
	if !containsJapanese(response) {
		return fmt.Errorf("日本語応答が確認できません。モデル設定を確認してください")
	}

	return nil
}

// GeneratePersonalizedProblem 個人に最適化された問題を生成（オフライン対応）
func (e *Engine) GeneratePersonalizedProblem(ctx context.Context, studyContext StudyContext) (*Problem, error) {
	// オンライン状態チェック
	if !e.shouldTryAI() {
		return e.generateOfflineProblem(studyContext), nil
	}

	prompt := e.buildPersonalizedPrompt(studyContext)
	response, err := e.generate(ctx, prompt)
	if err != nil {
		e.recordFailure()
		return e.generateOfflineProblem(studyContext), nil
	}

	e.recordSuccess()
	return e.parseProblemResponse(response)
}

// GenerateFeedback フィードバックを生成（オフライン対応）
func (e *Engine) GenerateFeedback(ctx context.Context, req FeedbackRequest) (*FeedbackResponse, error) {
	// オンライン状態チェック
	if !e.shouldTryAI() {
		return e.generateOfflineFeedback(req), nil
	}

	prompt := e.buildFeedbackPrompt(req)
	response, err := e.generate(ctx, prompt)
	if err != nil {
		e.recordFailure()
		return e.generateOfflineFeedback(req), nil
	}

	e.recordSuccess()
	return e.parseFeedbackResponse(response)
}

// buildPersonalizedPrompt 学習指導要領準拠プロンプト（架空資料参照禁止）
func (e *Engine) buildPersonalizedPrompt(context StudyContext) string {
	// 学年別学習内容マップ（2024年度学習指導要領準拠）
	gradeContent := map[int]map[string]string{
		1: {
			"数学": "正の数・負の数、文字と式、一次方程式、比例と反比例、平面図形、空間図形、データの活用",
			"英語": "アルファベット、基本単語、be動詞、一般動詞、疑問文、否定文、現在進行形",
			"国語": "漢字の読み書き、詩歌の鑑賞、説明文の読解、古典の基礎、文法（品詞）",
			"理科": "植物の生活と種類、身のまわりの物質、光・音・力、大地の変化",
			"社会": "世界の地理、日本の地理、歴史（古代文明から平安時代）",
		},
		2: {
			"数学": "式の計算、連立方程式、一次関数、図形の性質と合同、確率、データの活用",
			"英語": "過去形、未来形、助動詞、比較級・最上級、不定詞、動名詞",
			"国語": "短歌・俳句、説明文・論説文、小説、古典（古文・漢文の基礎）、敬語",
			"理科": "動物の生活と生物の変遷、電流とその利用、化学変化と原子・分子、天気とその変化",
			"社会": "日本の歴史（鎌倉時代から江戸時代）、世界と日本の地理",
		},
		3: {
			"数学": "二次方程式、二次関数、相似、三平方の定理、円の性質、標本調査",
			"英語": "現在完了、受動態、関係代名詞、間接疑問文、分詞",
			"国語": "近現代文学、古典文学、文法の総復習、論説文・評論文の読解",
			"理科": "生命の連続性、運動とエネルギー、化学変化とイオン、地球と宇宙",
			"社会": "日本の歴史（明治維新から現代）、公民（政治・経済・国際社会）",
		},
	}

	gradeText := []string{"", "中1", "中2", "中3"}
	content := gradeContent[context.Grade][context.Subject]

	// 数学問題の場合の追加制約
	mathConstraints := ""
	if context.Subject == "数学" || context.Subject == "算数" {
		mathConstraints = `

【数学的正確性の絶対要求】
- 必ず問題作成前に全ての計算を実行し検証すること
- 角度問題：三角形の内角の和=180度、二等辺三角形で等しい角の計算を正確に行う
- 方程式問題：必ず代入して検算し正解を確認する
- 計算問題：全ての演算を段階的に実行し検証する
- 正解以外の選択肢も数学的に意味のある値にする
- 学習指導要領に完全準拠した内容のみ出題する`
	}

	return fmt.Sprintf(`%s%sの問題を1問作成。

【重要な制約】
- 学習範囲: %s
- 上記範囲のみから出題すること
- 架空の資料、文章、教科書は一切参照しないこと
- "次の文中から""下の図""以下の文""次の文字""次の単語""次の数式""次の図""次の表は""次の資料"といった、問題文には存在しない資料への言及は絶対禁止
- 問題文には必要なすべての情報（例文、数式、数値など）を直接含めること
- 問題文は必ず完全に自己完結させること%s

形式:
TITLE: タイトル
DESCRIPTION: 問題文
OPTION1: 選択肢1
OPTION2: 選択肢2
OPTION3: 選択肢3
OPTION4: 選択肢4
CORRECT: 1
EXPLANATION: 解説
DIFFICULTY: %d
TIME: 180
ENCOURAGEMENT: 応援メッセージ
TYPE: カテゴリ

上記形式のみで回答。`,
		gradeText[context.Grade], context.Subject, content, mathConstraints, context.Difficulty)
}

// buildFeedbackPrompt 数学的正確性重視フィードバックプロンプト
func (e *Engine) buildFeedbackPrompt(req FeedbackRequest) string {
	resultText := "不正解"
	if req.IsCorrect {
		resultText = "正解"
	}

	// 数学問題かどうかを判定
	isMathProblem := strings.Contains(req.Problem.Description, "角") ||
		strings.Contains(req.Problem.Description, "三角形") ||
		strings.Contains(req.Problem.Description, "度") ||
		strings.Contains(req.Problem.Description, "計算") ||
		strings.Contains(req.Problem.Description, "方程式") ||
		strings.Contains(req.Problem.Description, "面積") ||
		strings.Contains(req.Problem.Description, "体積") ||
		strings.Contains(req.Problem.Description, "√") ||
		strings.Contains(req.Problem.Description, "²") ||
		strings.Contains(req.Problem.Description, "平方") ||
		strings.Contains(req.Problem.Description, "=")

	basePrompt := fmt.Sprintf(`結果: %s
問題: %s
回答: %s
正解: %s`, resultText, req.Problem.Description, req.UserAnswer, req.Problem.Options[req.Problem.CorrectAnswer])

	if isMathProblem {
		return basePrompt + `

【重要】数学問題のため、必ず計算過程を含めること。

フィードバックを以下形式で:

MESSAGE: メッセージ
CALCULATION: 段階的計算過程（必須）
EXPLANATION: 数学的根拠と解説
ENCOURAGEMENT: 励まし
NEXT_STEPS: 次のステップ
TIP: 数学のコツ

例）二等辺三角形で角A=角C=60度の場合:
CALCULATION: 角A + 角B + 角C = 180度, 60度 + 角B + 60度 = 180度, 角B = 180度 - 120度 = 60度

上記形式のみで回答。`
	}

	return basePrompt + `

フィードバックを以下形式で:

MESSAGE: メッセージ
EXPLANATION: 解説
ENCOURAGEMENT: 励まし
NEXT_STEPS: 次のステップ
TIP: コツ

上記形式のみで回答。`
}

// generate Ollama APIを使用してテキスト生成
func (e *Engine) generate(ctx context.Context, prompt string) (string, error) {
	reqBody := OllamaRequest{
		Model:  e.config.Model,
		Prompt: prompt,
		Stream: true, // 500エラー解決: ストリーミングモード使用
		Options: map[string]interface{}{
			"temperature": 0.7,  // 日本語モデル最適値
			"top_p":       0.9,  // 多様性バランス
			"top_k":       40,   // 選択肢制限
			"num_predict": 512,  // 処理時間短縮用制限
			"num_ctx":     8192, // コンテキスト長
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("リクエスト作成エラー: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.config.OllamaURL+"/api/generate", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("HTTPリクエスト作成エラー: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTPリクエストエラー: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama APIエラー: %d - %s", resp.StatusCode, string(body))
	}

	// ストリーミングレスポンス処理（NDJSON形式）
	scanner := bufio.NewScanner(resp.Body)
	var fullResponse strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var ollamaResp OllamaResponse
		if err := json.Unmarshal([]byte(line), &ollamaResp); err != nil {
			continue // 不正なJSONはスキップ
		}

		if ollamaResp.Error != "" {
			return "", fmt.Errorf("ollama処理エラー: %s", ollamaResp.Error)
		}

		// レスポンステキストを蓄積
		fullResponse.WriteString(ollamaResp.Response)

		// 生成完了チェック
		if ollamaResp.Done {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("ストリーミング読み取りエラー: %w", err)
	}

	return strings.TrimSpace(fullResponse.String()), nil
}

// parseProblemResponse 問題生成レスポンスをパース
func (e *Engine) parseProblemResponse(response string) (*Problem, error) {
	// キー:値形式でパース
	fields := parseKeyValueResponse(response)
	if len(fields) == 0 {
		return nil, fmt.Errorf("レスポンス解析エラー: %s", response)
	}

	problem := &Problem{
		Title:       getField(fields, "TITLE", ""),
		Description: getField(fields, "DESCRIPTION", ""),
		Options: []string{
			getField(fields, "OPTION1", ""),
			getField(fields, "OPTION2", ""),
			getField(fields, "OPTION3", ""),
			getField(fields, "OPTION4", ""),
		},
		CorrectAnswer: parseInt(getField(fields, "CORRECT", "1")) - 1, // 1-indexedから0-indexedに変換
		Explanation:   getField(fields, "EXPLANATION", ""),
		Difficulty:    parseInt(getField(fields, "DIFFICULTY", "3")),
		EstimatedTime: parseInt(getField(fields, "TIME", "300")),
		Encouragement: getField(fields, "ENCOURAGEMENT", ""),
		ProblemType:   getField(fields, "TYPE", ""),
	}

	// 必須フィールドの検証
	if err := validateProblem(problem); err != nil {
		return nil, fmt.Errorf("問題検証エラー: %w", err)
	}

	return problem, nil
}

// parseFeedbackResponse フィードバックレスポンスをパース
func (e *Engine) parseFeedbackResponse(response string) (*FeedbackResponse, error) {
	// キー:値形式でパース
	fields := parseKeyValueResponse(response)
	if len(fields) == 0 {
		return nil, fmt.Errorf("レスポンス解析エラー: %s", response)
	}

	feedback := &FeedbackResponse{
		Message:       getField(fields, "MESSAGE", ""),
		Explanation:   getField(fields, "EXPLANATION", ""),
		Encouragement: getField(fields, "ENCOURAGEMENT", ""),
		NextSteps:     getField(fields, "NEXT_STEPS", ""),
		TipOfDay:      getField(fields, "TIP", ""),
	}

	return feedback, nil
}

// parseKeyValueResponse キー:値形式のレスポンスをパース
func parseKeyValueResponse(response string) map[string]string {
	// マークダウンのコードブロックを除去
	response = strings.ReplaceAll(response, "```", "")
	response = strings.TrimSpace(response)

	fields := make(map[string]string)
	lines := strings.Split(response, "\n")

	var currentKey string
	var currentValue strings.Builder

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// キー:値の行を検出
		if colonIndex := strings.Index(line, ":"); colonIndex != -1 {
			// 前のキーがあれば保存
			if currentKey != "" {
				fields[currentKey] = strings.TrimSpace(currentValue.String())
			}

			// 新しいキーと値を設定
			currentKey = strings.TrimSpace(line[:colonIndex])
			value := strings.TrimSpace(line[colonIndex+1:])
			currentValue.Reset()
			currentValue.WriteString(value)
		} else if currentKey != "" {
			// 継続行（複数行にわたる値）
			if currentValue.Len() > 0 {
				currentValue.WriteString("\n")
			}
			currentValue.WriteString(line)
		}
	}

	// 最後のキーを保存
	if currentKey != "" {
		fields[currentKey] = strings.TrimSpace(currentValue.String())
	}

	return fields
}

// getField フィールドから値を取得（デフォルト値付き）
func getField(fields map[string]string, key, defaultValue string) string {
	if value, exists := fields[key]; exists && value != "" {
		return value
	}
	return defaultValue
}

// parseInt 文字列を整数に変換（エラー時はデフォルト値）
func parseInt(value string) int {
	if result, err := strconv.Atoi(value); err == nil {
		return result
	}
	return 0 // デフォルト値
}

// validateProblem 問題の妥当性チェック（数学的正確性検証を含む）
func validateProblem(problem *Problem) error {
	if problem.Title == "" {
		return fmt.Errorf("タイトルが空です")
	}
	if problem.Description == "" {
		return fmt.Errorf("問題文が空です")
	}
	if len(problem.Options) < 2 {
		return fmt.Errorf("選択肢が不足しています（最低2つ必要）")
	}
	if problem.CorrectAnswer < 0 || problem.CorrectAnswer >= len(problem.Options) {
		return fmt.Errorf("正解インデックスが無効です")
	}
	if problem.Difficulty < 1 || problem.Difficulty > 5 {
		return fmt.Errorf("難易度が範囲外です（1-5）")
	}
	if problem.EstimatedTime <= 0 {
		problem.EstimatedTime = 300 // デフォルト5分
	}

	// 数学問題の場合の追加検証
	if isMathProblem := strings.Contains(problem.Description, "角") ||
		strings.Contains(problem.Description, "三角形") ||
		strings.Contains(problem.Description, "度") ||
		strings.Contains(problem.Description, "計算") ||
		strings.Contains(problem.Description, "方程式") ||
		strings.Contains(problem.Description, "√") ||
		strings.Contains(problem.Description, "²") ||
		strings.Contains(problem.Description, "="); isMathProblem {
		
		// 架空資料参照の禁止チェック
		forbiddenPhrases := []string{
			"次の文中から", "下の図", "以下の文", "次の文字は", "次の単語は", 
			"次の数式は", "次の図", "次の表は", "次の資料",
		}
		for _, phrase := range forbiddenPhrases {
			if strings.Contains(problem.Description, phrase) {
				return fmt.Errorf("数学問題で架空資料への参照が検出されました: %s", phrase)
			}
		}

		// 二等辺三角形の角度問題の検証例
		if strings.Contains(problem.Description, "二等辺三角形") && strings.Contains(problem.Description, "角") {
			if err := validateIsoscelesTriangleProblem(problem); err != nil {
				return fmt.Errorf("二等辺三角形問題の数学的エラー: %w", err)
			}
		}
	}

	return nil
}

// validateIsoscelesTriangleProblem 二等辺三角形問題の数学的正確性を検証
func validateIsoscelesTriangleProblem(problem *Problem) error {
	// 簡単な検証例：角A=角C=45度で角B=90度の場合
	if strings.Contains(problem.Description, "45度") && strings.Contains(problem.Description, "角B") {
		correctAnswer := problem.Options[problem.CorrectAnswer]
		if !strings.Contains(correctAnswer, "90") {
			return fmt.Errorf("二等辺三角形で角A=角C=45度の場合、角B=90度が正解ですが、設定された正解は %s です", correctAnswer)
		}
	}
	return nil
}

// containsJapanese 日本語文字が含まれているかチェック
func containsJapanese(text string) bool {
	for _, r := range text {
		// ひらがな、カタカナ、漢字の範囲をチェック
		if (r >= 0x3040 && r <= 0x309F) || // ひらがな
			(r >= 0x30A0 && r <= 0x30FF) || // カタカナ
			(r >= 0x4E00 && r <= 0x9FAF) { // 漢字
			return true
		}
	}
	return false
}

// GetAvailableModels 利用可能なモデル一覧を取得
func (e *Engine) GetAvailableModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", e.config.OllamaURL+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("リクエスト作成エラー: %w", err)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTPリクエストエラー: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("レスポンス読み取りエラー: %w", err)
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("レスポンス解析エラー: %w", err)
	}

	models := make([]string, len(result.Models))
	for i, model := range result.Models {
		models[i] = model.Name
	}

	return models, nil
}

// UpdateConfig AI設定を更新
func (e *Engine) UpdateConfig(newConfig config.AIConfig) error {
	e.config = newConfig

	// 新しい設定での接続テスト
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return e.testConnection(ctx)
}

// GetCurrentModel 現在使用中のモデルを取得
func (e *Engine) GetCurrentModel() string {
	return e.config.Model
}

// GenerateStudyTip 学習のコツを生成
func (e *Engine) GenerateStudyTip(ctx context.Context, subject string, weakness string) (string, error) {
	prompt := fmt.Sprintf(`
中学生向けの学習アドバイザーとして、以下の情報に基づいて具体的で実践的な学習のコツを1つ提供してください。

【対象】
- 教科: %s
- 苦手分野: %s

【指示】
- 中学生でも簡単に実践できる具体的なコツを提供
- なぜそのコツが効果的なのか簡潔に説明
- 前向きで励みになる表現を使用
- 100文字以内で簡潔に

一つの学習のコツのみを返してください。
`, subject, weakness)

	return e.generate(ctx, prompt)
}

// generateOfflineProblem オフライン時の代替問題を生成
func (e *Engine) generateOfflineProblem(context StudyContext) *Problem {
	// 教科と学年に基づいてサンプル問題を提供
	switch context.Subject {
	case "数学", "算数":
		return e.getMathProblem(context.Grade, context.Difficulty)
	case "英語":
		return e.getEnglishProblem(context.Grade, context.Difficulty)
	case "国語":
		return e.getJapaneseProblem(context.Grade, context.Difficulty)
	case "理科":
		return e.getScienceProblem(context.Grade, context.Difficulty)
	case "社会":
		return e.getSocialStudiesProblem(context.Grade, context.Difficulty)
	default:
		return e.getGeneralProblem(context.Grade, context.Difficulty)
	}
}

// generateOfflineFeedback オフライン時の代替フィードバックを生成
func (e *Engine) generateOfflineFeedback(req FeedbackRequest) *FeedbackResponse {
	if req.IsCorrect {
		return &FeedbackResponse{
			Message:       "🎉 正解です！よく頑張りました！",
			Explanation:   "素晴らしい理解力です。この調子で学習を続けていきましょう。",
			Encouragement: "あなたの努力が実っています。この調子で頑張りましょう！",
			NextSteps:     "次はもう少し難しい問題にチャレンジしてみましょう。",
			TipOfDay:      "理解したことを自分の言葉で説明してみると、さらに記憶に定着しやすくなります。",
		}
	} else {
		return &FeedbackResponse{
			Message:       "📚 おしい！間違いも学習の大切な一歩です。",
			Explanation:   "今回は間違えましたが、これも貴重な学習経験です。正解を確認して理解を深めましょう。",
			Encouragement: "失敗は成功の母です。諦めずに続けていけば必ず理解できます！",
			NextSteps:     "同じ問題を時間を置いてもう一度挑戦してみましょう。",
			TipOfDay:      "間違えた問題は記録しておき、後で復習すると理解が深まります。",
		}
	}
}

// getMathProblem 数学の問題を取得
func (e *Engine) getMathProblem(grade, _ int) *Problem {
	mathProblems := map[int][]*Problem{
		1: { // 中学1年
			{
				Title:         "正負の数の計算",
				Description:   "次の計算をしてください。\n(-3) + (+5) = ?",
				Options:       []string{"+2", "-2", "+8", "-8"},
				CorrectAnswer: 0,
				Explanation:   "(-3) + (+5) = 5 - 3 = +2 です。正の数から負の数を引くときは、符号に注意しましょう。",
				Difficulty:    2,
				EstimatedTime: 180,
				Encouragement: "正負の数の計算は慣れれば簡単です！",
				ProblemType:   "計算",
			},
			{
				Title:         "文字と式",
				Description:   "x = 3 のとき、2x + 1 の値を求めてください。",
				Options:       []string{"5", "6", "7", "8"},
				CorrectAnswer: 2,
				Explanation:   "x = 3 を代入すると、2×3 + 1 = 6 + 1 = 7 です。",
				Difficulty:    3,
				EstimatedTime: 200,
				Encouragement: "代入の計算は順序を守れば確実に解けます！",
				ProblemType:   "文字式",
			},
			{
				Title:         "平方根の計算",
				Description:   "✓ 9 の値はいくつでしょうか？",
				Options:       []string{"3", "4", "6", "9"},
				CorrectAnswer: 0,
				Explanation:   "✓ 9 = 3 です。なぜなら3 × 3 = 9 だからです。",
				Difficulty:    2,
				EstimatedTime: 160,
				Encouragement: "平方根は九九を覚えるとよいでしょう！",
				ProblemType:   "平方根",
			},
		},
		2: { // 中学2年
			{
				Title:         "連立方程式",
				Description:   "次の連立方程式を解いてください。\nx + y = 5\nx - y = 1\nxの値は？",
				Options:       []string{"1", "2", "3", "4"},
				CorrectAnswer: 2,
				Explanation:   "2つの式を足すと 2x = 6 なので x = 3 です。",
				Difficulty:    3,
				EstimatedTime: 300,
				Encouragement: "連立方程式は代入法や加減法をマスターすれば簡単です！",
				ProblemType:   "方程式",
			},
			{
				Title:         "一次関数",
				Description:   "y = 2x + 1 において、x = 2 のときの y の値は？",
				Options:       []string{"3", "4", "5", "6"},
				CorrectAnswer: 2,
				Explanation:   "y = 2 × 2 + 1 = 4 + 1 = 5 です。",
				Difficulty:    3,
				EstimatedTime: 200,
				Encouragement: "一次関数の代入は基本です！",
				ProblemType:   "関数",
			},
		},
		3: { // 中学3年
			{
				Title:         "二次方程式",
				Description:   "二次方程式 x² - 5x + 6 = 0 を解いてください。\n解のうち小さい方は？",
				Options:       []string{"1", "2", "3", "6"},
				CorrectAnswer: 1,
				Explanation:   "因数分解すると (x-2)(x-3) = 0 なので、x = 2, 3 です。小さい方は 2 です。",
				Difficulty:    4,
				EstimatedTime: 400,
				Encouragement: "二次方程式は因数分解の基本をマスターすれば解けます！",
				ProblemType:   "二次方程式",
			},
			{
				Title:         "平方根の应用",
				Description:   "√50 を簡単な形に表すと？",
				Options:       []string{"5√2", "2√5", "25", "50"},
				CorrectAnswer: 0,
				Explanation:   "√50 = √(25 × 2) = 5√2 です。",
				Difficulty:    4,
				EstimatedTime: 350,
				Encouragement: "平方根の簡単化は因数分解が鍵です！",
				ProblemType:   "平方根",
			},
			{
				Title:         "二等辺三角形の角度",
				Description:   "二等辺三角形ABCで、角A = 角C = 45度のとき、角Bの大きさは何度ですか？",
				Options:       []string{"45度", "60度", "90度", "120度"},
				CorrectAnswer: 2,
				Explanation:   "三角形の内角の和は180度です。角A + 角B + 角C = 180度なので、45度 + 角B + 45度 = 180度、よって角B = 180度 - 90度 = 90度です。",
				Difficulty:    3,
				EstimatedTime: 250,
				Encouragement: "二等辺三角形の性質と三角形の内角の和を理解すれば解けます！",
				ProblemType:   "図形",
			},
		},
	}

	subjectKey := fmt.Sprintf("数学_G%d", grade)
	if problems, exists := mathProblems[grade]; exists && len(problems) > 0 {
		e.mu.Lock()
		index := e.problemIndex[subjectKey] % len(problems)
		e.problemIndex[subjectKey] = index + 1
		e.mu.Unlock()
		return problems[index]
	}

	// デフォルト問題
	return &Problem{
		Title:         "基本計算",
		Description:   "7 + 8 = ?",
		Options:       []string{"14", "15", "16", "17"},
		CorrectAnswer: 1,
		Explanation:   "7 + 8 = 15 です。",
		Difficulty:    1,
		EstimatedTime: 120,
		Encouragement: "計算の基本から始めましょう！",
		ProblemType:   "基本計算",
	}
}

// getEnglishProblem 英語の問題を取得
func (e *Engine) getEnglishProblem(grade, _ int) *Problem {
	englishProblems := []*Problem{
		{
			Title:         "基本英単語",
			Description:   "次の英単語の意味として正しいものを選んでください。\n「book」の意味は？",
			Options:       []string{"本", "ペン", "机", "椅子"},
			CorrectAnswer: 0,
			Explanation:   "「book」は「本」という意味です。基本的な英単語ですね。",
			Difficulty:    2,
			EstimatedTime: 150,
			Encouragement: "英単語を覚えることで英語の理解が深まります！",
			ProblemType:   "語彙",
		},
		{
			Title:         "動詞の意味",
			Description:   "「play」の意味として正しいものは？",
			Options:       []string{"遊ぶ", "食べる", "歩く", "寝る"},
			CorrectAnswer: 0,
			Explanation:   "「play」は「遊ぶ」という意味です。",
			Difficulty:    2,
			EstimatedTime: 140,
			Encouragement: "動詞を理解することが英語上達の鍵です！",
			ProblemType:   "動詞",
		},
		{
			Title:         "形容詞の利用",
			Description:   "「big」の反対の意味の単語は？",
			Options:       []string{"small", "fast", "good", "new"},
			CorrectAnswer: 0,
			Explanation:   "「big」の反対は「small」です。",
			Difficulty:    2,
			EstimatedTime: 130,
			Encouragement: "反対語を覚えると語彙が増えます！",
			ProblemType:   "形容詞",
		},
	}

	subjectKey := fmt.Sprintf("英語_G%d", grade)
	e.mu.Lock()
	index := e.problemIndex[subjectKey] % len(englishProblems)
	e.problemIndex[subjectKey] = index + 1
	e.mu.Unlock()
	return englishProblems[index]
}

// getJapaneseProblem 国語の問題を取得
func (e *Engine) getJapaneseProblem(_, _ int) *Problem {
	return &Problem{
		Title:         "漢字の読み",
		Description:   "次の漢字の読み方として正しいものを選んでください。\n「学習」の読み方は？",
		Options:       []string{"がくしゅう", "がくしゅ", "がくしゆう", "がくし"},
		CorrectAnswer: 0,
		Explanation:   "「学習」は「がくしゅう」と読みます。日々の学習で身につけましょう。",
		Difficulty:    2,
		EstimatedTime: 150,
		Encouragement: "漢字の読み方は練習すれば必ず覚えられます！",
		ProblemType:   "漢字",
	}
}

// getScienceProblem 理科の問題を取得
func (e *Engine) getScienceProblem(_, _ int) *Problem {
	return &Problem{
		Title:         "植物の基本",
		Description:   "植物が光合成を行うために必要なものとして正しくないものはどれですか？",
		Options:       []string{"二酸化炭素", "水", "光", "酸素"},
		CorrectAnswer: 3,
		Explanation:   "光合成には二酸化炭素、水、光が必要です。酸素は光合成の産物です。",
		Difficulty:    3,
		EstimatedTime: 200,
		Encouragement: "生物の仕組みを理解することで自然への理解が深まります！",
		ProblemType:   "生物",
	}
}

// getSocialStudiesProblem 社会の問題を取得
func (e *Engine) getSocialStudiesProblem(_, _ int) *Problem {
	return &Problem{
		Title:         "日本の地理",
		Description:   "日本の首都はどこですか？",
		Options:       []string{"大阪", "京都", "東京", "名古屋"},
		CorrectAnswer: 2,
		Explanation:   "日本の首都は東京です。政治や経済の中心地です。",
		Difficulty:    1,
		EstimatedTime: 120,
		Encouragement: "地理の知識は世界を理解する第一歩です！",
		ProblemType:   "地理",
	}
}

// getGeneralProblem 一般的な問題を取得
func (e *Engine) getGeneralProblem(_, _ int) *Problem {
	return &Problem{
		Title:         "一般常識",
		Description:   "1年は何日でしょうか？（平年の場合）",
		Options:       []string{"364日", "365日", "366日", "367日"},
		CorrectAnswer: 1,
		Explanation:   "平年は365日です。うるう年は366日になります。",
		Difficulty:    1,
		EstimatedTime: 120,
		Encouragement: "基本的な知識から学習を始めましょう！",
		ProblemType:   "一般常識",
	}
}

// Close AIエンジンをクリーンアップ
func (e *Engine) Close() error {
	// HTTPクライアントのクリーンアップは特に必要なし
	return nil
}
