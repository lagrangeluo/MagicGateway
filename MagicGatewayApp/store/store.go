package store

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

// ---- Models ----

type User struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"`
	CreatedAt    string `json:"created_at"`
}

type APIKey struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"user_id"`
	KeyHash   string `json:"-"`
	KeyPrefix string `json:"key_prefix"`
	KeyName   string `json:"key_name"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at"`
}

type UsageLog struct {
	ID            int64  `json:"id"`
	APIKeyID      int64  `json:"api_key_id"`
	UserID        int64  `json:"user_id"`
	UserName      string `json:"user_name"`
	Model         string `json:"model"`
	InputTokens   int    `json:"input_tokens"`
	OutputTokens  int    `json:"output_tokens"`
	RequestID     string `json:"request_id"`
	DurationMs    int64  `json:"duration_ms"`
	CreatedAt     string `json:"created_at"`
}

type StatsRow struct {
	UserName     string `json:"user_name"`
	Model        string `json:"model"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	TotalTokens  int    `json:"total_tokens"`
	RequestCount int    `json:"request_count"`
}

type OverviewStats struct {
	TotalUsers   int   `json:"total_users"`
	TotalKeys    int   `json:"total_keys"`
	TodayTokens  int64 `json:"today_tokens"`
	MonthTokens  int64 `json:"month_tokens"`
}

// ---- Init ----

func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(1) // SQLite serializes writes

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	schema := `
	PRAGMA journal_mode=WAL;
	PRAGMA foreign_keys=ON;

	CREATE TABLE IF NOT EXISTS users (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		username        TEXT UNIQUE NOT NULL,
		password_hash   TEXT NOT NULL,
		role            TEXT NOT NULL DEFAULT 'user',
		created_at      DATETIME DEFAULT (datetime('now','localtime'))
	);

	CREATE TABLE IF NOT EXISTS api_keys (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id     INTEGER NOT NULL,
		key_hash    TEXT UNIQUE NOT NULL,
		key_prefix  TEXT NOT NULL,
		key_name    TEXT DEFAULT '',
		is_active   INTEGER DEFAULT 1,
		created_at  DATETIME DEFAULT (datetime('now','localtime')),
		FOREIGN KEY (user_id) REFERENCES users(id)
	);

	CREATE TABLE IF NOT EXISTS usage_logs (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		api_key_id      INTEGER NOT NULL,
		user_id         INTEGER NOT NULL,
		user_name       TEXT NOT NULL,
		model           TEXT DEFAULT '',
		input_tokens    INTEGER DEFAULT 0,
		output_tokens   INTEGER DEFAULT 0,
		request_id      TEXT DEFAULT '',
		duration_ms     INTEGER DEFAULT 0,
		created_at      DATETIME DEFAULT (datetime('now','localtime')),
		FOREIGN KEY (api_key_id) REFERENCES api_keys(id),
		FOREIGN KEY (user_id) REFERENCES users(id)
	);

	CREATE INDEX IF NOT EXISTS idx_usage_created ON usage_logs(created_at);
	CREATE INDEX IF NOT EXISTS idx_usage_user   ON usage_logs(user_id);
	CREATE INDEX IF NOT EXISTS idx_usage_key    ON usage_logs(api_key_id);
	`
	_, err := s.db.Exec(schema)
	return err
}

// ---- User methods ----

func (s *Store) CreateDefaultAdmin(username, password string) error {
	existing, _ := s.GetUserByUsername(username)
	if existing != nil {
		return nil // already exists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(
		"INSERT INTO users (username, password_hash, role) VALUES (?, ?, 'admin')",
		username, string(hash),
	)
	return err
}

func (s *Store) CreateUser(username, password string) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	res, err := s.db.Exec(
		"INSERT INTO users (username, password_hash, role) VALUES (?, ?, 'user')",
		username, string(hash),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, fmt.Errorf("username already exists")
		}
		return nil, err
	}

	id, _ := res.LastInsertId()
	return s.GetUserByID(id)
}

func (s *Store) UpdatePassword(userID int64, passwordHash string) error {
	_, err := s.db.Exec("UPDATE users SET password_hash = ? WHERE id = ?", passwordHash, userID)
	return err
}

func (s *Store) GetUserByUsername(username string) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(
		"SELECT id, username, password_hash, role, created_at FROM users WHERE username = ?",
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Store) GetUserByID(id int64) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(
		"SELECT id, username, password_hash, role, created_at FROM users WHERE id = ?",
		id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query("SELECT id, username, role, created_at FROM users ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (s *Store) DeleteUser(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM usage_logs WHERE user_id = ?", id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM api_keys WHERE user_id = ?", id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM users WHERE id = ? AND role != 'admin'", id); err != nil {
		return err
	}
	return tx.Commit()
}

// ---- API Key methods ----

func GenerateAPIKey() (fullKey, hash, prefix string) {
	b := make([]byte, 16)
	rand.Read(b)
	randomPart := hex.EncodeToString(b)
	fullKey = "sk-magic-" + randomPart

	h := sha256.Sum256([]byte(fullKey))
	hash = hex.EncodeToString(h[:])

	prefix = fullKey[:20] // "sk-magic-" + first 8 hex chars
	return
}

func (s *Store) CreateKey(userID int64, keyName string) (fullKey string, key *APIKey, err error) {
	fullKey, hash, prefix := GenerateAPIKey()

	res, err := s.db.Exec(
		"INSERT INTO api_keys (user_id, key_hash, key_prefix, key_name) VALUES (?, ?, ?, ?)",
		userID, hash, prefix, keyName,
	)
	if err != nil {
		return "", nil, err
	}

	id, _ := res.LastInsertId()
	key, err = s.GetKeyByID(id)
	return fullKey, key, err
}

func (s *Store) GetKeyByHash(hash string) (*APIKey, error) {
	k := &APIKey{}
	err := s.db.QueryRow(
		"SELECT id, user_id, key_hash, key_prefix, key_name, is_active, created_at FROM api_keys WHERE key_hash = ?",
		hash,
	).Scan(&k.ID, &k.UserID, &k.KeyHash, &k.KeyPrefix, &k.KeyName, &k.IsActive, &k.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return k, nil
}

func (s *Store) GetKeyByID(id int64) (*APIKey, error) {
	k := &APIKey{}
	err := s.db.QueryRow(
		"SELECT id, user_id, key_hash, key_prefix, key_name, is_active, created_at FROM api_keys WHERE id = ?",
		id,
	).Scan(&k.ID, &k.UserID, &k.KeyHash, &k.KeyPrefix, &k.KeyName, &k.IsActive, &k.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return k, nil
}

func (s *Store) ListKeysByUserID(userID int64) ([]APIKey, error) {
	rows, err := s.db.Query(
		"SELECT id, user_id, key_prefix, key_name, is_active, created_at FROM api_keys WHERE user_id = ? AND is_active = 1 ORDER BY id",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.KeyPrefix, &k.KeyName, &k.IsActive, &k.CreatedAt); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, nil
}

func (s *Store) ListAllKeys(userIDFilter int64) ([]APIKey, error) {
	var rows *sql.Rows
	var err error
	if userIDFilter > 0 {
		rows, err = s.db.Query(
			"SELECT id, user_id, key_prefix, key_name, is_active, created_at FROM api_keys WHERE user_id = ? ORDER BY id",
			userIDFilter,
		)
	} else {
		rows, err = s.db.Query(
			"SELECT id, user_id, key_prefix, key_name, is_active, created_at FROM api_keys ORDER BY id",
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.KeyPrefix, &k.KeyName, &k.IsActive, &k.CreatedAt); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, nil
}

func (s *Store) RevokeKey(id int64) error {
	_, err := s.db.Exec("UPDATE api_keys SET is_active = 0 WHERE id = ?", id)
	return err
}

func (s *Store) EnableKey(id int64) error {
	_, err := s.db.Exec("UPDATE api_keys SET is_active = 1 WHERE id = ?", id)
	return err
}

// ---- Usage methods ----

func (s *Store) InsertUsage(apiKeyID, userID int64, userName, model, requestID string, inputTokens, outputTokens int, durationMs int64) error {
	_, err := s.db.Exec(
		`INSERT INTO usage_logs (api_key_id, user_id, user_name, model, request_id, input_tokens, output_tokens, duration_ms)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		apiKeyID, userID, userName, model, requestID, inputTokens, outputTokens, durationMs,
	)
	return err
}

func (s *Store) GetUserStats(userID int64, period, date string) ([]StatsRow, error) {
	var query string
	var args []interface{}

	switch period {
	case "daily":
		query = `
			SELECT user_name, model,
			       SUM(input_tokens) as input_tokens,
			       SUM(output_tokens) as output_tokens,
			       COUNT(*) as request_count
			FROM usage_logs
			WHERE user_id = ? AND date(created_at) = ?
			GROUP BY user_name, model
			ORDER BY user_name`
		args = []interface{}{userID, date}
	case "weekly":
		weekStart, err := time.Parse("2006-01-02", date)
		if err != nil {
			return nil, fmt.Errorf("invalid date format: %s", date)
		}
		weekEnd := weekStart.AddDate(0, 0, 7)
		query = `
			SELECT user_name, model,
			       SUM(input_tokens) as input_tokens,
			       SUM(output_tokens) as output_tokens,
			       COUNT(*) as request_count
			FROM usage_logs
			WHERE user_id = ? AND created_at >= ? AND created_at < ?
			GROUP BY user_name, model
			ORDER BY user_name`
		args = []interface{}{userID, weekStart.Format("2006-01-02"), weekEnd.Format("2006-01-02")}
	case "monthly":
		query = `
			SELECT user_name, model,
			       SUM(input_tokens) as input_tokens,
			       SUM(output_tokens) as output_tokens,
			       COUNT(*) as request_count
			FROM usage_logs
			WHERE user_id = ? AND strftime('%Y-%m', created_at) = ?
			GROUP BY user_name, model
			ORDER BY user_name`
		args = []interface{}{userID, date}
	case "yearly":
		query = `
			SELECT user_name, model,
			       SUM(input_tokens) as input_tokens,
			       SUM(output_tokens) as output_tokens,
			       COUNT(*) as request_count
			FROM usage_logs
			WHERE user_id = ? AND strftime('%Y', created_at) = ?
			GROUP BY user_name, model
			ORDER BY user_name`
		args = []interface{}{userID, date}
	default:
		return nil, fmt.Errorf("invalid period: %s", period)
	}

	return s.queryStats(query, args...)
}

func (s *Store) GetAllUserStats(period, date string, userID int64) ([]StatsRow, error) {
	var query string
	var args []interface{}

	switch period {
	case "daily":
		query = `
			SELECT user_name, model,
			       SUM(input_tokens) as input_tokens,
			       SUM(output_tokens) as output_tokens,
			       COUNT(*) as request_count
			FROM usage_logs
			WHERE date(created_at) = ?
			GROUP BY user_name, model
			ORDER BY user_name`
		args = []interface{}{date}
	case "weekly":
		weekStart, err := time.Parse("2006-01-02", date)
		if err != nil {
			return nil, fmt.Errorf("invalid date format: %s", date)
		}
		weekEnd := weekStart.AddDate(0, 0, 7)
		query = `
			SELECT user_name, model,
			       SUM(input_tokens) as input_tokens,
			       SUM(output_tokens) as output_tokens,
			       COUNT(*) as request_count
			FROM usage_logs
			WHERE created_at >= ? AND created_at < ?
			GROUP BY user_name, model
			ORDER BY user_name`
		args = []interface{}{weekStart.Format("2006-01-02"), weekEnd.Format("2006-01-02")}
	case "monthly":
		query = `
			SELECT user_name, model,
			       SUM(input_tokens) as input_tokens,
			       SUM(output_tokens) as output_tokens,
			       COUNT(*) as request_count
			FROM usage_logs
			WHERE strftime('%Y-%m', created_at) = ?
			GROUP BY user_name, model
			ORDER BY user_name`
		args = []interface{}{date}
	case "yearly":
		query = `
			SELECT user_name, model,
			       SUM(input_tokens) as input_tokens,
			       SUM(output_tokens) as output_tokens,
			       COUNT(*) as request_count
			FROM usage_logs
			WHERE strftime('%Y', created_at) = ?
			GROUP BY user_name, model
			ORDER BY user_name`
		args = []interface{}{date}
	default:
		return nil, fmt.Errorf("invalid period: %s", period)
	}

	if userID > 0 {
		query = strings.Replace(query, "WHERE", "WHERE user_id = ? AND", 1)
		args = append([]interface{}{userID}, args...)
	}

	return s.queryStats(query, args...)
}

func (s *Store) queryStats(query string, args ...interface{}) ([]StatsRow, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []StatsRow
	for rows.Next() {
		var r StatsRow
		if err := rows.Scan(&r.UserName, &r.Model, &r.InputTokens, &r.OutputTokens, &r.RequestCount); err != nil {
			return nil, err
		}
		r.TotalTokens = r.InputTokens + r.OutputTokens
		stats = append(stats, r)
	}
	return stats, nil
}

func (s *Store) GetOverview() (*OverviewStats, error) {
	o := &OverviewStats{}

	if err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&o.TotalUsers); err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}
	if err := s.db.QueryRow("SELECT COUNT(*) FROM api_keys WHERE is_active = 1").Scan(&o.TotalKeys); err != nil {
		return nil, fmt.Errorf("count keys: %w", err)
	}
	if err := s.db.QueryRow(
		"SELECT COALESCE(SUM(input_tokens + output_tokens), 0) FROM usage_logs WHERE date(created_at) = date('now','localtime')",
	).Scan(&o.TodayTokens); err != nil {
		return nil, fmt.Errorf("today tokens: %w", err)
	}
	if err := s.db.QueryRow(
		"SELECT COALESCE(SUM(input_tokens + output_tokens), 0) FROM usage_logs WHERE strftime('%Y-%m', created_at) = strftime('%Y-%m', 'now', 'localtime')",
	).Scan(&o.MonthTokens); err != nil {
		return nil, fmt.Errorf("month tokens: %w", err)
	}

	return o, nil
}

type UserStatsRow struct {
	User
	KeyCount    int   `json:"key_count"`
	MonthTokens int64 `json:"month_tokens"`
}

func (s *Store) GetUsersWithStats() ([]UserStatsRow, error) {
	rows, err := s.db.Query(`
		SELECT u.id, u.username, u.role, u.created_at,
			(SELECT COUNT(*) FROM api_keys k WHERE k.user_id = u.id AND k.is_active = 1) as key_count,
			COALESCE((SELECT SUM(input_tokens + output_tokens) FROM usage_logs l WHERE l.user_id = u.id AND strftime('%Y-%m', l.created_at) = strftime('%Y-%m', 'now', 'localtime')), 0) as month_tokens
		FROM users u ORDER BY u.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []UserStatsRow
	for rows.Next() {
		var u UserStatsRow
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt, &u.KeyCount, &u.MonthTokens); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if users == nil {
		users = []UserStatsRow{}
	}
	return users, nil
}

type RankingRow struct {
	UserName     string `json:"user_name"`
	TotalTokens  int64  `json:"total_tokens"`
	RequestCount int    `json:"request_count"`
}

func (s *Store) GetRanking(period, date string) ([]RankingRow, error) {
	var query string
	var args []interface{}

	switch period {
	case "daily":
		query = `SELECT user_name, SUM(input_tokens + output_tokens), COUNT(*)
		           FROM usage_logs WHERE date(created_at) = ?
		           GROUP BY user_name ORDER BY 2 DESC`
		args = []interface{}{date}
	case "weekly":
		weekStart, err := time.Parse("2006-01-02", date)
		if err != nil {
			return nil, fmt.Errorf("invalid date format: %s", date)
		}
		weekEnd := weekStart.AddDate(0, 0, 7)
		query = `SELECT user_name, SUM(input_tokens + output_tokens), COUNT(*)
		           FROM usage_logs WHERE created_at >= ? AND created_at < ?
		           GROUP BY user_name ORDER BY 2 DESC`
		args = []interface{}{weekStart.Format("2006-01-02"), weekEnd.Format("2006-01-02")}
	case "monthly":
		query = `SELECT user_name, SUM(input_tokens + output_tokens), COUNT(*)
		           FROM usage_logs WHERE strftime('%Y-%m', created_at) = ?
		           GROUP BY user_name ORDER BY 2 DESC`
		args = []interface{}{date}
	case "yearly":
		query = `SELECT user_name, SUM(input_tokens + output_tokens), COUNT(*)
		           FROM usage_logs WHERE strftime('%Y', created_at) = ?
		           GROUP BY user_name ORDER BY 2 DESC`
		args = []interface{}{date}
	default:
		return nil, fmt.Errorf("invalid period: %s", period)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ranking []RankingRow
	for rows.Next() {
		var r RankingRow
		if err := rows.Scan(&r.UserName, &r.TotalTokens, &r.RequestCount); err != nil {
			return nil, err
		}
		ranking = append(ranking, r)
	}
	if ranking == nil {
		ranking = []RankingRow{}
	}
	return ranking, nil
}

type BreakdownRow struct {
	Label        string `json:"label"`
	TotalTokens  int64  `json:"total_tokens"`
	RequestCount int    `json:"request_count"`
}

func (s *Store) GetBreakdown(userID int64, period, date string) ([]BreakdownRow, error) {
	var rows []BreakdownRow

	switch period {
	case "weekly":
		d, err := time.Parse("2006-01-02", date)
		if err != nil {
			return nil, fmt.Errorf("invalid date format: %s", date)
		}
		wd := d.Weekday()
		if wd == 0 { wd = 7 }
		d = d.AddDate(0, 0, -int(wd-1))
		labels := []string{"周一","周二","周三","周四","周五","周六","周日"}
		for i := 0; i < 7; i++ {
			day := d.AddDate(0, 0, i).Format("2006-01-02")
			var total int64; var count int
			if err := s.db.QueryRow("SELECT COALESCE(SUM(input_tokens+output_tokens),0), COUNT(*) FROM usage_logs WHERE user_id=? AND date(created_at)=?", userID, day).Scan(&total, &count); err != nil {
				return nil, fmt.Errorf("breakdown weekly scan: %w", err)
			}
			rows = append(rows, BreakdownRow{Label: labels[i], TotalTokens: total, RequestCount: count})
		}
	case "monthly":
		firstDay, err := time.Parse("2006-01", date)
		if err != nil {
			return nil, fmt.Errorf("invalid date format: %s", date)
		}
		lastDay := firstDay.AddDate(0, 1, -1)
		for d := firstDay; !d.After(lastDay); d = d.AddDate(0, 0, 1) {
			dayStr := d.Format("2006-01-02")
			var total int64; var count int
			if err := s.db.QueryRow("SELECT COALESCE(SUM(input_tokens+output_tokens),0), COUNT(*) FROM usage_logs WHERE user_id=? AND date(created_at)=?", userID, dayStr).Scan(&total, &count); err != nil {
				return nil, fmt.Errorf("breakdown monthly scan: %w", err)
			}
			rows = append(rows, BreakdownRow{Label: fmt.Sprintf("%d日", d.Day()), TotalTokens: total, RequestCount: count})
		}
	case "yearly":
		for m := 1; m <= 12; m++ {
			mStr := fmt.Sprintf("%s-%02d", date, m)
			var total int64; var count int
			if err := s.db.QueryRow("SELECT COALESCE(SUM(input_tokens+output_tokens),0), COUNT(*) FROM usage_logs WHERE user_id=? AND strftime('%Y-%m',created_at)=?", userID, mStr).Scan(&total, &count); err != nil {
				return nil, fmt.Errorf("breakdown yearly scan: %w", err)
			}
			rows = append(rows, BreakdownRow{Label: fmt.Sprintf("%d月", m), TotalTokens: total, RequestCount: count})
		}
	}
	return rows, nil
}

func (s *Store) GetKeyLastUsed(apiKeyID int64) (string, int) {
	var lastUsed string
	var totalTokens int
	err := s.db.QueryRow(
		"SELECT created_at, input_tokens + output_tokens FROM usage_logs WHERE api_key_id = ? ORDER BY id DESC LIMIT 1",
		apiKeyID,
	).Scan(&lastUsed, &totalTokens)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("[store] GetKeyLastUsed key=%d: %v", apiKeyID, err)
	}
	return lastUsed, totalTokens
}
