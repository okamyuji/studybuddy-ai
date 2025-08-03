package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"studybuddy-ai/internal/ai"
	"studybuddy-ai/internal/config"
	"studybuddy-ai/internal/database"
	"studybuddy-ai/internal/gui"
)

const (
	AppName    = "StudyBuddy AI"
	AppVersion = "1.0.0"
	AppID      = "studybuddy.ai.app"
)

// AppContext アプリケーション全体のコンテキスト管理
type AppContext struct {
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	cleanupFns []func() error
	mu         sync.Mutex
}

// NewAppContext 新しいアプリケーションコンテキストを作成
func NewAppContext() *AppContext {
	ctx, cancel := context.WithCancel(context.Background())
	return &AppContext{
		ctx:        ctx,
		cancel:     cancel,
		cleanupFns: make([]func() error, 0),
	}
}

// AddCleanup クリーンアップ関数を追加
func (ac *AppContext) AddCleanup(fn func() error) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.cleanupFns = append(ac.cleanupFns, fn)
}

// Shutdown アプリケーションを適切に終了
func (ac *AppContext) Shutdown() {
	log.Println("🛑 アプリケーション終了プロセス開始...")

	// コンテキストをキャンセル
	ac.cancel()

	// すべてのgoroutineの終了を待機（タイムアウト付き）
	done := make(chan struct{})
	go func() {
		ac.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("✅ すべてのgoroutineが正常終了")
	case <-time.After(5 * time.Second):
		log.Println("⚠️ goroutine終了タイムアウト（強制終了）")
	}

	// クリーンアップ関数を逆順で実行
	ac.mu.Lock()
	defer ac.mu.Unlock()

	for i := len(ac.cleanupFns) - 1; i >= 0; i-- {
		if err := ac.cleanupFns[i](); err != nil {
			log.Printf("⚠️ クリーンアップエラー: %v", err)
		}
	}

	log.Println("✅ アプリケーション終了完了")
}

func main() {
	// アプリケーションコンテキスト初期化
	appCtx := NewAppContext()
	defer appCtx.Shutdown() // メイン終了時のクリーンアップ保証

	// シグナルハンドラー設定（Ctrl+C、強制終了対応）
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("🛑 終了シグナル受信")
		appCtx.Shutdown()
		os.Exit(0)
	}()

	// 日本語フォント設定（ビルド後も動作するように実行ファイルからの相対パス）
	setupJapaneseFonts()

	// アプリケーション初期化
	myApp := app.NewWithID(AppID)

	// アプリケーション終了時のクリーンアップを設定
	// Note: SetCloseInterceptはウィンドウレベルで設定（gui.goで実装済み）

	// 設定読み込み
	cfg, err := config.Load()
	if err != nil {
		log.Printf("設定読み込みエラー: %v", err)
		// デフォルト設定で続行
		cfg = config.Default()
	}

	// データベース初期化
	db, err := database.Initialize(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("データベース初期化エラー: %v", err)
	}
	appCtx.AddCleanup(func() error {
		log.Println("📊 データベース接続クローズ")
		return db.Close()
	})

	// AIエンジン初期化
	aiEngine, err := ai.NewEngine(cfg.AI)
	if err != nil {
		log.Printf("AI初期化エラー: %v", err)
		showAISetupDialog(myApp, appCtx)
		return
	}
	appCtx.AddCleanup(func() error {
		log.Println("🤖 AIエンジンクローズ")
		return aiEngine.Close()
	})

	// メインアプリケーション構築
	mainApp := gui.NewMainApp(myApp, db, aiEngine, cfg)
	appCtx.AddCleanup(func() error {
		log.Println("🖥️ GUIシステムクローズ")
		return mainApp.Close()
	})

	// 起動確認ダイアログ
	if cfg.FirstRun {
		showWelcomeDialog(myApp, mainApp, appCtx)
	} else {
		mainApp.Show()
	}

	// アプリケーション実行
	log.Println("🚀 StudyBuddy AI 起動完了")
	myApp.Run()

	// Run()終了後はdefer appCtx.Shutdown()が自動実行される
	log.Println("🏁 メインループ終了")
}

// setupJapaneseFonts 日本語フォント設定（ビルド後も動作する）
func setupJapaneseFonts() {
	// 実行ファイルのディレクトリを取得
	execPath, err := os.Executable()
	if err != nil {
		log.Printf("実行ファイルパス取得エラー: %v", err)
		return
	}
	execDir := filepath.Dir(execPath)

	// フォントファイルの候補パス（ビルド後も動作するように複数指定）
	fontPaths := []string{
		filepath.Join(execDir, "assets", "fonts", "Mplus1-Regular.ttf"), // ビルド後のパス
		"assets/fonts/Mplus1-Regular.ttf",                               // go run での相対パス
		filepath.Join(".", "assets", "fonts", "Mplus1-Regular.ttf"),     // カレントディレクトリから
	}

	// 存在するフォントファイルを探す
	for _, fontPath := range fontPaths {
		if _, err := os.Stat(fontPath); err == nil {
			if err := os.Setenv("FYNE_FONT", fontPath); err != nil {
				log.Printf("フォント環境変数設定エラー: %v", err)
				continue
			}
			log.Printf("日本語フォント設定: %s", fontPath)
			return
		}
	}

	log.Printf("警告: 日本語フォントファイルが見つかりません。デフォルトフォントを使用します。")
}

// AI設定ダイアログ
func showAISetupDialog(app fyne.App, appCtx *AppContext) {
	w := app.NewWindow("AI設定 - StudyBuddy AI")
	w.Resize(fyne.NewSize(500, 300))
	w.CenterOnScreen()

	content := container.NewVBox(
		widget.NewCard("AI設定が必要です", "",
			container.NewVBox(
				widget.NewLabel("StudyBuddy AIを使用するには、ローカルAI (Ollama) の設定が必要です。"),
				widget.NewSeparator(),
				widget.NewRichTextFromMarkdown(`
**必要な手順:**

1. **Ollama をインストール**
   - https://ollama.ai からダウンロード
   - インストール後、ターミナルでOllamaを起動

2. **日本語対応モデルをダウンロード**
   `+"```bash"+`
   ollama pull dsasai/llama3-elyza-jp-8b:latest
   # または
   ollama pull 7shi/ezo-gemma-2-jpn:2b-instruct-q8_0
   `+"```"+`

3. **StudyBuddy AI を再起動**

設定完了後、このアプリケーションを再起動してください。
				`),
			),
		),
		widget.NewButton("設定方法を確認しました", func() {
			log.Println("🛑 AI設定ダイアログから終了")
			appCtx.Shutdown()
			app.Quit()
		}),
	)

	w.SetContent(content)
	w.Show()
}

// ウェルカムダイアログ
func showWelcomeDialog(app fyne.App, mainApp *gui.MainApp, appCtx *AppContext) {
	w := app.NewWindow("ようこそ StudyBuddy AI へ！")
	w.Resize(fyne.NewSize(600, 400))
	w.CenterOnScreen()

	content := container.NewVBox(
		widget.NewCard("🎓 StudyBuddy AI へようこそ！", "",
			container.NewVBox(
				widget.NewRichTextFromMarkdown(`
# あなた専用のAI学習コンパニオン

StudyBuddy AIは、中学生の学習をサポートする革新的なアプリです。

## ✨ 主な機能

- **🤖 AIチューター**: あなたの理解度に合わせた個別指導
- **📊 学習分析**: リアルタイムで学習進捗を追跡
- **🎯 カスタム問題**: 弱点を克服する専用練習問題
- **🔒 プライバシー保護**: すべてのデータは端末内で安全に管理

## 🚀 はじめましょう

最初に、あなたの学習プロファイルを設定します。
どの学年ですか？
				`),
			),
		),
		widget.NewButton("中学1年生", func() {
			startApp(w, mainApp, 1, appCtx)
		}),
		widget.NewButton("中学2年生", func() {
			startApp(w, mainApp, 2, appCtx)
		}),
		widget.NewButton("中学3年生", func() {
			startApp(w, mainApp, 3, appCtx)
		}),
	)

	w.SetContent(content)
	w.Show()
}

func startApp(welcomeWindow fyne.Window, mainApp *gui.MainApp, grade int, _ *AppContext) {
	// 初期設定を保存
	cfg := config.Default()
	cfg.FirstRun = false
	cfg.UserGrade = grade

	if err := config.Save(cfg); err != nil {
		log.Printf("設定保存エラー: %v", err)
	}

	welcomeWindow.Close()
	mainApp.Show()

	// 初回セットアップ完了メッセージ
	fmt.Printf("StudyBuddy AI 初期化完了 - 中学%d年生\n", grade)
}
