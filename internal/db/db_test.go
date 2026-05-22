package db

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// Настройка перед всеми тестами
	code := m.Run()
	// Очистка после всех тестов
	os.Exit(code)
}

func setupTestDB(t *testing.T) string {
	t.Helper()

	// Создаем временный файл БД
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	return dbPath
}

func cleanupTestDB() {
	if DB != nil {
		DB.Close()
		DB = nil
	}
}

func TestInit(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name:    "successful initialization",
			config:  Config{DBPath: setupTestDB(t)},
			wantErr: false,
		},
		{
			name:    "invalid db path",
			config:  Config{DBPath: "/invalid/path/test.db"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer cleanupTestDB()

			err := Init(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Init() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && DB == nil {
				t.Error("Init() DB should not be nil on success")
			}
		})
	}
}

func TestCreateTables(t *testing.T) {
	dbPath := setupTestDB(t)
	defer cleanupTestDB()

	// Инициализируем базу данных
	cfg := Config{DBPath: dbPath}
	if err := Init(cfg); err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}

	// Проверяем, что все таблицы создались
	tables := []string{
		"usertypes",
		"users",
		"subjects",
		"courses",
		"groups",
		"student_in_group",
		"course_admins",
		"subject_attributes",
		"students_points",
		"users_info",
	}

	for _, table := range tables {
		t.Run("check table "+table, func(t *testing.T) {
			query := "SELECT name FROM sqlite_master WHERE type='table' AND name=?"
			var name string
			err := DB.QueryRow(query, table).Scan(&name)
			if err != nil {
				t.Errorf("Table %s not found: %v", table, err)
			}
			if name != table {
				t.Errorf("Expected table %s, got %s", table, name)
			}
		})
	}
}

func TestCreateUsertypes(t *testing.T) {
	dbPath := setupTestDB(t)
	defer cleanupTestDB()

	// Инициализируем базу данных
	cfg := Config{DBPath: dbPath}
	if err := Init(cfg); err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}

	rows := []string{
		"Student",
		"Professor",
	}

	for _, row := range rows {
		t.Run("check row "+row, func(t *testing.T) {
			query := "SELECT name FROM usertypes WHERE name=?"
			var name string
			err := DB.QueryRow(query, row).Scan(&name)
			if err != nil {
				t.Errorf("Row with %s not found: %v", row, err)
			}
		})
	}
}

func TestTableStructure(t *testing.T) {
	dbPath := setupTestDB(t)
	defer cleanupTestDB()

	cfg := Config{DBPath: dbPath}
	if err := Init(cfg); err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}

	// Тест внешних ключей
	t.Run("check foreign keys", func(t *testing.T) {
		var foreignKeysEnabled bool
		err := DB.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeysEnabled)
		if err != nil {
			t.Fatalf("Failed to check foreign keys: %v", err)
		}
		if !foreignKeysEnabled {
			t.Error("Foreign keys are not enabled")
		}
	})

	// Тест конкретных связей между таблицами
	t.Run("check users foreign key", func(t *testing.T) {
		// Проверяем, что type_id в users ссылается на usertypes(id)
		var fkInfo struct {
			ID       int
			Seq      int
			Table    string
			From     string
			To       string
			OnUpdate string
			OnDelete string
			Match    string
		}

		rows, err := DB.Query("PRAGMA foreign_key_list(users)")
		if err != nil {
			t.Fatalf("Failed to get foreign key list: %v", err)
		}
		defer rows.Close()

		found := false
		for rows.Next() {
			err := rows.Scan(&fkInfo.ID, &fkInfo.Seq, &fkInfo.Table, &fkInfo.From, &fkInfo.To, &fkInfo.OnUpdate, &fkInfo.OnDelete, &fkInfo.Match)
			if err != nil {
				t.Fatalf("Failed to scan foreign key info: %v", err)
			}
			if fkInfo.Table == "usertypes" && fkInfo.From == "type_id" && fkInfo.To == "id" {
				found = true
				break
			}
		}

		if !found {
			t.Error("Foreign key from users.type_id to usertypes.id not found")
		}
	})
}

func TestInsertSampleData(t *testing.T) {
	dbPath := setupTestDB(t)
	defer cleanupTestDB()

	cfg := Config{DBPath: dbPath}
	if err := Init(cfg); err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}

	// Тест вставки данных и проверки связей
	t.Run("insert and verify relationships", func(t *testing.T) {
		// Вставляем тестовые данные
		_, err := DB.Exec(`
			INSERT INTO users (type_id, username, password) 
			VALUES ((SELECT id FROM usertypes WHERE name='Student'), 'testuser', 'testpass');
			
			INSERT INTO subjects (name) VALUES ('math');
			
			INSERT INTO courses (subject_id) VALUES (1);
		`)
		if err != nil {
			t.Fatalf("Failed to insert test data: %v", err)
		}

		// Проверяем, что данные корректно вставились
		var username string
		err = DB.QueryRow("SELECT username FROM users WHERE id = 1").Scan(&username)
		if err != nil {
			t.Fatalf("Failed to query user: %v", err)
		}
		if username != "testuser" {
			t.Errorf("Expected username 'testuser', got '%s'", username)
		}

		// Проверяем связь пользователя с типом
		var userType string
		err = DB.QueryRow(`
			SELECT ut.name 
			FROM users u 
			JOIN usertypes ut ON u.type_id = ut.id 
			WHERE u.id = 1
		`).Scan(&userType)
		if err != nil {
			t.Fatalf("Failed to query user type: %v", err)
		}
		if userType != "Student" {
			t.Errorf("Expected user type 'student', got '%s'", userType)
		}
	})
}

func TestAddUser(t *testing.T) {
	dbPath := setupTestDB(t)
	defer cleanupTestDB()

	cfg := Config{DBPath: dbPath}
	if err := Init(cfg); err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}

	tests := []struct {
		name        string
		userType    string
		username    string
		password    string
		wantErr     bool
		errorSubstr string
	}{
		{
			name:     "successful student creation",
			userType: "Student",
			username: "student1",
			password: "pass123",
			wantErr:  false,
		},
		{
			name:     "successful professor creation",
			userType: "Professor",
			username: "prof1",
			password: "profpass",
			wantErr:  false,
		},
		{
			name:        "duplicate username",
			userType:    "Student",
			username:    "student1",
			password:    "pass456",
			wantErr:     true,
			errorSubstr: "UNIQUE constraint",
		},
		{
			name:        "invalid user type",
			userType:    "InvalidType",
			username:    "user2",
			password:    "pass123",
			wantErr:     true,
			errorSubstr: "invalid user type",
		},
		{
			name:        "empty username",
			userType:    "Student",
			username:    "",
			password:    "pass123",
			wantErr:     true,
			errorSubstr: "username cannot be empty",
		},
		{
			name:        "empty password",
			userType:    "Student",
			username:    "user3",
			password:    "",
			wantErr:     true,
			errorSubstr: "password cannot be empty",
		},
		{
			name:        "whitespace username",
			userType:    "Student",
			username:    "   ",
			password:    "pass123",
			wantErr:     true,
			errorSubstr: "username cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := AddUser(tt.userType, tt.username, tt.password)

			if (err != nil) != tt.wantErr {
				t.Errorf("AddUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Проверяем текст ошибки если нужно
			if tt.wantErr && tt.errorSubstr != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errorSubstr) {
					t.Errorf("AddUser() error = %v, should contain %s", err, tt.errorSubstr)
				}
			}

			// Если ошибки не ожидалось, проверяем что пользователь создался
			if !tt.wantErr && err == nil {
				verifyUserExists(t, tt.username, tt.userType)
			}
		})
	}
}

// Вспомогательная функция для проверки существования пользователя
func verifyUserExists(t *testing.T, username, expectedUserType string) {
	t.Helper()

	var dbUsername, dbUserType string
	query := `
		SELECT u.username, ut.name 
		FROM users u 
		JOIN usertypes ut ON u.type_id = ut.id 
		WHERE u.username = ?`

	err := DB.QueryRow(query, username).Scan(&dbUsername, &dbUserType)
	if err != nil {
		t.Fatalf("User verification failed: %v", err)
	}

	if dbUsername != username {
		t.Errorf("Expected username %s, got %s", username, dbUsername)
	}
	if dbUserType != expectedUserType {
		t.Errorf("Expected user type %s, got %s", expectedUserType, dbUserType)
	}
}

func TestAddUserWithoutDB(t *testing.T) {
	// Сохраняем текущее состояние DB
	originalDB := DB
	defer func() {
		DB = originalDB // восстанавливаем
	}()

	// Устанавливаем DB в nil для теста
	DB = nil

	err := AddUser("Student", "testuser", "testpass")
	if err == nil {
		t.Error("AddUser() should return error when DB is not initialized")
	}
	if !strings.Contains(err.Error(), "database not initialized") {
		t.Errorf("Expected 'database not initialized' error, got: %v", err)
	}
}

func TestAddUserConcurrent(t *testing.T) {
	dbPath := setupTestDB(t)
	defer cleanupTestDB()

	cfg := Config{DBPath: dbPath}
	if err := Init(cfg); err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}

	// Тест на конкурентное добавление пользователей
	const numUsers = 10
	errors := make(chan error, numUsers)

	for i := 0; i < numUsers; i++ {
		go func(id int) {
			username := fmt.Sprintf("concurrent_user_%d", id)
			err := AddUser("Student", username, "password")
			errors <- err
		}(i)
	}

	// Собираем результаты
	successCount := 0
	for i := 0; i < numUsers; i++ {
		err := <-errors
		if err == nil {
			successCount++
		}
	}

	if successCount != numUsers {
		t.Errorf("Expected %d successful inserts, got %d", numUsers, successCount)
	}

	// Проверяем что все пользователи добавлены
	var userCount int
	err := DB.QueryRow("SELECT COUNT(*) FROM users WHERE username LIKE 'concurrent_user_%'").Scan(&userCount)
	if err != nil {
		t.Fatalf("Failed to count users: %v", err)
	}

	if userCount != numUsers {
		t.Errorf("Expected %d users in database, got %d", numUsers, userCount)
	}
}

func TestConstraints(t *testing.T) {
	dbPath := setupTestDB(t)
	defer cleanupTestDB()

	cfg := Config{DBPath: dbPath}
	if err := Init(cfg); err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}

	t.Run("test unique constraint", func(t *testing.T) {
		// Сначала создаем запись в usertypes и получаем её ID
		result, err := DB.Exec("INSERT INTO usertypes (name) VALUES ('student')")
		if err != nil {
			t.Fatalf("Failed to insert user type: %v", err)
		}

		typeID, err := result.LastInsertId()
		if err != nil {
			t.Fatalf("Failed to get last insert ID: %v", err)
		}

		// Вставляем первого пользователя
		_, err = DB.Exec("INSERT INTO users (type_id, username, password) VALUES (?, 'uniqueuser', 'pass')", typeID)
		if err != nil {
			t.Fatalf("Failed to insert first user: %v", err)
		}

		// Пытаемся вставить пользователя с таким же username (должно быть ошибка)
		_, err = DB.Exec("INSERT INTO users (type_id, username, password) VALUES (?, 'uniqueuser', 'pass2')", typeID)
		if err == nil {
			t.Error("Expected unique constraint violation for duplicate username")
		} else {
			// Проверяем, что ошибка именно о нарушении уникальности
			if !strings.Contains(err.Error(), "UNIQUE constraint failed") {
				t.Errorf("Expected UNIQUE constraint error, got: %v", err)
			}
		}
	})

	t.Run("test foreign key constraint", func(t *testing.T) {
		// Пытаемся вставить пользователя с несуществующим type_id
		_, err := DB.Exec("INSERT INTO users (type_id, username, password) VALUES (999, 'test', 'pass')")
		if err == nil {
			t.Error("Expected foreign key constraint violation for non-existent type_id")
		} else {
			// Проверяем, что ошибка именно о внешнем ключе
			if !strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
				t.Errorf("Expected FOREIGN KEY constraint error, got: %v", err)
			}
		}
	})
}

func TestExecuteSQLFile(t *testing.T) {
	dbPath := setupTestDB(t)
	defer cleanupTestDB()

	cfg := Config{DBPath: dbPath}
	if err := Init(cfg); err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}

	tests := []struct {
		name     string
		filePath string
		wantErr  bool
	}{
		{
			name:     "valid sql file",
			filePath: "queries/createtables.sql",
			wantErr:  false,
		},
		{
			name:     "non-existent file",
			filePath: "queries/nonexistent.sql",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := executeSQLFile(tt.filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("executeSQLFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClose(t *testing.T) {
	dbPath := setupTestDB(t)

	cfg := Config{DBPath: dbPath}
	if err := Init(cfg); err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}

	// Проверяем, что соединение открыто
	if DB == nil {
		t.Fatal("DB should be initialized")
	}

	// Закрываем соединение
	if err := Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Проверяем, что соединение закрыто
	if err := DB.Ping(); err == nil {
		t.Error("Ping() should fail after Close()")
	}
}
