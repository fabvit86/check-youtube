package database

import (
	"checkYoutube/configs"
	"checkYoutube/logging"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
)

//go:embed migrations/schema.sql
var schema []byte

type StorageInterface interface {
	Init() error
	RunMigrations() error
	GetRefreshTokenByUserId(userId string) (string, error)
	UpsertRefreshToken(userId, refreshToken string) error
}

type Storage struct {
	db *sql.DB
}

func (s *Storage) Init() error {
	const funcName = "Init"

	dbPath, err := configs.GetEnvOrErr("SQLITE_DB_PATH")
	if err != nil {
		slog.Error(fmt.Sprintf("failed to get database path: %s", err.Error()), logging.FuncNameAttr(funcName))
		return err
	}

	s.db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		slog.Error(fmt.Sprintf("failed to connect to database: %s", err.Error()), logging.FuncNameAttr(funcName))
		return err
	}

	slog.Info("sqLite database connection created", logging.FuncNameAttr(funcName))
	return nil
}

// RunMigrations runs database migrations from embedded files
func (s *Storage) RunMigrations() error {
	const funcName = "RunMigrations"

	_, err := s.db.Exec(string(schema))
	if err != nil {
		slog.Error(fmt.Sprintf("failed to run migrations: %s", err.Error()), logging.FuncNameAttr(funcName))
	}
	return err
}

func (s *Storage) GetRefreshTokenByUserId(userId string) (string, error) {
	const funcName = "GetRefreshTokenByUserId"

	var refreshToken string
	row := s.db.QueryRow("SELECT refresh_token FROM auth WHERE user_id = ?", userId)
	err := row.Scan(&refreshToken)
	if errors.Is(err, sql.ErrNoRows) {
		slog.Warn(fmt.Sprintf("no entry found in database for userId %s", userId), logging.FuncNameAttr(funcName))
		return "", nil
	}
	return refreshToken, err
}

func (s *Storage) UpsertRefreshToken(userId, refreshToken string) error {
	_, err := s.db.Exec("INSERT INTO auth (user_id, refresh_token) VALUES (?, ?) "+
		"ON CONFLICT(user_id) DO UPDATE SET refresh_token = excluded.refresh_token, updated_at = datetime('now')",
		userId, refreshToken)
	return err
}
