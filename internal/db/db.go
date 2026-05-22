package db

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type Config struct {
	DBPath string
}

var DB *sql.DB
var config Config

//go:embed queries/*.sql
var sqlFiles embed.FS

func Init(cfg Config) error {
	var err error

	config.DBPath = cfg.DBPath

	// Открываем соединение с базой данных
	DB, err = sql.Open("sqlite3", config.DBPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Проверяем соединение
	if err = DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Устанавливаем практичные настройки для SQLite
	if _, err = DB.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	if _, err = DB.Exec("PRAGMA journal_mode = WAL;"); err != nil {
		return fmt.Errorf("failed to set journal mode: %w", err)
	}

	if _, err = DB.Exec("PRAGMA busy_timeout = 5000;"); err != nil {
		return fmt.Errorf("failed to set busy timeout: %w", err)
	}

	// Создаем таблицы
	if err = createTables(); err != nil {
		return err
	}

	if err = createUsertypes(); err != nil {
		return err
	}

	log.Printf("Database initialized successfully: %s", config.DBPath)
	return nil
}

func Close() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}

func Ping() error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}
	return DB.Ping()
}

func AddUser(usertype string, name string, password string) error {
	var err error

	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	// Валидация входных данных
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("username cannot be empty")
	}

	if strings.TrimSpace(password) == "" {
		return fmt.Errorf("password cannot be empty")
	}

	// Проверяем существование типа пользователя
	var typeID int
	err = DB.QueryRow("SELECT id FROM usertypes WHERE name = ?", usertype).Scan(&typeID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("invalid user type: %s", usertype)
		}
		return fmt.Errorf("failed to check user type: %w", err)
	}

	// Выполняем вставку
	if err = executeSQLFileWithArgs("queries/adduser.sql", name, password, usertype); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

func createTables() error {
	var err error

	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	if err = executeSQLFile("queries/createtables.sql"); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	return nil
}

func createUsertypes() error {
	var err error

	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	if err = executeSQLFile("queries/createusertypes.sql"); err != nil {
		return fmt.Errorf("failed to create usertypes: %w", err)
	}

	return nil
}

func executeSQLFile(filePath string) error {
	// Убираем префикс пути, если он есть
	content, err := sqlFiles.ReadFile(strings.TrimPrefix(filePath, "./"))
	if err != nil {
		return err
	}

	queries := strings.Split(string(content), ";")

	for _, query := range queries {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}

		_, err := DB.Exec(query)
		if err != nil {
			return fmt.Errorf("query execution fail: %v\nquery: %s", err, query)
		}
	}

	return nil
}

func executeSQLFileWithArgs(filePath string, args ...interface{}) error {
	// Убираем префикс пути, если он есть
	content, err := sqlFiles.ReadFile(strings.TrimPrefix(filePath, "./"))
	if err != nil {
		return err
	}

	queries := strings.Split(string(content), ";")

	for _, query := range queries {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}

		_, err := DB.Exec(query, args...)
		if err != nil {
			return fmt.Errorf("query execution fail: %v\nquery: %s", err, query)
		}
	}

	return nil
}
