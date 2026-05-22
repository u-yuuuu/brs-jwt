package server

import (
	"net/http"
)

func setupRoutes() {
	mux := http.NewServeMux()

	// API routes только для авторизации
	mux.HandleFunc("POST /api/auth/login", loginHandler)
	mux.HandleFunc("POST /api/auth/logout", logoutHandler)
	mux.HandleFunc("GET /api/auth/verify", verifyHandler)
	mux.HandleFunc("GET /api/auth/me", getCurrentUserHandler)
	mux.HandleFunc("GET /api/health", healthHandler)

	// API routes для преподавателя
	mux.HandleFunc("GET /api/professor/courses", professorCoursesHandler)
	mux.HandleFunc("GET /api/professor/course/students", courseStudentsHandler)
	mux.HandleFunc("GET /api/professor/course/attributes", courseAttributesHandler)
	mux.HandleFunc("POST /api/professor/update-points", updateStudentPointsHandler)

	// API routes для студента
	mux.HandleFunc("GET /api/student/grades", studentGradesHandler)
	mux.HandleFunc("GET /api/student/courses", studentCoursesHandler)

	// API route для смены пароля
	mux.HandleFunc("POST /api/user/change-password", changePasswordHandler)

	// OAuth маршруты (добавьте эти строки)
	mux.HandleFunc("GET /api/auth/github", githubAuthHandler)
	mux.HandleFunc("GET /api/auth/github/callback", githubCallbackHandler)
	mux.HandleFunc("GET /api/auth/github/status", githubStatusHandler)
	mux.HandleFunc("GET /api/auth/github/connect", githubConnectHandler)
	mux.HandleFunc("POST /api/auth/github/disconnect", githubDisconnectHandler)

	// HTML страницы
	mux.HandleFunc("GET /dashboard", dashboardHandler)
	mux.HandleFunc("GET /professor", professorPageHandler)
	mux.HandleFunc("GET /student", studentPageHandler)
	mux.HandleFunc("GET /change-password", changePasswordPageHandler) // Новая страница
	mux.HandleFunc("GET /", homePageHandler)

	// JWT API routes
	mux.HandleFunc("POST /api/jwt/login", jwtLoginHandler)
	mux.HandleFunc("GET /api/jwt/verify", jwtVerifyHandler)

	httpServer.Handler = mux
}
