package theme

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// JapaneseTheme M+フォントを使用した日本語対応テーマ
type JapaneseTheme struct{}

// NewJapaneseTheme 新しい日本語テーマを作成
func NewJapaneseTheme() fyne.Theme {
	return &JapaneseTheme{}
}

// Font フォントリソースを返す
func (t *JapaneseTheme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Bold {
		return resourceMplus1BoldTtf
	}
	return resourceMplus1RegularTtf
}

// Color 色を返す（デフォルトテーマを使用）
func (t *JapaneseTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return theme.DefaultTheme().Color(name, variant)
}

// Icon アイコンを返す（デフォルトテーマを使用）
func (t *JapaneseTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

// Size サイズを返す（デフォルトテーマを使用）
func (t *JapaneseTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}