package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DB データベース接続
type DB struct {
	*sql.DB
}

// Initialize データベースを初期化
func Initialize(dbPath string) (*DB, error) {
	// データベースディレクトリを作成
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("データベースディレクトリ作成エラー: %w", err)
	}

	// データベース接続
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("データベース接続エラー: %w", err)
	}

	// 接続テスト
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("データベース接続テストエラー: %w", err)
	}

	wrapper := &DB{db}

	// スキーマ作成
	if err := wrapper.createSchema(); err != nil {
		return nil, fmt.Errorf("スキーマ作成エラー: %w", err)
	}

	return wrapper, nil
}

// createSchema データベーススキーマを作成
func (db *DB) createSchema() error {
	schemas := []string{
		createUsersTable,
		createStudySessionsTable,
		createProblemResultsTable,
		createLearningProgressTable,
		createVirtualPetsTable,
		createErrorPatternsTable,
		createIndices,
	}

	for _, schema := range schemas {
		if _, err := db.Exec(schema); err != nil {
			return fmt.Errorf("スキーマ実行エラー: %w", err)
		}
	}

	return nil
}

// ユーザーテーブル作成SQL
const createUsersTable = `
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    grade INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_login DATETIME,
    CONSTRAINT valid_grade CHECK (grade BETWEEN 1 AND 3)
);`

// 学習セッションテーブル作成SQL
const createStudySessionsTable = `
CREATE TABLE IF NOT EXISTS study_sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    subject TEXT NOT NULL,
    start_time DATETIME NOT NULL,
    end_time DATETIME,
    total_problems INTEGER DEFAULT 0,
    correct_answers INTEGER DEFAULT 0,
    average_emotion TEXT DEFAULT 'neutral',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id),
    CONSTRAINT valid_subject CHECK (subject IN ('数学', '英語', '国語', '理科', '社会'))
);`

// 問題解答記録テーブル作成SQL
const createProblemResultsTable = `
CREATE TABLE IF NOT EXISTS problem_results (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    problem_type TEXT NOT NULL,
    difficulty INTEGER NOT NULL,
    is_correct BOOLEAN NOT NULL,
    time_taken INTEGER NOT NULL,
    emotion_at_answer TEXT,
    error_category TEXT,
    problem_content TEXT,
    user_answer TEXT,
    correct_answer TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES study_sessions(id),
    CONSTRAINT valid_difficulty CHECK (difficulty BETWEEN 1 AND 5)
);`

// 学習進捗統計テーブル作成SQL
const createLearningProgressTable = `
CREATE TABLE IF NOT EXISTS learning_progress (
    user_id TEXT NOT NULL,
    subject TEXT NOT NULL,
    total_problems INTEGER DEFAULT 0,
    correct_answers INTEGER DEFAULT 0,
    total_study_time INTEGER DEFAULT 0,
    study_streak INTEGER DEFAULT 0,
    last_study_date DATE,
    strength_areas TEXT,
    weakness_areas TEXT,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, subject),
    FOREIGN KEY (user_id) REFERENCES users(id),
    CONSTRAINT valid_subject CHECK (subject IN ('数学', '英語', '国語', '理科', '社会'))
);`

// バーチャルペットテーブル作成SQL
const createVirtualPetsTable = `
CREATE TABLE IF NOT EXISTS virtual_pets (
    user_id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    species TEXT NOT NULL,
    level INTEGER DEFAULT 1,
    experience INTEGER DEFAULT 0,
    health INTEGER DEFAULT 100,
    happiness INTEGER DEFAULT 100,
    intelligence INTEGER DEFAULT 50,
    evolution TEXT DEFAULT 'basic',
    last_fed DATETIME,
    last_played DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id),
    CONSTRAINT valid_species CHECK (species IN ('cat', 'dog', 'dragon', 'unicorn')),
    CONSTRAINT valid_level CHECK (level >= 1),
    CONSTRAINT valid_health CHECK (health BETWEEN 0 AND 100),
    CONSTRAINT valid_happiness CHECK (happiness BETWEEN 0 AND 100),
    CONSTRAINT valid_intelligence CHECK (intelligence BETWEEN 0 AND 100)
);`

// 間違いパターン分析テーブル作成SQL
const createErrorPatternsTable = `
CREATE TABLE IF NOT EXISTS error_patterns (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    subject TEXT NOT NULL,
    problem_type TEXT NOT NULL,
    error_type TEXT NOT NULL,
    frequency INTEGER DEFAULT 1,
    last_occurred DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_resolved BOOLEAN DEFAULT FALSE,
    resolution_date DATETIME,
    FOREIGN KEY (user_id) REFERENCES users(id),
    CONSTRAINT valid_subject CHECK (subject IN ('数学', '英語', '国語', '理科', '社会'))
);`

// インデックス作成SQL
const createIndices = `
CREATE INDEX IF NOT EXISTS idx_study_sessions_user_id ON study_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_study_sessions_subject ON study_sessions(subject);
CREATE INDEX IF NOT EXISTS idx_study_sessions_start_time ON study_sessions(start_time);
CREATE INDEX IF NOT EXISTS idx_problem_results_session_id ON problem_results(session_id);
CREATE INDEX IF NOT EXISTS idx_problem_results_is_correct ON problem_results(is_correct);
CREATE INDEX IF NOT EXISTS idx_error_patterns_user_subject ON error_patterns(user_id, subject);
CREATE INDEX IF NOT EXISTS idx_learning_progress_last_study ON learning_progress(last_study_date);
`

// User ユーザー構造体
type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Grade     int       `json:"grade"`
	CreatedAt time.Time `json:"created_at"`
	LastLogin *time.Time `json:"last_login"`
}

// StudySession 学習セッション構造体
type StudySession struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	Subject        string    `json:"subject"`
	StartTime      time.Time `json:"start_time"`
	EndTime        *time.Time `json:"end_time"`
	TotalProblems  int       `json:"total_problems"`
	CorrectAnswers int       `json:"correct_answers"`
	AverageEmotion string    `json:"average_emotion"`
	CreatedAt      time.Time `json:"created_at"`
}

// ProblemResult 問題解答結果構造体
type ProblemResult struct {
	ID              string    `json:"id"`
	SessionID       string    `json:"session_id"`
	ProblemType     string    `json:"problem_type"`
	Difficulty      int       `json:"difficulty"`
	IsCorrect       bool      `json:"is_correct"`
	TimeTaken       int       `json:"time_taken"`
	EmotionAtAnswer string    `json:"emotion_at_answer"`
	ErrorCategory   string    `json:"error_category"`
	ProblemContent  string    `json:"problem_content"`
	UserAnswer      string    `json:"user_answer"`
	CorrectAnswer   string    `json:"correct_answer"`
	CreatedAt       time.Time `json:"created_at"`
}

// LearningProgress 学習進捗構造体
type LearningProgress struct {
	UserID         string    `json:"user_id"`
	Subject        string    `json:"subject"`
	TotalProblems  int       `json:"total_problems"`
	CorrectAnswers int       `json:"correct_answers"`
	TotalStudyTime int       `json:"total_study_time"`
	StudyStreak    int       `json:"study_streak"`
	LastStudyDate  *time.Time `json:"last_study_date"`
	StrengthAreas  string    `json:"strength_areas"`
	WeaknessAreas  string    `json:"weakness_areas"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// VirtualPet バーチャルペット構造体
type VirtualPet struct {
	UserID       string     `json:"user_id"`
	Name         string     `json:"name"`
	Species      string     `json:"species"`
	Level        int        `json:"level"`
	Experience   int        `json:"experience"`
	Health       int        `json:"health"`
	Happiness    int        `json:"happiness"`
	Intelligence int        `json:"intelligence"`
	Evolution    string     `json:"evolution"`
	LastFed      *time.Time `json:"last_fed"`
	LastPlayed   *time.Time `json:"last_played"`
	CreatedAt    time.Time  `json:"created_at"`
}

// ErrorPattern 間違いパターン構造体
type ErrorPattern struct {
	ID             string     `json:"id"`
	UserID         string     `json:"user_id"`
	Subject        string     `json:"subject"`
	ProblemType    string     `json:"problem_type"`
	ErrorType      string     `json:"error_type"`
	Frequency      int        `json:"frequency"`
	LastOccurred   time.Time  `json:"last_occurred"`
	IsResolved     bool       `json:"is_resolved"`
	ResolutionDate *time.Time `json:"resolution_date"`
}

// CreateUser ユーザー作成
func (db *DB) CreateUser(user *User) error {
	query := `
		INSERT INTO users (id, name, grade, created_at, last_login)
		VALUES (?, ?, ?, ?, ?)
	`
	_, err := db.Exec(query, user.ID, user.Name, user.Grade, user.CreatedAt, user.LastLogin)
	return err
}

// GetUser ユーザー取得
func (db *DB) GetUser(userID string) (*User, error) {
	query := `
		SELECT id, name, grade, created_at, last_login
		FROM users WHERE id = ?
	`
	row := db.QueryRow(query, userID)
	
	var user User
	err := row.Scan(&user.ID, &user.Name, &user.Grade, &user.CreatedAt, &user.LastLogin)
	if err != nil {
		return nil, err
	}
	
	return &user, nil
}

// UpdateUserLastLogin ユーザーの最終ログイン時刻を更新
func (db *DB) UpdateUserLastLogin(userID string) error {
	query := `UPDATE users SET last_login = ? WHERE id = ?`
	_, err := db.Exec(query, time.Now(), userID)
	return err
}

// CreateStudySession 学習セッション作成
func (db *DB) CreateStudySession(session *StudySession) error {
	query := `
		INSERT INTO study_sessions (id, user_id, subject, start_time, end_time, total_problems, correct_answers, average_emotion, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := db.Exec(query, session.ID, session.UserID, session.Subject, session.StartTime, 
		session.EndTime, session.TotalProblems, session.CorrectAnswers, session.AverageEmotion, session.CreatedAt)
	return err
}

// UpdateStudySession 学習セッション更新
func (db *DB) UpdateStudySession(session *StudySession) error {
	query := `
		UPDATE study_sessions 
		SET end_time = ?, total_problems = ?, correct_answers = ?, average_emotion = ?
		WHERE id = ?
	`
	_, err := db.Exec(query, session.EndTime, session.TotalProblems, session.CorrectAnswers, 
		session.AverageEmotion, session.ID)
	return err
}

// CreateProblemResult 問題解答結果作成
func (db *DB) CreateProblemResult(result *ProblemResult) error {
	query := `
		INSERT INTO problem_results (id, session_id, problem_type, difficulty, is_correct, time_taken, 
			emotion_at_answer, error_category, problem_content, user_answer, correct_answer, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := db.Exec(query, result.ID, result.SessionID, result.ProblemType, result.Difficulty,
		result.IsCorrect, result.TimeTaken, result.EmotionAtAnswer, result.ErrorCategory,
		result.ProblemContent, result.UserAnswer, result.CorrectAnswer, result.CreatedAt)
	return err
}

// GetLearningProgress 学習進捗取得
func (db *DB) GetLearningProgress(userID, subject string) (*LearningProgress, error) {
	query := `
		SELECT user_id, subject, total_problems, correct_answers, total_study_time, 
			study_streak, last_study_date, strength_areas, weakness_areas, updated_at
		FROM learning_progress WHERE user_id = ? AND subject = ?
	`
	row := db.QueryRow(query, userID, subject)
	
	var progress LearningProgress
	err := row.Scan(&progress.UserID, &progress.Subject, &progress.TotalProblems,
		&progress.CorrectAnswers, &progress.TotalStudyTime, &progress.StudyStreak,
		&progress.LastStudyDate, &progress.StrengthAreas, &progress.WeaknessAreas, &progress.UpdatedAt)
	
	if err == sql.ErrNoRows {
		// 初回の場合は空の進捗を返す
		return &LearningProgress{
			UserID:  userID,
			Subject: subject,
		}, nil
	}
	
	if err != nil {
		return nil, err
	}
	
	return &progress, nil
}

// UpsertLearningProgress 学習進捗更新（INSERT or UPDATE）
func (db *DB) UpsertLearningProgress(progress *LearningProgress) error {
	query := `
		INSERT INTO learning_progress (user_id, subject, total_problems, correct_answers, 
			total_study_time, study_streak, last_study_date, strength_areas, weakness_areas, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, subject) DO UPDATE SET
			total_problems = excluded.total_problems,
			correct_answers = excluded.correct_answers,
			total_study_time = excluded.total_study_time,
			study_streak = excluded.study_streak,
			last_study_date = excluded.last_study_date,
			strength_areas = excluded.strength_areas,
			weakness_areas = excluded.weakness_areas,
			updated_at = excluded.updated_at
	`
	_, err := db.Exec(query, progress.UserID, progress.Subject, progress.TotalProblems,
		progress.CorrectAnswers, progress.TotalStudyTime, progress.StudyStreak,
		progress.LastStudyDate, progress.StrengthAreas, progress.WeaknessAreas, time.Now())
	return err
}

// CreateVirtualPet バーチャルペット作成
func (db *DB) CreateVirtualPet(pet *VirtualPet) error {
	query := `
		INSERT INTO virtual_pets (user_id, name, species, level, experience, health, 
			happiness, intelligence, evolution, last_fed, last_played, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := db.Exec(query, pet.UserID, pet.Name, pet.Species, pet.Level, pet.Experience,
		pet.Health, pet.Happiness, pet.Intelligence, pet.Evolution, pet.LastFed, pet.LastPlayed, pet.CreatedAt)
	return err
}

// GetVirtualPet バーチャルペット取得
func (db *DB) GetVirtualPet(userID string) (*VirtualPet, error) {
	query := `
		SELECT user_id, name, species, level, experience, health, happiness, 
			intelligence, evolution, last_fed, last_played, created_at
		FROM virtual_pets WHERE user_id = ?
	`
	row := db.QueryRow(query, userID)
	
	var pet VirtualPet
	err := row.Scan(&pet.UserID, &pet.Name, &pet.Species, &pet.Level, &pet.Experience,
		&pet.Health, &pet.Happiness, &pet.Intelligence, &pet.Evolution, &pet.LastFed, &pet.LastPlayed, &pet.CreatedAt)
	
	if err != nil {
		return nil, err
	}
	
	return &pet, nil
}

// UpdateVirtualPet バーチャルペット更新
func (db *DB) UpdateVirtualPet(pet *VirtualPet) error {
	query := `
		UPDATE virtual_pets SET name = ?, level = ?, experience = ?, health = ?, 
			happiness = ?, intelligence = ?, evolution = ?, last_fed = ?, last_played = ?
		WHERE user_id = ?
	`
	_, err := db.Exec(query, pet.Name, pet.Level, pet.Experience, pet.Health,
		pet.Happiness, pet.Intelligence, pet.Evolution, pet.LastFed, pet.LastPlayed, pet.UserID)
	return err
}

// GetRecentStudySessions 最近の学習セッション取得
func (db *DB) GetRecentStudySessions(userID string, limit int) ([]StudySession, error) {
	query := `
		SELECT id, user_id, subject, start_time, end_time, total_problems, 
			correct_answers, average_emotion, created_at
		FROM study_sessions 
		WHERE user_id = ? 
		ORDER BY start_time DESC 
		LIMIT ?
	`
	rows, err := db.Query(query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	
	var sessions []StudySession
	for rows.Next() {
		var session StudySession
		err := rows.Scan(&session.ID, &session.UserID, &session.Subject, &session.StartTime,
			&session.EndTime, &session.TotalProblems, &session.CorrectAnswers, 
			&session.AverageEmotion, &session.CreatedAt)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	
	return sessions, nil
}

// Cleanup データベース接続を閉じる
func (db *DB) Cleanup() error {
	return db.Close()
}
