package server

import (
	"brs/internal/auth"
	"brs/internal/db"
	"brs/internal/utils"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

// Добавьте конфигурацию OAuth
var (
	githubOAuthConfig *oauth2.Config
	oauthStates       = make(map[string]bool) // временное хранилище состояний
)

type GitHubUser struct {
	ID        int    `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

func InitTemplates() error {
	var err error
	templates, err = template.ParseGlob("web/templates/*.html")
	return err
}

// Хранилище сессий (временное, для демо)
var sessions = make(map[string]session)

type session struct {
	Username string
	UserType string
	UserID   int
	Expires  time.Time
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Code     string `json:"code,omitempty"`  // для OAuth
	State    string `json:"state,omitempty"` // для OAuth
}

type LoginResponse struct {
	Message string `json:"message"`
	Token   string `json:"token,omitempty"`
	User    struct {
		Username string `json:"username"`
		UserType string `json:"user_type"`
	} `json:"user,omitempty"`
}

type UserResponse struct {
	Username string `json:"username"`
	UserType string `json:"user_type"`
}

// Структура для хранения информации о курсе
type Course struct {
	CourseName  string
	SubjectName string
	GroupName   string
}

// Структура для данных dashboard
type DashboardData struct {
	Username string
	UserType string
	Courses  []Course
}

// Добавим новые структуры для управления баллами
type StudentPoints struct {
	StudentID   int     `json:"student_id"`
	StudentName string  `json:"student_name"`
	Points      float64 `json:"points"`
	MaxPoints   float64 `json:"max_points"`
}

type CourseDetails struct {
	CourseID    int    `json:"course_id"`
	CourseName  string `json:"course_name"`
	SubjectName string `json:"subject_name"`
	GroupName   string `json:"group_name"`
}

type PointsUpdateRequest struct {
	StudentID   int     `json:"student_id"`
	CourseID    int     `json:"course_id"`
	AttributeID int     `json:"attribute_id"`
	Points      float64 `json:"points"`
}

type Attribute struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	MaxPoints float64 `json:"max_points"`
}

type StudentGrade struct {
	CourseName    string  `json:"course_name"`
	SubjectName   string  `json:"subject_name"`
	AttributeName string  `json:"attribute_name"`
	Points        float64 `json:"points"`
	MaxPoints     float64 `json:"max_points"`
	Percentage    float64 `json:"percentage"`
	Grade         string  `json:"grade"`
}

type CourseGrades struct {
	CourseName  string         `json:"course_name"`
	SubjectName string         `json:"subject_name"`
	GroupName   string         `json:"group_name"`
	Grades      []StudentGrade `json:"grades"`
	TotalPoints float64        `json:"total_points"`
	TotalMax    float64        `json:"total_max"`
	Average     float64        `json:"average"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

type ChangePasswordResponse struct {
	Message string `json:"message"`
	Success bool   `json:"success"`
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if err := db.Ping(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	})
}

func InitOAuth(clientID, clientSecret, redirectURL string) {
	githubOAuthConfig = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"user:email"},
		Endpoint:     github.Endpoint,
	}
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Валидация
	if req.Username == "" || req.Password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	// Аутентификация пользователя
	user, userID, err := authenticateUser(req.Username, req.Password)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Создание сессии
	token := generateSessionToken()
	sessions[token] = session{
		Username: user.Username,
		UserType: user.UserType,
		UserID:   userID,
		Expires:  time.Now().Add(24 * time.Hour),
	}

	// Установка cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		Path:     "/",
	})

	// Ответ
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LoginResponse{
		Message: "Login successful",
		Token:   token,
		User: struct {
			Username string `json:"username"`
			UserType string `json:"user_type"`
		}{
			Username: user.Username,
			UserType: user.UserType,
		},
	})
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	// Получаем токен из cookie
	cookie, err := r.Cookie("session_token")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Already logged out",
		})
		return
	}

	// Удаляем сессию
	delete(sessions, cookie.Value)

	// Удаляем cookie
	http.SetCookie(w, &http.Cookie{
		Name:    "session_token",
		Value:   "",
		Expires: time.Now().Add(-1 * time.Hour),
		Path:    "/",
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Logout successful",
	})
}

func verifyHandler(w http.ResponseWriter, r *http.Request) {
	// Получаем токен из cookie или заголовка
	token := getTokenFromRequest(r)
	if token == "" {
		http.Error(w, "No token provided", http.StatusUnauthorized)
		return
	}

	// Проверяем сессию
	session, exists := sessions[token]
	if !exists || time.Now().After(session.Expires) {
		if !exists {
			delete(sessions, token)
		}
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message":   "Token is valid",
		"username":  session.Username,
		"user_type": session.UserType,
	})
}

func getCurrentUserHandler(w http.ResponseWriter, r *http.Request) {
	// Получаем токен из cookie или заголовка
	token := getTokenFromRequest(r)
	if token == "" {
		http.Error(w, "No token provided", http.StatusUnauthorized)
		return
	}

	// Проверяем сессию
	session, exists := sessions[token]
	if !exists || time.Now().After(session.Expires) {
		if !exists {
			delete(sessions, token)
		}
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UserResponse{
		Username: session.Username,
		UserType: session.UserType,
	})
}

func homePageHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем авторизацию
	if token := getTokenFromRequest(r); token != "" {
		if session, exists := sessions[token]; exists && time.Now().Before(session.Expires) {
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
			return
		}
	}

	// Отображаем home page
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "home.html", nil); err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func dashboardHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем авторизацию
	token := getTokenFromRequest(r)
	if token == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	session, exists := sessions[token]
	if !exists || time.Now().After(session.Expires) {
		if !exists {
			delete(sessions, token)
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Получаем курсы в зависимости от типа пользователя
	var courses []Course
	var err error

	if session.UserType == "Student" {
		courses, err = getStudentCourses(session.UserID)
	} else if session.UserType == "Professor" {
		courses, err = getProfessorCourses(session.UserID)
	}

	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Данные для dashboard
	data := DashboardData{
		Username: session.Username,
		UserType: session.UserType,
		Courses:  courses,
	}

	// Отображаем dashboard
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "dashboard.html", data); err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

// Вспомогательные функции

func authenticateUser(username, password string) (*UserResponse, int, error) {
	var userID int
	var dbPassword, userType string
	var githubID sql.NullString

	err := db.DB.QueryRow(`
        SELECT u.id, u.password, ut.name, u.github_id
        FROM users u 
        JOIN usertypes ut ON u.type_id = ut.id 
        WHERE u.username = ?
    `, username).Scan(&userID, &dbPassword, &userType, &githubID)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, 0, fmt.Errorf("user not found")
		}
		return nil, 0, err
	}

	// Проверяем пароль
	if !utils.VerifyPassword(password, dbPassword) {
		return nil, 0, fmt.Errorf("invalid password")
	}

	return &UserResponse{
		Username: username,
		UserType: userType,
	}, userID, nil
}

func generateSessionToken() string {
	return utils.SimpleHash(time.Now().String() + "session_salt")
}

func getTokenFromRequest(r *http.Request) string {
	// Пробуем получить из cookie
	if cookie, err := r.Cookie("session_token"); err == nil {
		return cookie.Value
	}

	// Пробуем получить из заголовка Authorization
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			return authHeader[7:]
		}
	}

	return ""
}

// Функция для получения курсов студента
func getStudentCourses(userID int) ([]Course, error) {
	query := `
		SELECT 
			s.name as subject_name,
			g.name as group_name
		FROM users u
		JOIN student_in_group sig ON u.id = sig.user_id
		JOIN groups g ON sig.group_id = g.id
		JOIN course_attendees ca ON g.id = ca.group_id
		JOIN courses c ON ca.course_id = c.id
		JOIN subjects s ON c.subject_id = s.id
		WHERE u.id = ?
	`

	rows, err := db.DB.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var courses []Course
	for rows.Next() {
		var course Course
		err := rows.Scan(&course.SubjectName, &course.GroupName)
		if err != nil {
			return nil, err
		}
		course.CourseName = fmt.Sprintf("Курс: %s", course.SubjectName)
		courses = append(courses, course)
	}

	return courses, nil
}

// Функция для получения курсов преподавателя
func getProfessorCourses(userID int) ([]Course, error) {
	query := `
		SELECT DISTINCT
			s.name as subject_name,
			g.name as group_name
		FROM users u
		JOIN course_admins ca ON u.id = ca.user_id
		JOIN courses c ON ca.course_id = c.id
		JOIN subjects s ON c.subject_id = s.id
		LEFT JOIN course_attendees ca2 ON c.id = ca2.course_id
		LEFT JOIN groups g ON ca2.group_id = g.id
		WHERE u.id = ?
	`

	rows, err := db.DB.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var courses []Course
	for rows.Next() {
		var course Course
		var groupName sql.NullString

		err := rows.Scan(&course.SubjectName, &groupName)
		if err != nil {
			return nil, err
		}

		course.CourseName = fmt.Sprintf("Курс: %s", course.SubjectName)
		if groupName.Valid {
			course.GroupName = groupName.String
		} else {
			course.GroupName = "Все группы"
		}
		courses = append(courses, course)
	}

	return courses, nil
}

func professorCoursesHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем авторизацию
	token := getTokenFromRequest(r)
	if token == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	session, exists := sessions[token]
	if !exists || time.Now().After(session.Expires) || session.UserType != "Professor" {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	courses, err := getProfessorCoursesWithIDs(session.UserID)
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(courses)
}

func courseStudentsHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем авторизацию
	token := getTokenFromRequest(r)
	if token == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	session, exists := sessions[token]
	if !exists || time.Now().After(session.Expires) || session.UserType != "Professor" {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	courseID := r.URL.Query().Get("course_id")
	if courseID == "" {
		http.Error(w, "Course ID is required", http.StatusBadRequest)
		return
	}

	// Проверяем, что преподаватель имеет доступ к этому курсу
	if !isProfessorCourseAdmin(session.UserID, courseID) {
		http.Error(w, "Access denied to this course", http.StatusForbidden)
		return
	}

	students, err := getCourseStudents(courseID)
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(students)
}

func courseAttributesHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем авторизацию
	token := getTokenFromRequest(r)
	if token == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	session, exists := sessions[token]
	if !exists || time.Now().After(session.Expires) || session.UserType != "Professor" {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	courseID := r.URL.Query().Get("course_id")
	if courseID == "" {
		http.Error(w, "Course ID is required", http.StatusBadRequest)
		return
	}

	// Проверяем, что преподаватель имеет доступ к этому курсу
	if !isProfessorCourseAdmin(session.UserID, courseID) {
		http.Error(w, "Access denied to this course", http.StatusForbidden)
		return
	}

	attributes, err := getCourseAttributes(courseID)
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(attributes)
}

func updateStudentPointsHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем авторизацию
	token := getTokenFromRequest(r)
	if token == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	session, exists := sessions[token]
	if !exists || time.Now().After(session.Expires) || session.UserType != "Professor" {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	var req PointsUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Проверяем, что преподаватель имеет доступ к этому курсу
	if !isProfessorCourseAdmin(session.UserID, fmt.Sprintf("%d", req.CourseID)) {
		http.Error(w, "Access denied to this course", http.StatusForbidden)
		return
	}

	err := updateStudentPoints(req)
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Points updated successfully",
	})
}

func getProfessorCoursesWithIDs(userID int) ([]CourseDetails, error) {
	query := `
        SELECT DISTINCT
            c.id as course_id,
            s.name as subject_name,
            g.name as group_name
        FROM users u
        JOIN course_admins ca ON u.id = ca.user_id
        JOIN courses c ON ca.course_id = c.id
        JOIN subjects s ON c.subject_id = s.id
        LEFT JOIN course_attendees ca2 ON c.id = ca2.course_id
        LEFT JOIN groups g ON ca2.group_id = g.id
        WHERE u.id = ?
    `

	rows, err := db.DB.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var courses []CourseDetails
	for rows.Next() {
		var course CourseDetails
		var groupName sql.NullString

		err := rows.Scan(&course.CourseID, &course.SubjectName, &groupName)
		if err != nil {
			return nil, err
		}

		course.CourseName = fmt.Sprintf("%s", course.SubjectName)
		if groupName.Valid {
			course.GroupName = groupName.String
		} else {
			course.GroupName = "Все группы"
		}
		courses = append(courses, course)
	}

	return courses, nil
}

func getCourseStudents(courseID string) ([]StudentPoints, error) {
	query := `
        SELECT 
            u.id as student_id,
            COALESCE(ui.last_name || ' ' || ui.first_name, u.username) as student_name,
            COALESCE(sp.point, 0) as points,
            COALESCE(sa.max_points, 100) as max_points
        FROM course_attendees ca
        JOIN groups g ON ca.group_id = g.id
        JOIN student_in_group sig ON g.id = sig.group_id
        JOIN users u ON sig.user_id = u.id
        LEFT JOIN users_info ui ON u.id = ui.user_id
        LEFT JOIN students_points sp ON u.id = sp.user_id AND ca.course_id = sp.course_id
        LEFT JOIN subject_attributes sa ON sp.attribute_id = sa.id
        WHERE ca.course_id = ?
        GROUP BY u.id
    `

	rows, err := db.DB.Query(query, courseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var students []StudentPoints
	for rows.Next() {
		var student StudentPoints
		err := rows.Scan(&student.StudentID, &student.StudentName, &student.Points, &student.MaxPoints)
		if err != nil {
			return nil, err
		}
		students = append(students, student)
	}

	return students, nil
}

func professorPageHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем авторизацию
	token := getTokenFromRequest(r)
	if token == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	session, exists := sessions[token]
	if !exists || time.Now().After(session.Expires) || session.UserType != "Professor" {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	// Отображаем страницу преподавателя
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "professor.html", map[string]interface{}{
		"Username": session.Username,
		"UserType": session.UserType,
	}); err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func getCourseAttributes(courseID string) ([]Attribute, error) {
	query := `
        SELECT 
            sa.id,
            sa.name,
            sa.max_points
        FROM courses c
        JOIN subject_attributes sa ON c.subject_id = sa.subject_id
        WHERE c.id = ?
    `

	rows, err := db.DB.Query(query, courseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attributes []Attribute
	for rows.Next() {
		var attr Attribute
		err := rows.Scan(&attr.ID, &attr.Name, &attr.MaxPoints)
		if err != nil {
			return nil, err
		}
		attributes = append(attributes, attr)
	}

	return attributes, nil
}

func isProfessorCourseAdmin(userID int, courseID string) bool {
	var count int
	err := db.DB.QueryRow(`
        SELECT COUNT(*) 
        FROM course_admins 
        WHERE user_id = ? AND course_id = ?
    `, userID, courseID).Scan(&count)

	return err == nil && count > 0
}

func updateStudentPoints(req PointsUpdateRequest) error {
	// Сначала проверяем, существует ли запись
	var exists bool
	err := db.DB.QueryRow(`
        SELECT EXISTS(
            SELECT 1 FROM students_points 
            WHERE user_id = ? AND course_id = ? AND attribute_id = ?
        )
    `, req.StudentID, req.CourseID, req.AttributeID).Scan(&exists)

	if err != nil {
		return err
	}

	if exists {
		// Обновляем существующую запись
		_, err = db.DB.Exec(`
            UPDATE students_points 
            SET point = ? 
            WHERE user_id = ? AND course_id = ? AND attribute_id = ?
        `, req.Points, req.StudentID, req.CourseID, req.AttributeID)
	} else {
		// Создаем новую запись
		_, err = db.DB.Exec(`
            INSERT INTO students_points (user_id, course_id, attribute_id, point)
            VALUES (?, ?, ?, ?)
        `, req.StudentID, req.CourseID, req.AttributeID, req.Points)
	}

	return err
}

func studentGradesHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем авторизацию
	token := getTokenFromRequest(r)
	if token == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	session, exists := sessions[token]
	if !exists || time.Now().After(session.Expires) || session.UserType != "Student" {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	grades, err := getStudentGrades(session.UserID)
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(grades)
}

func studentCoursesHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем авторизацию
	token := getTokenFromRequest(r)
	if token == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	session, exists := sessions[token]
	if !exists || time.Now().After(session.Expires) || session.UserType != "Student" {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	courses, err := getStudentCoursesWithDetails(session.UserID)
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(courses)
}

func getStudentGrades(userID int) ([]CourseGrades, error) {
	query := `
        SELECT 
            s.name as subject_name,
            g.name as group_name,
            sa.name as attribute_name,
            COALESCE(sp.point, 0) as points,
            COALESCE(sa.max_points, 100) as max_points,
            c.id as course_id
        FROM users u
        JOIN student_in_group sig ON u.id = sig.user_id
        JOIN groups g ON sig.group_id = g.id
        JOIN course_attendees ca ON g.id = ca.group_id
        JOIN courses c ON ca.course_id = c.id
        JOIN subjects s ON c.subject_id = s.id
        LEFT JOIN subject_attributes sa ON s.id = sa.subject_id
        LEFT JOIN students_points sp ON u.id = sp.user_id AND c.id = sp.course_id AND sa.id = sp.attribute_id
        WHERE u.id = ?
        ORDER BY s.name, sa.name
    `

	rows, err := db.DB.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Группируем оценки по курсам
	coursesMap := make(map[string]*CourseGrades)

	for rows.Next() {
		var subjectName, groupName string
		var courseID int
		var attributeName sql.NullString
		var points, maxPoints sql.NullFloat64

		err := rows.Scan(&subjectName, &groupName, &attributeName, &points, &maxPoints, &courseID)
		if err != nil {
			return nil, err
		}

		courseKey := fmt.Sprintf("%d", courseID)

		// Создаем запись курса если ее нет
		if _, exists := coursesMap[courseKey]; !exists {
			coursesMap[courseKey] = &CourseGrades{
				CourseName:  fmt.Sprintf("Курс: %s", subjectName),
				SubjectName: subjectName,
				GroupName:   groupName,
				Grades:      []StudentGrade{},
				TotalPoints: 0,
				TotalMax:    0,
				Average:     0,
			}
		}

		// Добавляем оценку если есть атрибут
		if attributeName.Valid {
			pointsValue := 0.0
			if points.Valid {
				pointsValue = points.Float64
			}

			maxPointsValue := 100.0
			if maxPoints.Valid {
				maxPointsValue = maxPoints.Float64
			}

			percentage := 0.0
			if maxPointsValue > 0 {
				percentage = (pointsValue / maxPointsValue) * 100
			}

			grade := calculateGrade(percentage)

			courseGrade := StudentGrade{
				CourseName:    coursesMap[courseKey].CourseName,
				SubjectName:   subjectName,
				AttributeName: attributeName.String,
				Points:        pointsValue,
				MaxPoints:     maxPointsValue,
				Percentage:    percentage,
				Grade:         grade,
			}

			coursesMap[courseKey].Grades = append(coursesMap[courseKey].Grades, courseGrade)

			// Обновляем общие баллы
			coursesMap[courseKey].TotalPoints += pointsValue
			coursesMap[courseKey].TotalMax += maxPointsValue
		}
	}

	// Рассчитываем средние значения и преобразуем в slice
	var courses []CourseGrades
	for _, course := range coursesMap {
		if course.TotalMax > 0 {
			course.Average = (course.TotalPoints / course.TotalMax) * 100
		} else if len(course.Grades) > 0 {
			// Если есть оценки, но TotalMax = 0, пересчитываем
			totalPoints := 0.0
			totalMax := 0.0
			for _, grade := range course.Grades {
				totalPoints += grade.Points
				totalMax += grade.MaxPoints
			}
			if totalMax > 0 {
				course.Average = (totalPoints / totalMax) * 100
				course.TotalPoints = totalPoints
				course.TotalMax = totalMax
			}
		}
		courses = append(courses, *course)
	}

	// Если нет курсов с оценками, возвращаем пустой массив
	if courses == nil {
		courses = []CourseGrades{}
	}

	return courses, nil
}

func getStudentCoursesWithDetails(userID int) ([]CourseDetails, error) {
	query := `
        SELECT DISTINCT
            c.id as course_id,
            s.name as subject_name,
            g.name as group_name
        FROM users u
        JOIN student_in_group sig ON u.id = sig.user_id
        JOIN groups g ON sig.group_id = g.id
        JOIN course_attendees ca ON g.id = ca.group_id
        JOIN courses c ON ca.course_id = c.id
        JOIN subjects s ON c.subject_id = s.id
        WHERE u.id = ?
    `

	rows, err := db.DB.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var courses []CourseDetails
	for rows.Next() {
		var course CourseDetails
		err := rows.Scan(&course.CourseID, &course.SubjectName, &course.GroupName)
		if err != nil {
			return nil, err
		}
		course.CourseName = fmt.Sprintf("Курс: %s", course.SubjectName)
		courses = append(courses, course)
	}

	return courses, nil
}

func calculateGrade(percentage float64) string {
	switch {
	case percentage >= 90:
		return "A (Отлично)"
	case percentage >= 80:
		return "B (Хорошо)"
	case percentage >= 70:
		return "C (Удовлетворительно)"
	case percentage >= 60:
		return "D (Достаточно)"
	default:
		return "F (Неудовлетворительно)"
	}
}

func studentPageHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем авторизацию
	token := getTokenFromRequest(r)
	if token == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	session, exists := sessions[token]
	if !exists || time.Now().After(session.Expires) || session.UserType != "Student" {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	// Отображаем страницу студента
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "student.html", map[string]interface{}{
		"Username": session.Username,
		"UserType": session.UserType,
	}); err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

// Обработчик смены пароля
func changePasswordHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем авторизацию
	token := getTokenFromRequest(r)
	if token == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	session, exists := sessions[token]
	if !exists || time.Now().After(session.Expires) {
		http.Error(w, "Invalid or expired session", http.StatusUnauthorized)
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Валидация
	if req.CurrentPassword == "" || req.NewPassword == "" || req.ConfirmPassword == "" {
		http.Error(w, "All password fields are required", http.StatusBadRequest)
		return
	}

	if req.NewPassword != req.ConfirmPassword {
		http.Error(w, "New passwords do not match", http.StatusBadRequest)
		return
	}

	if len(req.NewPassword) < 6 {
		http.Error(w, "New password must be at least 6 characters long", http.StatusBadRequest)
		return
	}

	// Проверяем текущий пароль
	if !verifyCurrentPassword(session.UserID, req.CurrentPassword) {
		http.Error(w, "Current password is incorrect", http.StatusUnauthorized)
		return
	}

	// Обновляем пароль
	if err := updatePassword(session.UserID, req.NewPassword); err != nil {
		http.Error(w, "Failed to update password: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ChangePasswordResponse{
		Message: "Password updated successfully",
		Success: true,
	})
}

// Вспомогательные функции для работы с паролями
func verifyCurrentPassword(userID int, currentPassword string) bool {
	var dbPassword string
	err := db.DB.QueryRow("SELECT password FROM users WHERE id = ?", userID).Scan(&dbPassword)
	if err != nil {
		return false
	}
	return utils.VerifyPassword(currentPassword, dbPassword)
}

func updatePassword(userID int, newPassword string) error {
	var err error
	hashedPassword := utils.SimpleHash(newPassword)
	_, err = db.DB.Exec("UPDATE users SET password = ? WHERE id = ?", hashedPassword, userID)
	return err
}

func changePasswordPageHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем авторизацию
	token := getTokenFromRequest(r)
	if token == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	session, exists := sessions[token]
	if !exists || time.Now().After(session.Expires) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Отображаем страницу смены пароля
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "change-password.html", map[string]interface{}{
		"Username": session.Username,
		"UserType": session.UserType,
	}); err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

// Генерация случайного состояния для OAuth
func generateState() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// Обработчик для начала OAuth потока
func githubAuthHandler(w http.ResponseWriter, r *http.Request) {
	state, err := generateState()
	if err != nil {
		log.Printf("Failed to generate state: %v", err)
		http.Error(w, "Failed to generate state", http.StatusInternalServerError)
		return
	}

	// Сохраняем состояние в базе (используем правильные имена колонок)
	_, err = db.DB.Exec("INSERT INTO oauth_states (state) VALUES (?)", state)
	if err != nil {
		log.Printf("Failed to save state to database: %v", err)
		// Выведем дополнительную информацию об ошибке
		log.Printf("Table structure: id, state, created_at, used")
		http.Error(w, "Failed to save state: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Генерируем URL для перенаправления на GitHub
	url := githubOAuthConfig.AuthCodeURL(state)

	log.Printf("Generated OAuth URL for state: %s", state)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"auth_url": url,
	})
}

func githubStatusHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем авторизацию
	token := getTokenFromRequest(r)
	if token == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	session, exists := sessions[token]
	if !exists || time.Now().After(session.Expires) {
		http.Error(w, "Invalid or expired session", http.StatusUnauthorized)
		return
	}

	// Получаем информацию о GitHub привязке
	var githubID sql.NullString
	// var githubLogin sql.NullString

	// Если есть колонка github_id в таблице users
	err := db.DB.QueryRow(`
        SELECT github_id FROM users WHERE id = ?
    `, session.UserID).Scan(&githubID)

	if err != nil {
		// Если колонки нет или ошибка
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"github_id":    nil,
			"github_login": nil,
			"connected":    false,
		})
		return
	}

	response := map[string]interface{}{
		"github_id": nil,
		"connected": false,
	}

	if githubID.Valid && githubID.String != "" {
		response["github_id"] = githubID.String
		response["connected"] = true

		// Можно попробовать получить логин из GitHub API если нужно
		// Но для простоты показываем только ID
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Начать процесс привязки GitHub
func githubConnectHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем авторизацию
	token := getTokenFromRequest(r)
	if token == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	session, exists := sessions[token]
	if !exists || time.Now().After(session.Expires) {
		http.Error(w, "Invalid or expired session", http.StatusUnauthorized)
		return
	}

	// Генерируем состояние и сохраняем сессию пользователя
	state, err := generateState()
	if err != nil {
		http.Error(w, "Failed to generate state", http.StatusInternalServerError)
		return
	}

	// Сохраняем состояние в базе с привязкой к пользователю
	_, err = db.DB.Exec(`
        INSERT INTO oauth_states (state, user_id) 
        VALUES (?, ?)
    `, state, session.UserID)
	if err != nil {
		log.Printf("Failed to save state: %v", err)
		http.Error(w, "Failed to save state", http.StatusInternalServerError)
		return
	}

	// Генерируем URL для привязки (добавляем специальный параметр)
	url := githubOAuthConfig.AuthCodeURL(state)
	url += "&allow_signup=false" // Не предлагать регистрацию на GitHub

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"auth_url": url,
	})
}

// Отвязать GitHub
func githubDisconnectHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем авторизацию
	token := getTokenFromRequest(r)
	if token == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	session, exists := sessions[token]
	if !exists || time.Now().After(session.Expires) {
		http.Error(w, "Invalid or expired session", http.StatusUnauthorized)
		return
	}

	// Обновляем запись пользователя, удаляя GitHub ID
	_, err := db.DB.Exec(`
		UPDATE users SET github_id = NULL WHERE id = ?
	`, session.UserID)
	if err != nil {
		log.Printf("Failed to disconnect GitHub: %v", err)
		http.Error(w, "Failed to disconnect GitHub", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "GitHub аккаунт успешно отвязан",
	})
}

func githubCallbackHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("GitHub callback received")

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	log.Printf("Code: %s, State: %s", code[:10]+"...", state[:10]+"...")

	if code == "" || state == "" {
		log.Println("Missing code or state")
		http.Error(w, "Missing code or state", http.StatusBadRequest)
		return
	}

	// Проверяем состояние и получаем user_id если есть
	var exists bool
	var userIDFromState int
	err := db.DB.QueryRow(`
        SELECT 1, COALESCE(user_id, 0) FROM oauth_states 
        WHERE state = ? AND used = 0 AND 
        created_at > datetime('now', '-5 minutes')
    `, state).Scan(&exists, &userIDFromState)

	if err != nil || !exists {
		log.Printf("Invalid or expired state: %v, exists: %v", err, exists)
		http.Error(w, "Invalid or expired state", http.StatusBadRequest)
		return
	}

	// Помечаем состояние как использованное
	_, err = db.DB.Exec("UPDATE oauth_states SET used = 1 WHERE state = ?", state)
	if err != nil {
		log.Printf("Failed to mark state as used: %v", err)
	}

	// Обмениваем код на токен
	log.Println("Exchanging code for token...")
	token, err := githubOAuthConfig.Exchange(context.Background(), code)
	if err != nil {
		log.Printf("Failed to exchange token: %v", err)
		http.Error(w, "Failed to exchange token", http.StatusInternalServerError)
		return
	}

	// Получаем данные пользователя из GitHub
	client := githubOAuthConfig.Client(context.Background(), token)

	log.Println("Fetching GitHub user info...")
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		log.Printf("Failed to get user info: %v", err)
		http.Error(w, "Failed to get user info", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read response: %v", err)
		http.Error(w, "Failed to read response", http.StatusInternalServerError)
		return
	}

	var githubUser GitHubUser
	if err := json.Unmarshal(body, &githubUser); err != nil {
		log.Printf("Failed to parse user info: %v", err)
		http.Error(w, "Failed to parse user info", http.StatusInternalServerError)
		return
	}

	log.Printf("GitHub user: %s (ID: %d), State userID: %d",
		githubUser.Login, githubUser.ID, userIDFromState)

	// Определяем, как обрабатывать запрос
	if userIDFromState > 0 {
		// Это запрос на привязку существующего аккаунта
		log.Printf("Binding GitHub to existing user ID: %d", userIDFromState)

		// Проверяем, не привязан ли этот GitHub аккаунт к другому пользователю
		var existingUserID int
		err = db.DB.QueryRow(`
			SELECT id FROM users WHERE github_id = ?
		`, fmt.Sprintf("%d", githubUser.ID)).Scan(&existingUserID)

		if err == nil && existingUserID > 0 && existingUserID != userIDFromState {
			// GitHub аккаунт уже привязан к другому пользователю
			log.Printf("GitHub account already linked to user ID: %d", existingUserID)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusConflict)
			fmt.Fprintf(w, `
				<html>
					<body>
						<h1>GitHub аккаунт уже привязан</h1>
						<p>GitHub аккаунт %s уже привязан к другому пользователю в системе.</p>
						<p>Пожалуйста, отвяжите его от другого аккаунта или используйте другой GitHub аккаунт.</p>
						<a href="/dashboard">Вернуться в личный кабинет</a>
					</body>
				</html>
			`, githubUser.Login)
			return
		}

		// Обновляем GitHub ID для текущего пользователя
		_, err = db.DB.Exec(`
			UPDATE users SET github_id = ? WHERE id = ?
		`, fmt.Sprintf("%d", githubUser.ID), userIDFromState)

		if err != nil {
			log.Printf("Failed to update GitHub ID: %v", err)
			http.Error(w, "Failed to link GitHub account", http.StatusInternalServerError)
			return
		}

		log.Printf("Successfully linked GitHub to user ID: %d", userIDFromState)

		// Перенаправляем на страницу настроек или профиля
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	// Это обычный вход через GitHub
	log.Printf("GitHub login: %s (ID: %d)", githubUser.Login, githubUser.ID)

	// Пытаемся найти пользователя по GitHub ID
	var userID int
	var username, userType string
	var dbPassword sql.NullString

	err = db.DB.QueryRow(`
		SELECT u.id, u.username, u.password, ut.name 
		FROM users u 
		JOIN usertypes ut ON u.type_id = ut.id 
		WHERE u.github_id = ?
	`, fmt.Sprintf("%d", githubUser.ID)).Scan(
		&userID, &username, &dbPassword, &userType,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("No user found with GitHub ID: %d", githubUser.ID)

			// Если пользователь не найден, показываем сообщение о необходимости привязки
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, `
				<html>
					<body>
						<h1>GitHub аккаунт не привязан</h1>
						<p>GitHub аккаунт %s не привязан к пользователю в системе.</p>
						<p>Пожалуйста, сначала войдите через обычную форму, затем привяжите GitHub в настройках профиля.</p>
						<a href="/">Войти в систему</a>
					</body>
				</html>
			`, githubUser.Login)
			return
		} else {
			log.Printf("Database error: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
	}

	// Создаем сессию
	sessionToken := generateSessionToken()
	sessions[sessionToken] = session{
		Username: username,
		UserType: userType,
		UserID:   userID,
		Expires:  time.Now().Add(24 * time.Hour),
	}

	log.Printf("Created session for user %s, user type: %s", username, userType)

	// Устанавливаем cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    sessionToken,
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	})

	log.Printf("User %s logged in via GitHub, redirecting to dashboard", username)

	// Перенаправляем на dashboard
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func jwtLoginHandler(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Аутентификация существующего пользователя
	user, userID, err := authenticateUser(req.Username, req.Password)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Генерация JWT вместо сессии
	token, err := auth.GenerateJWT(userID, user.Username, user.UserType)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":   token,
		"user":    user,
		"message": "JWT login successful",
	})
}

func jwtVerifyHandler(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "No token provided", http.StatusUnauthorized)
		return
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		http.Error(w, "Invalid token format", http.StatusUnauthorized)
		return
	}

	claims, err := auth.ValidateJWT(parts[1])
	if err != nil {
		http.Error(w, "Invalid token: "+err.Error(), http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"valid":      true,
		"user_id":    claims.UserID,
		"username":   claims.Username,
		"user_type":  claims.UserType,
		"expires_at": claims.ExpiresAt,
	})
}
