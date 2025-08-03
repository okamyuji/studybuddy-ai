package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config アプリケーション設定
type Config struct {
	// アプリケーション基本設定
	FirstRun     bool   `json:"first_run"`
	UserGrade    int    `json:"user_grade"` // 1:中1, 2:中2, 3:中3
	DatabasePath string `json:"database_path"`

	// AI設定
	AI AIConfig `json:"ai"`

	// UI設定
	UI UIConfig `json:"ui"`

	// 学習設定
	Learning LearningConfig `json:"learning"`
}

// AIConfig AI関連設定
type AIConfig struct {
	Model       string  `json:"model"`       // 使用するAIモデル
	Temperature float64 `json:"temperature"` // 生成の創造性 (0.0-1.0)
	MaxTokens   int     `json:"max_tokens"`  // 最大トークン数
	TopP        float64 `json:"top_p"`       // 核サンプリング確率
	OllamaURL   string  `json:"ollama_url"`  // OllamaサーバーURL
}

// UIConfig UI関連設定
type UIConfig struct {
	DarkMode     bool   `json:"dark_mode"`
	Language     string `json:"language"`  // "ja" | "en"
	FontSize     int    `json:"font_size"` // フォントサイズ
	WindowWidth  int    `json:"window_width"`
	WindowHeight int    `json:"window_height"`
}

// LearningConfig 学習関連設定
type LearningConfig struct {
	EmotionTracking bool     `json:"emotion_tracking"` // 感情分析有効/無効
	SubjectPrefs    []string `json:"subject_prefs"`    // 好きな科目順
	DifficultyLevel int      `json:"difficulty_level"` // 基本難易度 (1-5)
	StudyGoalTime   int      `json:"study_goal_time"`  // 1日の学習目標時間(分)

	// ゲーミフィケーション設定
	PetEnabled bool   `json:"pet_enabled"`
	PetSpecies string `json:"pet_species"` // "cat" | "dog" | "dragon" | "unicorn"
}

// Default デフォルト設定を生成
func Default() *Config {
	homeDir, _ := os.UserHomeDir()
	appDir := filepath.Join(homeDir, ".studybuddy-ai")

	return &Config{
		FirstRun:     true,
		UserGrade:    1,
		DatabasePath: filepath.Join(appDir, "studybuddy.db"),
		AI: AIConfig{
			Model:       "7shi/ezo-gemma-2-jpn:2b-instruct-q8_0",
			Temperature: 0.7,
			MaxTokens:   2048,
			TopP:        0.9,
			OllamaURL:   "http://localhost:11434",
		},
		UI: UIConfig{
			DarkMode:     false,
			Language:     "ja",
			FontSize:     14,
			WindowWidth:  1200,
			WindowHeight: 800,
		},
		Learning: LearningConfig{
			EmotionTracking: false, // 初期は無効（ユーザーの許可後に有効化）
			SubjectPrefs:    []string{"数学", "英語", "国語", "理科", "社会"},
			DifficultyLevel: 3,
			StudyGoalTime:   60, // 60分
			PetEnabled:      true,
			PetSpecies:      "cat",
		},
	}
}

// Load 設定ファイルを読み込み
func Load() (*Config, error) {
	configPath := getConfigPath()

	// 設定ファイルが存在しない場合はデフォルト設定を返す
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return Default(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("設定ファイル読み込みエラー: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("設定ファイル解析エラー: %w", err)
	}

	return &config, nil
}

// Save 設定をファイルに保存
func Save(config *Config) error {
	configPath := getConfigPath()
	configDir := filepath.Dir(configPath)

	// 設定ディレクトリを作成
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("設定ディレクトリ作成エラー: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("設定データ変換エラー: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("設定ファイル保存エラー: %w", err)
	}

	return nil
}

// Validate 設定の妥当性チェック
func (c *Config) Validate() error {
	// 学年チェック
	if c.UserGrade < 1 || c.UserGrade > 3 {
		return fmt.Errorf("無効な学年: %d (1-3である必要があります)", c.UserGrade)
	}

	// AI設定チェック
	if c.AI.Temperature < 0.0 || c.AI.Temperature > 1.0 {
		return fmt.Errorf("無効なTemperature: %f (0.0-1.0である必要があります)", c.AI.Temperature)
	}

	if c.AI.MaxTokens < 1 || c.AI.MaxTokens > 8192 {
		return fmt.Errorf("無効なMaxTokens: %d (1-8192である必要があります)", c.AI.MaxTokens)
	}

	// 学習設定チェック
	if c.Learning.DifficultyLevel < 1 || c.Learning.DifficultyLevel > 5 {
		return fmt.Errorf("無効な難易度レベル: %d (1-5である必要があります)", c.Learning.DifficultyLevel)
	}

	if c.Learning.StudyGoalTime < 10 || c.Learning.StudyGoalTime > 480 {
		return fmt.Errorf("無効な学習目標時間: %d分 (10-480分である必要があります)", c.Learning.StudyGoalTime)
	}

	return nil
}

// UpdateAIModel AIモデルを更新
func (c *Config) UpdateAIModel(model string) {
	c.AI.Model = model
}

// UpdateDifficulty 難易度を更新
func (c *Config) UpdateDifficulty(level int) {
	if level >= 1 && level <= 5 {
		c.Learning.DifficultyLevel = level
	}
}

// ToggleEmotionTracking 感情追跡機能の有効/無効を切り替え
func (c *Config) ToggleEmotionTracking() {
	c.Learning.EmotionTracking = !c.Learning.EmotionTracking
}

// SetPetSpecies ペットの種類を設定
func (c *Config) SetPetSpecies(species string) {
	validSpecies := []string{"cat", "dog", "dragon", "unicorn"}
	for _, valid := range validSpecies {
		if species == valid {
			c.Learning.PetSpecies = species
			return
		}
	}
}

// getConfigPath 設定ファイルのパスを取得
func getConfigPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".studybuddy-ai", "config.json")
}

// IsConfigured アプリケーションが設定済みかチェック
func IsConfigured() bool {
	configPath := getConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return false
	}

	config, err := Load()
	if err != nil {
		return false
	}

	return !config.FirstRun && config.Validate() == nil
}

// GetAppDir アプリケーションデータディレクトリを取得
func GetAppDir() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".studybuddy-ai")
}

// EnsureAppDir アプリケーションディレクトリを確実に作成
func EnsureAppDir() error {
	appDir := GetAppDir()
	return os.MkdirAll(appDir, 0755)
}
