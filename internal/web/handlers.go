package web

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"todo/internal/domain"
	"todo/internal/repository"
	"todo/internal/service"

	"github.com/go-chi/chi/v5"
)

type contextKey string

const currentUserContextKey contextKey = "current-user"

type HandlerOptions struct {
	TemplateDir       string
	StaticDir         string
	MaxUploadSize     int64
	Location          *time.Location
	SessionCookieName string
	SessionSecure     bool
	AllowRegistration bool
}

type Handler struct {
	taskService       *service.TaskService
	authService       *service.AuthService
	templates         *template.Template
	staticDir         string
	maxUploadSize     int64
	location          *time.Location
	sessionCookieName string
	sessionSecure     bool
	allowRegistration bool
}

type UserView struct {
	DisplayName string
	Username    string
	IsAdmin     bool
}

type DashboardPageData struct {
	CurrentUser          *UserView
	Error                string
	FocusTitle           string
	FocusDateISO         string
	FocusYear            string
	FocusMonth           string
	FocusDay             string
	FocusTasks           []TaskCard
	YesterdayPath        string
	TodayPath            string
	TomorrowPath         string
	DayAfterTomorrowPath string
}

type AccountPageData struct {
	CurrentUser *UserView
}

type AdminPageData struct {
	CurrentUser  *UserView
	Message      string
	Error        string
	PendingUsers []PendingUserCard
}

type AuthPageData struct {
	Title            string
	Heading          string
	Message          string
	Error            string
	Action           string
	SubmitLabel      string
	AlternativeLabel string
	AlternativePath  string
	ShowDisplayName  bool
	UsernameValue    string
	DisplayNameValue string
}

type TaskCard struct {
	ID            string
	Title         string
	KindLabel     string
	KindClass     string
	StatusLine    string
	Note          string
	CanComplete   bool
	CanPostpone   bool
	PostponeValue string
	ReturnDate    string
}

type PendingUserCard struct {
	ID          string
	DisplayName string
	Username    string
	CreatedAt   string
}

func NewHandler(taskService *service.TaskService, authService *service.AuthService, options HandlerOptions) (*Handler, error) {
	templates, err := template.ParseGlob(filepath.Join(options.TemplateDir, "*.html"))
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	return &Handler{
		taskService:       taskService,
		authService:       authService,
		templates:         templates,
		staticDir:         options.StaticDir,
		maxUploadSize:     options.MaxUploadSize,
		location:          options.Location,
		sessionCookieName: options.SessionCookieName,
		sessionSecure:     options.SessionSecure,
		allowRegistration: options.AllowRegistration,
	}, nil
}

func (h *Handler) Router() http.Handler {
	router := chi.NewRouter()

	fileServer := http.FileServer(http.Dir(h.staticDir))
	router.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	router.Get("/login", h.handleLoginPage)
	router.Post("/login", h.handleLogin)
	router.Get("/register", h.handleRegisterPage)
	router.Post("/register", h.handleRegister)
	router.Post("/logout", h.handleLogout)

	router.Group(func(r chi.Router) {
		r.Use(h.requireAuth)

		r.Get("/", h.handleIndex)
		r.Get("/me", h.handleAccountPage)
		r.Post("/tasks", h.handleCreateTask)
		r.Post("/tasks/{taskID}/complete", h.handleCompleteTask)
		r.Post("/tasks/{taskID}/postpone", h.handlePostponeTask)
		r.Post("/imports/ics", h.handleImportICS)

		r.Group(func(r chi.Router) {
			r.Use(h.requireAdmin)
			r.Get("/admin/users", h.handleAdminUsers)
			r.Post("/admin/users/{userID}/approve", h.handleApproveUser)
			r.Post("/admin/users/{userID}/reject", h.handleRejectUser)
		})
	})

	return router
}

func (h *Handler) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.optionalCurrentUser(r); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	data := AuthPageData{
		Title:         "登录",
		Heading:       "登录你的 Todo 账户",
		Message:       r.URL.Query().Get("msg"),
		Error:         r.URL.Query().Get("err"),
		Action:        "/login",
		SubmitLabel:   "登录",
		UsernameValue: strings.TrimSpace(r.URL.Query().Get("username")),
	}
	if h.allowRegistration {
		data.AlternativeLabel = "注册新账号"
		data.AlternativePath = "/register"
	}

	if err := h.templates.ExecuteTemplate(w, "login.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) handleRegisterPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.optionalCurrentUser(r); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if !h.allowRegistration {
		h.redirectToLogin(w, r, "", "当前已关闭注册")
		return
	}

	data := AuthPageData{
		Title:            "注册",
		Heading:          "创建你的 Todo 账户",
		Message:          r.URL.Query().Get("msg"),
		Error:            r.URL.Query().Get("err"),
		Action:           "/register",
		SubmitLabel:      "提交注册申请",
		AlternativeLabel: "已有账号，去登录",
		AlternativePath:  "/login",
		ShowDisplayName:  true,
		UsernameValue:    strings.TrimSpace(r.URL.Query().Get("username")),
		DisplayNameValue: strings.TrimSpace(r.URL.Query().Get("display_name")),
	}

	if err := h.templates.ExecuteTemplate(w, "register.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.redirectToLogin(w, r, "", "请求解析失败")
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")

	result, err := h.authService.Login(r.Context(), username, password, r.UserAgent(), clientIPAddress(r))
	if err != nil {
		h.redirectToAuth(
			w,
			r,
			"/login",
			"",
			humanizeError(err),
			map[string]string{"username": username},
		)
		return
	}

	h.setSessionCookie(w, result.Token, result.ExpiresAt)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	if !h.allowRegistration {
		h.redirectToLogin(w, r, "", "当前已关闭注册")
		return
	}
	if err := r.ParseForm(); err != nil {
		h.redirectToRegister(w, r, "", "请求解析失败", "", "")
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	displayName := strings.TrimSpace(r.FormValue("display_name"))
	password := r.FormValue("password")

	result, err := h.authService.Register(r.Context(), username, displayName, password, r.UserAgent(), clientIPAddress(r))
	if err != nil {
		h.redirectToRegister(w, r, "", humanizeError(err), username, displayName)
		return
	}

	if result.PendingApproval {
		h.redirectToLogin(w, r, "注册申请已提交，等待管理员审批后再登录", "")
		return
	}

	h.setSessionCookie(w, result.Token, result.ExpiresAt)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	_ = h.authService.Logout(r.Context(), h.sessionToken(r))
	h.clearSessionCookie(w)
	h.redirectToLogin(w, r, "已退出登录", "")
}

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(r)
	if !ok {
		h.redirectToLogin(w, r, "", "请先登录")
		return
	}

	focusDate, err := h.resolveFocusDate(r)
	if err != nil {
		h.redirectWithQuery(w, r, "/", "", "日期格式不正确", nil)
		return
	}

	if err := h.renderIndex(w, r, user, focusDate, r.URL.Query().Get("msg"), r.URL.Query().Get("err")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(r)
	if !ok {
		h.redirectToLogin(w, r, "", "请先登录")
		return
	}
	if err := r.ParseForm(); err != nil {
		h.redirectHome(w, r, "", "请求解析失败")
		return
	}

	input := strings.TrimSpace(r.FormValue("input"))
	if input == "" {
		h.redirectHome(w, r, "", "输入不能为空")
		return
	}

	if _, err := h.taskService.CreateFromInput(r.Context(), user.ID, input); err != nil {
		h.redirectHome(w, r, "", humanizeError(err))
		return
	}

	h.redirectHome(w, r, "", "")
}

func (h *Handler) handleCompleteTask(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(r)
	if !ok {
		h.redirectToLogin(w, r, "", "请先登录")
		return
	}

	taskID := chi.URLParam(r, "taskID")
	if err := h.taskService.Complete(r.Context(), user.ID, taskID); err != nil {
		h.redirectHome(w, r, "", humanizeError(err))
		return
	}

	h.redirectHome(w, r, "", "")
}

func (h *Handler) handlePostponeTask(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(r)
	if !ok {
		h.redirectToLogin(w, r, "", "请先登录")
		return
	}
	if err := r.ParseForm(); err != nil {
		h.redirectHome(w, r, "", "请求解析失败")
		return
	}

	taskID := chi.URLParam(r, "taskID")
	targetDate := strings.TrimSpace(r.FormValue("target_date"))
	if targetDate == "" {
		h.redirectHome(w, r, "", "请选择新的日期")
		return
	}

	if err := h.taskService.Postpone(r.Context(), user.ID, taskID, targetDate); err != nil {
		h.redirectHome(w, r, "", humanizeError(err))
		return
	}

	h.redirectHome(w, r, "", "")
}

func (h *Handler) handleImportICS(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(r)
	if !ok {
		h.redirectToLogin(w, r, "", "请先登录")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadSize)
	if err := r.ParseMultipartForm(h.maxUploadSize); err != nil {
		h.redirectHome(w, r, "", "ICS 文件过大或格式错误")
		return
	}

	file, header, err := r.FormFile("ics_file")
	if err != nil {
		h.redirectHome(w, r, "", "请选择 ICS 文件")
		return
	}
	defer file.Close()

	body, err := io.ReadAll(file)
	if err != nil {
		h.redirectHome(w, r, "", "读取 ICS 文件失败")
		return
	}

	inserted, err := h.taskService.ImportICS(r.Context(), user.ID, header.Filename, body)
	if err != nil {
		h.redirectHome(w, r, "", humanizeError(err))
		return
	}

	_ = inserted
	h.redirectHome(w, r, "", "")
}

func (h *Handler) renderIndex(w http.ResponseWriter, r *http.Request, user domain.User, focusDate time.Time, message, errorMessage string) error {
	dashboard, err := h.taskService.DashboardForDate(r.Context(), user.ID, focusDate)
	if err != nil {
		return err
	}

	today := normalizeDateForView(time.Now().In(h.location), h.location)
	pageData := DashboardPageData{
		CurrentUser:          buildUserView(user),
		Error:                errorMessage,
		FocusTitle:           buildFocusTitle(focusDate, today, h.location),
		FocusDateISO:         focusDate.In(h.location).Format("2006-01-02"),
		FocusYear:            focusDate.In(h.location).Format("2006"),
		FocusMonth:           focusDate.In(h.location).Format("01"),
		FocusDay:             focusDate.In(h.location).Format("02"),
		FocusTasks:           buildFocusTaskCards(dashboard, focusDate, h.location),
		YesterdayPath:        buildDatePath(today.AddDate(0, 0, -1), h.location),
		TodayPath:            buildDatePath(today, h.location),
		TomorrowPath:         buildDatePath(today.AddDate(0, 0, 1), h.location),
		DayAfterTomorrowPath: buildDatePath(today.AddDate(0, 0, 2), h.location),
	}
	_ = message

	return h.templates.ExecuteTemplate(w, "index.html", pageData)
}

func (h *Handler) handleAccountPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(r)
	if !ok {
		h.redirectToLogin(w, r, "", "请先登录")
		return
	}

	if err := h.templates.ExecuteTemplate(w, "account.html", AccountPageData{
		CurrentUser: buildUserView(user),
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(r)
	if !ok {
		h.redirectToLogin(w, r, "", "请先登录")
		return
	}

	pendingUsers, err := h.authService.ListPendingUsers(r.Context(), user)
	if err != nil {
		h.redirectHome(w, r, "", humanizeError(err))
		return
	}

	data := AdminPageData{
		CurrentUser:  buildUserView(user),
		Message:      r.URL.Query().Get("msg"),
		Error:        r.URL.Query().Get("err"),
		PendingUsers: buildPendingUserCards(pendingUsers, h.location),
	}

	if err := h.templates.ExecuteTemplate(w, "admin_users.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) handleApproveUser(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(r)
	if !ok {
		h.redirectToLogin(w, r, "", "请先登录")
		return
	}

	userID := chi.URLParam(r, "userID")
	approvedUser, err := h.authService.ApproveUser(r.Context(), user, userID)
	if err != nil {
		h.redirectToAdminUsers(w, r, "", humanizeError(err))
		return
	}

	h.redirectToAdminUsers(w, r, fmt.Sprintf("已批准 @%s", approvedUser.Username), "")
}

func (h *Handler) handleRejectUser(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(r)
	if !ok {
		h.redirectToLogin(w, r, "", "请先登录")
		return
	}

	userID := chi.URLParam(r, "userID")
	rejectedUser, err := h.authService.RejectUser(r.Context(), user, userID)
	if err != nil {
		h.redirectToAdminUsers(w, r, "", humanizeError(err))
		return
	}

	h.redirectToAdminUsers(w, r, fmt.Sprintf("已拒绝并删除 @%s", rejectedUser.Username), "")
}

func (h *Handler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := h.authService.Authenticate(r.Context(), h.sessionToken(r))
		if err != nil {
			h.clearSessionCookie(w)
			h.redirectToLogin(w, r, "", "请先登录")
			return
		}

		ctx := context.WithValue(r.Context(), currentUserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handler) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := h.currentUser(r)
		if !ok {
			h.redirectToLogin(w, r, "", "请先登录")
			return
		}
		if !user.IsAdmin() {
			h.redirectHome(w, r, "", "只有管理员可以执行该操作")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) currentUser(r *http.Request) (domain.User, bool) {
	user, ok := r.Context().Value(currentUserContextKey).(domain.User)
	return user, ok
}

func (h *Handler) optionalCurrentUser(r *http.Request) (domain.User, bool) {
	token := h.sessionToken(r)
	if token == "" {
		return domain.User{}, false
	}

	user, err := h.authService.Authenticate(r.Context(), token)
	if err != nil {
		return domain.User{}, false
	}
	return user, true
}

func (h *Handler) setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   h.sessionSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   h.sessionSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) sessionToken(r *http.Request) string {
	cookie, err := r.Cookie(h.sessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (h *Handler) redirectHome(w http.ResponseWriter, r *http.Request, message, errorMessage string) {
	extra := map[string]string{}
	if date := h.currentViewDateParam(r); date != "" {
		extra["date"] = date
	}
	h.redirectWithQuery(w, r, "/", message, errorMessage, extra)
}

func (h *Handler) redirectToAdminUsers(w http.ResponseWriter, r *http.Request, message, errorMessage string) {
	h.redirectWithQuery(w, r, "/admin/users", message, errorMessage, nil)
}

func (h *Handler) redirectToLogin(w http.ResponseWriter, r *http.Request, message, errorMessage string) {
	h.redirectWithQuery(w, r, "/login", message, errorMessage, nil)
}

func (h *Handler) redirectToRegister(w http.ResponseWriter, r *http.Request, message, errorMessage, username, displayName string) {
	h.redirectWithQuery(w, r, "/register", message, errorMessage, map[string]string{
		"username":     username,
		"display_name": displayName,
	})
}

func (h *Handler) redirectToAuth(w http.ResponseWriter, r *http.Request, path, message, errorMessage string, extra map[string]string) {
	h.redirectWithQuery(w, r, path, message, errorMessage, extra)
}

func (h *Handler) redirectWithQuery(w http.ResponseWriter, r *http.Request, path, message, errorMessage string, extra map[string]string) {
	values := url.Values{}
	if message != "" {
		values.Set("msg", message)
	}
	if errorMessage != "" {
		values.Set("err", errorMessage)
	}
	for key, value := range extra {
		if strings.TrimSpace(value) != "" {
			values.Set(key, value)
		}
	}

	target := path
	if encoded := values.Encode(); encoded != "" {
		target = path + "?" + encoded
	}

	http.Redirect(w, r, target, http.StatusSeeOther)
}

func buildUserView(user domain.User) *UserView {
	return &UserView{
		DisplayName: user.DisplayName,
		Username:    user.Username,
		IsAdmin:     user.IsAdmin(),
	}
}

func buildPendingUserCards(users []domain.User, location *time.Location) []PendingUserCard {
	var cards []PendingUserCard
	for _, user := range users {
		cards = append(cards, PendingUserCard{
			ID:          user.ID.String(),
			DisplayName: user.DisplayName,
			Username:    user.Username,
			CreatedAt:   user.CreatedAt.In(location).Format("2006-01-02 15:04"),
		})
	}
	return cards
}

func buildFocusTaskCards(dashboard repository.Dashboard, focusDate time.Time, location *time.Location) []TaskCard {
	var cards []TaskCard

	for _, task := range dashboard.Today {
		cards = append(cards, buildTaskCard(task, focusDate, location))
	}
	for _, task := range dashboard.DDL {
		cards = append(cards, buildTaskCard(task, focusDate, location))
	}
	for _, task := range dashboard.Todo {
		cards = append(cards, buildTaskCard(task, focusDate, location))
	}
	return cards
}

func buildTaskCard(task domain.Task, focusDate time.Time, location *time.Location) TaskCard {
	card := TaskCard{
		ID:          task.ID.String(),
		Title:       task.Title,
		Note:        task.Note,
		CanComplete: task.SupportsCompletion(),
		CanPostpone: task.SupportsPostpone(),
		ReturnDate:  focusDate.In(location).Format("2006-01-02"),
	}

	focusDate = normalizeDateForView(focusDate, location)

	switch task.Type {
	case domain.TaskTypeSchedule:
		card.KindLabel = "日程"
		card.KindClass = "schedule"
		card.StatusLine = "这一天出现"
		if task.ScheduledFor != nil {
			card.PostponeValue = normalizeDateForView(*task.ScheduledFor, location).AddDate(0, 0, 1).Format("2006-01-02")
		}
	case domain.TaskTypeDDL:
		card.KindLabel = "DDL"
		card.KindClass = "ddl"
		if task.Deadline != nil {
			deadline := normalizeDateForView(*task.Deadline, location)
			diffDays := int(deadline.Sub(focusDate).Hours() / 24)
			switch {
			case diffDays < 0:
				card.StatusLine = fmt.Sprintf("已过期 %d 天", -diffDays)
			case diffDays == 0:
				card.StatusLine = "这一天截止"
			default:
				card.StatusLine = fmt.Sprintf("还有 %d 天截止", diffDays)
			}
			card.PostponeValue = deadline.AddDate(0, 0, 1).Format("2006-01-02")
		}
	case domain.TaskTypeTodo:
		card.KindLabel = "Todo"
		card.KindClass = "todo"
		card.StatusLine = "持续提醒"
	}

	if card.PostponeValue == "" {
		card.PostponeValue = focusDate.AddDate(0, 0, 1).Format("2006-01-02")
	}

	return card
}

func buildDatePath(targetDate time.Time, location *time.Location) string {
	date := normalizeDateForView(targetDate, location)
	today := normalizeDateForView(time.Now().In(location), location)
	if date.Equal(today) {
		return "/"
	}
	return "/?date=" + url.QueryEscape(date.Format("2006-01-02"))
}

func (h *Handler) resolveFocusDate(r *http.Request) (time.Time, error) {
	value := strings.TrimSpace(r.URL.Query().Get("date"))
	if value == "" {
		year := strings.TrimSpace(r.URL.Query().Get("year"))
		month := strings.TrimSpace(r.URL.Query().Get("month"))
		day := strings.TrimSpace(r.URL.Query().Get("day"))
		if year != "" || month != "" || day != "" {
			value = fmt.Sprintf("%s-%s-%s", padDatePart(year, 4), padDatePart(month, 2), padDatePart(day, 2))
		}
	}
	if value == "" {
		return normalizeDateForView(time.Now().In(h.location), h.location), nil
	}

	parsed, err := time.ParseInLocation("2006-01-02", value, h.location)
	if err != nil {
		return time.Time{}, err
	}
	return normalizeDateForView(parsed, h.location), nil
}

func (h *Handler) currentViewDateParam(r *http.Request) string {
	raw := strings.TrimSpace(r.FormValue("return_date"))
	if raw == "" {
		raw = strings.TrimSpace(r.URL.Query().Get("date"))
	}
	if raw == "" {
		return ""
	}

	parsed, err := time.ParseInLocation("2006-01-02", raw, h.location)
	if err != nil {
		return ""
	}

	date := normalizeDateForView(parsed, h.location)
	today := normalizeDateForView(time.Now().In(h.location), h.location)
	if date.Equal(today) {
		return ""
	}
	return date.Format("2006-01-02")
}

func padDatePart(value string, width int) string {
	trimmed := strings.TrimSpace(value)
	if width <= 0 {
		return trimmed
	}
	if len(trimmed) >= width {
		return trimmed
	}
	return strings.Repeat("0", width-len(trimmed)) + trimmed
}

func humanizeError(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, repository.ErrTaskNotFound):
		return "任务不存在"
	case errors.Is(err, repository.ErrUserNotFound):
		return "用户不存在"
	case errors.Is(err, repository.ErrUnsupportedOperation):
		return "这个任务不支持该操作"
	case errors.Is(err, repository.ErrInvalidTaskTransition):
		return "当前任务状态不允许该操作"
	case errors.Is(err, service.ErrInvalidCredentials):
		return "用户名或密码错误"
	case errors.Is(err, service.ErrInvalidSession):
		return "登录状态已失效，请重新登录"
	case errors.Is(err, service.ErrRegistrationDisabled):
		return "当前已关闭注册"
	case errors.Is(err, service.ErrUsernameTaken):
		return "用户名已存在"
	case errors.Is(err, service.ErrUserPendingApproval):
		return "账号还在等待管理员审批"
	case errors.Is(err, service.ErrPermissionDenied):
		return "你没有管理员权限"
	case errors.Is(err, service.ErrUserAlreadyApproved):
		return "这个账号已经审批过了"
	case errors.Is(err, service.ErrUserNotPendingReview):
		return "这个账号已不在待审批列表"
	case strings.Contains(err.Error(), "invalid user id"):
		return "用户 ID 无效"
	case strings.Contains(err.Error(), "invalid task id"):
		return "任务 ID 无效"
	case strings.Contains(err.Error(), "invalid target date"):
		return "延期日期格式不正确"
	default:
		return err.Error()
	}
}

func buildFocusTitle(focusDate, today time.Time, location *time.Location) string {
	focus := normalizeDateForView(focusDate, location)
	base := normalizeDateForView(today, location)
	diffDays := int(focus.Sub(base).Hours() / 24)

	switch diffDays {
	case -1:
		return "昨天"
	case 0:
		return "今天"
	case 1:
		return "明天"
	case 2:
		return "后天"
	default:
		return focus.In(location).Format("2006年1月2日")
	}
}

func normalizeDateForView(value time.Time, location *time.Location) time.Time {
	local := value.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}

func clientIPAddress(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
