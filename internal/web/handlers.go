package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

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
	quoteService      *service.QuoteService
	eventHub          *dashboardEventHub
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
	FocusWeekdayLabel    string
	FocusDayMarks        []string
	FocusDateISO         string
	TodayDateISO         string
	TomorrowDateISO      string
	DayAfterDateISO      string
	FocusYear            string
	FocusMonth           string
	FocusDay             string
	FocusTasks           []TaskCard
	CompletedTasks       []CompletedTaskCard
	EmptyQuote           *QuoteView
	YesterdayPath        string
	TodayPath            string
	TomorrowPath         string
	DayAfterTomorrowPath string
}

type QuoteView struct {
	Text     string `json:"text"`
	Author   string `json:"author"`
	Source   string `json:"source"`
	HasMeta  bool   `json:"has_meta"`
	MetaLine string `json:"meta_line"`
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
	ID            string `json:"id"`
	Title         string `json:"title"`
	KindLabel     string `json:"kind_label"`
	KindClass     string `json:"kind_class"`
	StatusLine    string `json:"status_line"`
	CompactStatus string `json:"compact_status_line"`
	MobileCompact bool   `json:"mobile_compact"`
	Note          string `json:"note"`
	CanComplete   bool   `json:"can_complete"`
	CanPostpone   bool   `json:"can_postpone"`
	PostponeMode  string `json:"postpone_mode"`
	PostponeValue string `json:"postpone_value"`
	PostponeMin   string `json:"postpone_min_value"`
	ReturnDate    string `json:"return_date"`
}

type CompletedTaskCard struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	KindLabel     string `json:"kind_label"`
	KindClass     string `json:"kind_class"`
	FinishedLine  string `json:"finished_line"`
	StatusLine    string `json:"status_line"`
	Note          string `json:"note"`
	CanPostpone   bool   `json:"can_postpone"`
	PostponeMode  string `json:"postpone_mode"`
	PostponeValue string `json:"postpone_value"`
	PostponeMin   string `json:"postpone_min_value"`
	ReturnDate    string `json:"return_date"`
}

type DashboardSnapshot struct {
	FocusTasks     []TaskCard          `json:"focus_tasks"`
	CompletedTasks []CompletedTaskCard `json:"completed_tasks"`
	EmptyQuote     *QuoteView          `json:"empty_quote,omitempty"`
}

type PendingUserCard struct {
	ID          string
	DisplayName string
	Username    string
	CreatedAt   string
}

func NewHandler(taskService *service.TaskService, authService *service.AuthService, quoteService *service.QuoteService, options HandlerOptions) (*Handler, error) {
	templates, err := template.ParseGlob(filepath.Join(options.TemplateDir, "*.html"))
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	return &Handler{
		taskService:       taskService,
		authService:       authService,
		quoteService:      quoteService,
		eventHub:          newDashboardEventHub(),
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
	router.Get("/favicon.ico", h.handleFavicon)
	router.Get("/manifest.webmanifest", h.handleManifest)
	router.Get("/sw.js", h.handleServiceWorker)

	router.Get("/login", h.handleLoginPage)
	router.Post("/login", h.handleLogin)
	router.Get("/register", h.handleRegisterPage)
	router.Post("/register", h.handleRegister)
	router.Post("/logout", h.handleLogout)

	router.Group(func(r chi.Router) {
		r.Use(h.requireAuth)

		r.Get("/", h.handleIndex)
		r.Get("/dashboard/snapshot", h.handleDashboardSnapshot)
		r.Get("/events", h.handleEventStream)
		r.Get("/me", h.handleAccountPage)
		r.Post("/tasks", h.handleCreateTask)
		r.Post("/tasks/manual", h.handleCreateManualTask)
		r.Post("/tasks/parse-sms", h.handleParseSMS)
		r.Post("/tasks/{taskID}/rename", h.handleRenameTask)
		r.Post("/tasks/{taskID}/complete", h.handleCompleteTask)
		r.Post("/tasks/{taskID}/restore", h.handleRestoreTask)
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

func (h *Handler) handleManifest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, filepath.Join(h.staticDir, "manifest.webmanifest"))
}

func (h *Handler) handleFavicon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/x-icon")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	http.ServeFile(w, r, filepath.Join(h.staticDir, "pwa", "favicon.ico"))
}

func (h *Handler) handleServiceWorker(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, filepath.Join(h.staticDir, "sw.js"))
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
	if err := h.parseRequestForm(r); err != nil {
		h.redirectHome(w, r, "", "请求解析失败")
		return
	}

	input := strings.TrimSpace(r.FormValue("input"))
	if input == "" {
		h.redirectHome(w, r, "", "输入不能为空")
		return
	}

	importance, err := parseOptionalImportance(r.FormValue("importance"))
	if err != nil {
		h.redirectHome(w, r, "", humanizeError(err))
		return
	}

	if _, err := h.taskService.CreateFromInputWithImportance(r.Context(), user.ID, input, importance); err != nil {
		if wantsAsyncResponse(r) {
			http.Error(w, humanizeError(err), http.StatusBadRequest)
			return
		}
		h.redirectHome(w, r, "", humanizeError(err))
		return
	}

	h.publishDashboardUpdate(user.ID.String(), requestClientID(r))

	if wantsAsyncResponse(r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	h.redirectHome(w, r, "", "")
}

func (h *Handler) handleCreateManualTask(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(r)
	if !ok {
		h.redirectToLogin(w, r, "", "请先登录")
		return
	}
	if err := h.parseRequestForm(r); err != nil {
		h.redirectHome(w, r, "", "请求解析失败")
		return
	}

	inputs, err := h.parseManualTaskForm(r)
	if err != nil {
		if wantsAsyncResponse(r) {
			http.Error(w, humanizeError(err), http.StatusBadRequest)
			return
		}
		h.redirectHome(w, r, "", humanizeError(err))
		return
	}

	if _, err := h.taskService.CreateManualTasks(r.Context(), user.ID, inputs); err != nil {
		if wantsAsyncResponse(r) {
			http.Error(w, humanizeError(err), http.StatusBadRequest)
			return
		}
		h.redirectHome(w, r, "", humanizeError(err))
		return
	}

	h.publishDashboardUpdate(user.ID.String(), requestClientID(r))

	if wantsAsyncResponse(r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	h.redirectHome(w, r, "", "")
}

func (h *Handler) handleParseSMS(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(r)
	if !ok {
		h.redirectToLogin(w, r, "", "请先登录")
		return
	}
	if err := h.parseRequestForm(r); err != nil {
		h.redirectHome(w, r, "", "请求解析失败")
		return
	}

	input := strings.TrimSpace(r.FormValue("sms_input"))
	if input == "" {
		h.redirectHome(w, r, "", "短信内容不能为空")
		return
	}

	if _, err := h.taskService.CreateFromSMSParse(r.Context(), user.ID, input); err != nil {
		if wantsAsyncResponse(r) {
			http.Error(w, humanizeError(err), http.StatusBadRequest)
			return
		}
		h.redirectHome(w, r, "", humanizeError(err))
		return
	}

	h.publishDashboardUpdate(user.ID.String(), requestClientID(r))

	if wantsAsyncResponse(r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	h.redirectHome(w, r, "", "")
}

func (h *Handler) handleRenameTask(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(r)
	if !ok {
		h.redirectToLogin(w, r, "", "请先登录")
		return
	}
	if err := h.parseRequestForm(r); err != nil {
		h.redirectHome(w, r, "", "请求解析失败")
		return
	}

	taskID := chi.URLParam(r, "taskID")
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		if wantsAsyncResponse(r) {
			http.Error(w, "标题不能为空", http.StatusBadRequest)
			return
		}
		h.redirectHome(w, r, "", "标题不能为空")
		return
	}

	if _, err := h.taskService.Rename(r.Context(), user.ID, taskID, title); err != nil {
		if wantsAsyncResponse(r) {
			http.Error(w, humanizeError(err), http.StatusBadRequest)
			return
		}
		h.redirectHome(w, r, "", humanizeError(err))
		return
	}

	h.publishDashboardUpdate(user.ID.String(), requestClientID(r))

	if wantsAsyncResponse(r) {
		w.WriteHeader(http.StatusNoContent)
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
	_, err := h.taskService.Complete(r.Context(), user.ID, taskID)
	if err != nil {
		if wantsAsyncResponse(r) {
			http.Error(w, humanizeError(err), http.StatusBadRequest)
			return
		}
		h.redirectHome(w, r, "", humanizeError(err))
		return
	}

	h.publishDashboardUpdate(user.ID.String(), requestClientID(r))

	if wantsAsyncResponse(r) {
		h.writeDashboardSnapshot(w, r, user)
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
	if err := h.parseRequestForm(r); err != nil {
		h.redirectHome(w, r, "", "请求解析失败")
		return
	}

	taskID := chi.URLParam(r, "taskID")
	targetDate := strings.TrimSpace(r.FormValue("target_value"))
	if targetDate == "" {
		targetDate = strings.TrimSpace(r.FormValue("target_date"))
	}
	if targetDate == "" {
		h.redirectHome(w, r, "", "请选择新的延期时间")
		return
	}

	if err := h.taskService.Postpone(r.Context(), user.ID, taskID, targetDate); err != nil {
		if wantsAsyncResponse(r) {
			http.Error(w, humanizeError(err), http.StatusBadRequest)
			return
		}
		h.redirectHome(w, r, "", humanizeError(err))
		return
	}

	h.publishDashboardUpdate(user.ID.String(), requestClientID(r))

	if wantsAsyncResponse(r) {
		h.writeDashboardSnapshot(w, r, user)
		return
	}

	h.redirectHome(w, r, "", "")
}

func (h *Handler) handleRestoreTask(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(r)
	if !ok {
		h.redirectToLogin(w, r, "", "请先登录")
		return
	}

	taskID := chi.URLParam(r, "taskID")
	if _, err := h.taskService.Restore(r.Context(), user.ID, taskID); err != nil {
		if wantsAsyncResponse(r) {
			http.Error(w, humanizeError(err), http.StatusBadRequest)
			return
		}
		h.redirectHome(w, r, "", humanizeError(err))
		return
	}

	h.publishDashboardUpdate(user.ID.String(), requestClientID(r))

	if wantsAsyncResponse(r) {
		h.writeDashboardSnapshot(w, r, user)
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
		if wantsAsyncResponse(r) {
			http.Error(w, humanizeError(err), http.StatusBadRequest)
			return
		}
		h.redirectHome(w, r, "", humanizeError(err))
		return
	}

	_ = inserted
	h.publishDashboardUpdate(user.ID.String(), requestClientID(r))
	if wantsAsyncResponse(r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	h.redirectHome(w, r, "", "")
}

func (h *Handler) renderIndex(w http.ResponseWriter, r *http.Request, user domain.User, focusDate time.Time, message, errorMessage string) error {
	pageData, err := h.buildDashboardPageData(r.Context(), user, focusDate, errorMessage)
	if err != nil {
		return err
	}
	_ = message

	return h.templates.ExecuteTemplate(w, "index.html", pageData)
}

func (h *Handler) buildDashboardPageData(ctx context.Context, user domain.User, focusDate time.Time, errorMessage string) (DashboardPageData, error) {
	now := time.Now().In(h.location)
	dashboard, err := h.taskService.DashboardForDate(ctx, user.ID, focusDate)
	if err != nil {
		return DashboardPageData{}, err
	}
	completedTasks, err := h.taskService.CompletedTasksForDate(ctx, user.ID, focusDate, 20)
	if err != nil {
		return DashboardPageData{}, err
	}

	today := normalizeDateForView(now, h.location)
	calendarMeta := service.CalendarMetaForDate(focusDate, h.location)
	focusTasks := buildFocusTaskCards(dashboard, now, focusDate, h.location)
	pageData := DashboardPageData{
		CurrentUser:          buildUserView(user),
		Error:                errorMessage,
		FocusTitle:           buildFocusTitle(focusDate, today, h.location),
		FocusWeekdayLabel:    calendarMeta.WeekdayLabel,
		FocusDayMarks:        calendarMeta.Tags,
		FocusDateISO:         focusDate.In(h.location).Format("2006-01-02"),
		TodayDateISO:         today.Format("2006-01-02"),
		TomorrowDateISO:      today.AddDate(0, 0, 1).Format("2006-01-02"),
		DayAfterDateISO:      today.AddDate(0, 0, 2).Format("2006-01-02"),
		FocusYear:            focusDate.In(h.location).Format("2006"),
		FocusMonth:           focusDate.In(h.location).Format("01"),
		FocusDay:             focusDate.In(h.location).Format("02"),
		FocusTasks:           focusTasks,
		CompletedTasks:       buildCompletedTaskCards(completedTasks, now, focusDate, h.location),
		YesterdayPath:        buildDatePath(today.AddDate(0, 0, -1), h.location),
		TodayPath:            buildDatePath(today, h.location),
		TomorrowPath:         buildDatePath(today.AddDate(0, 0, 1), h.location),
		DayAfterTomorrowPath: buildDatePath(today.AddDate(0, 0, 2), h.location),
	}
	if len(focusTasks) == 0 && h.quoteService != nil {
		quote, err := h.quoteService.Random(ctx)
		if err == nil && strings.TrimSpace(quote.Text) != "" {
			pageData.EmptyQuote = buildQuoteView(quote)
		}
	}
	return pageData, nil
}

func (h *Handler) writeDashboardSnapshot(w http.ResponseWriter, r *http.Request, user domain.User) {
	focusDate := h.actionFocusDate(r)
	pageData, err := h.buildDashboardPageData(r.Context(), user, focusDate, "")
	if err != nil {
		http.Error(w, humanizeError(err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(DashboardSnapshot{
		FocusTasks:     pageData.FocusTasks,
		CompletedTasks: pageData.CompletedTasks,
		EmptyQuote:     pageData.EmptyQuote,
	})
}

func (h *Handler) handleDashboardSnapshot(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(r)
	if !ok {
		http.Error(w, "请先登录", http.StatusUnauthorized)
		return
	}

	h.writeDashboardSnapshot(w, r, user)
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
			if wantsAsyncResponse(r) || wantsEventStream(r) {
				http.Error(w, "请先登录", http.StatusUnauthorized)
				return
			}
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

func buildQuoteView(quote service.Quote) *QuoteView {
	view := &QuoteView{
		Text:   quote.Text,
		Author: quote.Author,
		Source: quote.Source,
	}

	metaParts := make([]string, 0, 2)
	if view.Author != "" {
		metaParts = append(metaParts, view.Author)
	}
	if view.Source != "" {
		metaParts = append(metaParts, view.Source)
	}
	if len(metaParts) > 0 {
		view.HasMeta = true
		view.MetaLine = strings.Join(metaParts, " · ")
	}

	return view
}

func buildFocusTaskCards(dashboard repository.Dashboard, now, focusDate time.Time, location *time.Location) []TaskCard {
	var tasks []domain.Task

	for _, task := range dashboard.Today {
		tasks = append(tasks, task)
	}
	for _, task := range dashboard.DDL {
		if !shouldDisplayDDLOnFocusDate(task, focusDate, location) {
			continue
		}
		tasks = append(tasks, task)
	}
	for _, task := range dashboard.Todo {
		tasks = append(tasks, task)
	}

	sortTasksForFocus(tasks, now, focusDate, location)

	cards := make([]TaskCard, 0, len(tasks))
	for _, task := range tasks {
		cards = append(cards, buildTaskCard(task, now, focusDate, location))
	}

	return cards
}

func shouldDisplayDDLOnFocusDate(task domain.Task, focusDate time.Time, location *time.Location) bool {
	if task.Type != domain.TaskTypeDDL || task.Deadline == nil {
		return false
	}

	focusDay := normalizeDateForView(focusDate, location)
	createdDay := normalizeDateForView(task.CreatedAt, location)
	deadlineDay := normalizeDateForView(*task.Deadline, location)

	if focusDay.Before(createdDay) {
		return false
	}
	if focusDay.After(deadlineDay) {
		return false
	}
	return true
}

func buildTaskCard(task domain.Task, now, focusDate time.Time, location *time.Location) TaskCard {
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
		card.PostponeMode = "date"
		card.PostponeValue, card.PostponeMin = schedulePostponePickerValues(task, now, location)
	case domain.TaskTypeDDL:
		card.KindLabel = "DDL"
		card.KindClass = "ddl"
		card.MobileCompact = shouldPreferCompactMobileDDL(task.Title)
		if task.Deadline != nil {
			card.StatusLine = formatDDLCountdown(*task.Deadline, now, focusDate, location)
			card.CompactStatus = compactDDLCountdown(card.StatusLine)
		}
		card.PostponeMode = "datetime"
		card.PostponeValue, card.PostponeMin = ddlPostponePickerValues(task, now, location)
	case domain.TaskTypeTodo:
		card.KindLabel = "Todo"
		card.KindClass = "todo"
	}

	if card.PostponeValue == "" && card.PostponeMode == "date" {
		card.PostponeValue = focusDate.AddDate(0, 0, 1).Format("2006-01-02")
		card.PostponeMin = card.PostponeValue
	}

	return card
}

func compactDDLCountdown(value string) string {
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(trimmed, "还有 ") {
		return strings.TrimPrefix(trimmed, "还有 ")
	}
	return trimmed
}

func shouldPreferCompactMobileDDL(title string) bool {
	if strings.TrimSpace(title) == "" {
		return false
	}

	var units float64
	for _, r := range title {
		switch {
		case unicode.IsSpace(r):
			units += 0.32
		case unicode.Is(unicode.Han, r) || unicode.In(r, unicode.Hiragana, unicode.Katakana, unicode.Hangul):
			units += 1
		case unicode.IsDigit(r):
			units += 0.58
		case unicode.IsUpper(r):
			units += 0.68
		case unicode.IsPunct(r) && r > 127:
			units += 0.86
		case unicode.IsLetter(r):
			units += 0.6
		default:
			units += 0.62
		}
	}

	return units > 8.3
}

func sortTasksForFocus(tasks []domain.Task, now, focusDate time.Time, location *time.Location) {
	now = now.In(location)
	focusDate = normalizeDateForView(focusDate, location)

	sort.SliceStable(tasks, func(i, j int) bool {
		left := tasks[i]
		right := tasks[j]

		leftHourlyDDL := isHourlyCountdownDDL(left, now, focusDate, location)
		rightHourlyDDL := isHourlyCountdownDDL(right, now, focusDate, location)
		if leftHourlyDDL != rightHourlyDDL {
			return leftHourlyDDL
		}

		if left.Importance != right.Importance {
			return left.Importance > right.Importance
		}

		leftTime, leftHasTime := taskSortTime(left, focusDate, location)
		rightTime, rightHasTime := taskSortTime(right, focusDate, location)
		if leftHasTime != rightHasTime {
			return leftHasTime
		}
		if leftHasTime && !leftTime.Equal(rightTime) {
			return leftTime.Before(rightTime)
		}

		leftTypeRank := taskTypeSortRank(left.Type)
		rightTypeRank := taskTypeSortRank(right.Type)
		if leftTypeRank != rightTypeRank {
			return leftTypeRank < rightTypeRank
		}

		if !left.CreatedAt.Equal(right.CreatedAt) {
			return left.CreatedAt.Before(right.CreatedAt)
		}

		return left.ID.String() < right.ID.String()
	})
}

func taskSortTime(task domain.Task, focusDate time.Time, location *time.Location) (time.Time, bool) {
	switch task.Type {
	case domain.TaskTypeSchedule:
		if task.ScheduledFor == nil {
			return time.Time{}, false
		}
		return normalizeDateForView(*task.ScheduledFor, location), true
	case domain.TaskTypeDDL:
		if task.Deadline == nil {
			return time.Time{}, false
		}
		return task.Deadline.In(location), true
	default:
		_ = focusDate
		return time.Time{}, false
	}
}

func taskTypeSortRank(taskType domain.TaskType) int {
	switch taskType {
	case domain.TaskTypeSchedule:
		return 0
	case domain.TaskTypeDDL:
		return 1
	default:
		return 2
	}
}

func isHourlyCountdownDDL(task domain.Task, now, focusDate time.Time, location *time.Location) bool {
	if task.Type != domain.TaskTypeDDL || task.Deadline == nil {
		return false
	}
	actualToday := normalizeDateForView(now, location)
	focusDay := normalizeDateForView(focusDate, location)
	if !focusDay.Equal(actualToday) {
		return false
	}
	return normalizeDateForView(*task.Deadline, location).Equal(focusDay)
}

func formatDDLCountdown(deadline, now, focusDate time.Time, location *time.Location) string {
	deadlineLocal := deadline.In(location)
	nowLocal := now.In(location)
	deadlineDay := normalizeDateForView(deadlineLocal, location)
	focusDay := normalizeDateForView(focusDate, location)
	actualToday := normalizeDateForView(nowLocal, location)

	switch {
	case deadlineDay.After(focusDay):
		diffDays := int(deadlineDay.Sub(focusDay).Hours() / 24)
		return fmt.Sprintf("还有 %d 天", diffDays)
	case deadlineDay.Before(focusDay):
		diffDays := int(focusDay.Sub(deadlineDay).Hours() / 24)
		return fmt.Sprintf("已过期 %d 天", diffDays)
	}

	if !focusDay.Equal(actualToday) {
		return "今天"
	}

	remaining := deadlineLocal.Sub(nowLocal)
	if remaining <= 0 {
		overdue := -remaining
		if overdue >= time.Hour {
			return fmt.Sprintf("已超时 %d 小时", ceilDuration(overdue, time.Hour))
		}
		return fmt.Sprintf("已超时 %d 分钟", maxInt(1, ceilDuration(overdue, time.Minute)))
	}
	if remaining >= time.Hour {
		return fmt.Sprintf("还有 %d 小时", ceilDuration(remaining, time.Hour))
	}
	return fmt.Sprintf("还有 %d 分钟", maxInt(1, ceilDuration(remaining, time.Minute)))
}

func buildCompletedTaskCards(tasks []domain.Task, now, focusDate time.Time, location *time.Location) []CompletedTaskCard {
	cards := make([]CompletedTaskCard, 0, len(tasks))
	for _, task := range tasks {
		card := CompletedTaskCard{
			ID:           task.ID.String(),
			Title:        task.Title,
			KindLabel:    kindLabel(task.Type),
			KindClass:    string(task.Type),
			FinishedLine: formatCompletedAt(task, location),
			Note:         task.Note,
			ReturnDate:   normalizeDateForView(focusDate, location).Format("2006-01-02"),
		}

		if task.SupportsPostpone() {
			card.CanPostpone = true
		}

		switch task.Type {
		case domain.TaskTypeSchedule:
			card.PostponeMode = "date"
			card.PostponeValue, card.PostponeMin = schedulePostponePickerValues(task, now, location)
		case domain.TaskTypeDDL:
			if task.Deadline != nil {
				card.StatusLine = formatDDLCountdown(*task.Deadline, now, focusDate, location)
			}
			card.PostponeMode = "datetime"
			card.PostponeValue, card.PostponeMin = ddlPostponePickerValues(task, now, location)
		}

		cards = append(cards, card)
	}
	return cards
}

func kindLabel(taskType domain.TaskType) string {
	switch taskType {
	case domain.TaskTypeSchedule:
		return "日程"
	case domain.TaskTypeDDL:
		return "DDL"
	default:
		return "Todo"
	}
}

func formatCompletedAt(task domain.Task, location *time.Location) string {
	if task.CompletedAt == nil {
		return "已完成"
	}

	completedAt := task.CompletedAt.In(location)
	if task.Type == domain.TaskTypeDDL {
		return "完成于 " + completedAt.Format("1月2日 15:04")
	}
	return "完成于 " + completedAt.Format("1月2日")
}

func schedulePostponePickerValues(task domain.Task, now time.Time, location *time.Location) (string, string) {
	minimum := serviceEarliestSchedulePostponeDate(task, now, location)
	value := minimum.Format("2006-01-02")
	return value, value
}

func ddlPostponePickerValues(task domain.Task, now time.Time, location *time.Location) (string, string) {
	minimum := serviceEarliestDDLPostponeTime(task, now, location)
	value := minimum.Format("2006-01-02T15:04")
	return value, value
}

func serviceEarliestSchedulePostponeDate(task domain.Task, now time.Time, location *time.Location) time.Time {
	base := normalizeDateForView(now, location)
	if task.ScheduledFor != nil {
		scheduled := normalizeDateForView(*task.ScheduledFor, location)
		if scheduled.After(base) {
			base = scheduled
		}
	}
	return base.AddDate(0, 0, 1)
}

func serviceEarliestDDLPostponeTime(task domain.Task, now time.Time, location *time.Location) time.Time {
	base := now.In(location)
	if task.Deadline != nil {
		deadline := task.Deadline.In(location)
		if deadline.After(base) {
			base = deadline
		}
	}

	rounded := time.Date(
		base.Year(),
		base.Month(),
		base.Day(),
		base.Hour(),
		base.Minute(),
		0,
		0,
		location,
	)
	if !rounded.After(base) {
		rounded = rounded.Add(time.Minute)
	}
	return rounded
}

func ceilDuration(value, unit time.Duration) int {
	if unit <= 0 {
		return 0
	}
	quotient := value / unit
	if value%unit != 0 {
		quotient++
	}
	return int(quotient)
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func parseOptionalImportance(raw string) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("重要等级只能在 %d 到 %d 之间", domain.MinTaskImportance, domain.MaxTaskImportance)
	}

	normalized, err := domain.NormalizeTaskImportance(parsed)
	if err != nil {
		return 0, fmt.Errorf("重要等级只能在 %d 到 %d 之间", domain.MinTaskImportance, domain.MaxTaskImportance)
	}
	return normalized, nil
}

func (h *Handler) parseManualTaskForm(r *http.Request) ([]repository.TaskInput, error) {
	taskType := domain.TaskType(strings.TrimSpace(r.FormValue("task_type")))
	title := strings.TrimSpace(r.FormValue("title"))
	note := strings.TrimSpace(r.FormValue("note"))

	importance, err := parseOptionalImportance(r.FormValue("importance"))
	if err != nil {
		return nil, err
	}

	input := repository.TaskInput{
		Title:      title,
		Note:       note,
		Type:       taskType,
		Importance: importance,
	}

	switch taskType {
	case domain.TaskTypeTodo:
		return []repository.TaskInput{input}, nil
	case domain.TaskTypeSchedule:
		mode := strings.TrimSpace(r.FormValue("schedule_mode"))
		if mode == "" {
			mode = "single"
		}
		if mode == "batch" {
			return h.parseManualScheduleBatchForm(input, r)
		}

		rawDate := strings.TrimSpace(r.FormValue("scheduled_value"))
		if rawDate == "" {
			rawDate = strings.TrimSpace(r.FormValue("scheduled_date"))
		}
		if rawDate == "" {
			return nil, fmt.Errorf("请选择日程日期")
		}
		dateValue, err := time.ParseInLocation("2006-01-02", rawDate, h.location)
		if err != nil {
			return nil, fmt.Errorf("日程日期格式不正确")
		}
		input.ScheduledFor = &dateValue
		return []repository.TaskInput{input}, nil
	case domain.TaskTypeDDL:
		rawDateTime := strings.TrimSpace(r.FormValue("deadline_value"))
		if rawDateTime == "" {
			rawDate := strings.TrimSpace(r.FormValue("deadline_date"))
			rawTime := strings.TrimSpace(r.FormValue("deadline_time"))
			if rawDate == "" || rawTime == "" {
				return nil, fmt.Errorf("请选择截止日期和时间")
			}
			rawDateTime = rawDate + "T" + rawTime
		}
		deadline, err := time.ParseInLocation("2006-01-02T15:04", rawDateTime, h.location)
		if err != nil {
			return nil, fmt.Errorf("截止时间格式不正确")
		}
		input.Deadline = &deadline
		return []repository.TaskInput{input}, nil
	default:
		return nil, fmt.Errorf("任务类型不正确")
	}
}

func (h *Handler) parseManualScheduleBatchForm(base repository.TaskInput, r *http.Request) ([]repository.TaskInput, error) {
	rawStart := strings.TrimSpace(r.FormValue("batch_start_value"))
	rawEnd := strings.TrimSpace(r.FormValue("batch_end_value"))
	if rawStart == "" || rawEnd == "" {
		return nil, fmt.Errorf("请选择起始日期和截止日期")
	}

	startDate, err := time.ParseInLocation("2006-01-02", rawStart, h.location)
	if err != nil {
		return nil, fmt.Errorf("起始日期格式不正确")
	}
	endDate, err := time.ParseInLocation("2006-01-02", rawEnd, h.location)
	if err != nil {
		return nil, fmt.Errorf("截止日期格式不正确")
	}

	startDate = normalizeDateForView(startDate, h.location)
	endDate = normalizeDateForView(endDate, h.location)
	if endDate.Before(startDate) {
		return nil, fmt.Errorf("截止日期不能早于起始日期")
	}

	selectedWeekdays, err := parseBatchWeekdays(r.Form["batch_weekdays"])
	if err != nil {
		return nil, err
	}
	if len(selectedWeekdays) == 0 {
		return nil, fmt.Errorf("请至少选择一个星期")
	}

	inputs := make([]repository.TaskInput, 0, 8)
	for current := startDate; !current.After(endDate); current = current.AddDate(0, 0, 1) {
		if !selectedWeekdays[current.Weekday()] {
			continue
		}

		dateValue := current
		input := base
		input.ScheduledFor = &dateValue
		inputs = append(inputs, input)
	}

	if len(inputs) == 0 {
		return nil, fmt.Errorf("所选区间内没有匹配的日期")
	}

	return inputs, nil
}

func parseBatchWeekdays(values []string) (map[time.Weekday]bool, error) {
	weekdays := make(map[time.Weekday]bool, len(values))
	for _, raw := range values {
		switch strings.TrimSpace(raw) {
		case "mon":
			weekdays[time.Monday] = true
		case "tue":
			weekdays[time.Tuesday] = true
		case "wed":
			weekdays[time.Wednesday] = true
		case "thu":
			weekdays[time.Thursday] = true
		case "fri":
			weekdays[time.Friday] = true
		case "sat":
			weekdays[time.Saturday] = true
		case "sun":
			weekdays[time.Sunday] = true
		case "":
		default:
			return nil, fmt.Errorf("星期选择不正确")
		}
	}
	return weekdays, nil
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

func (h *Handler) parseRequestForm(r *http.Request) error {
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		maxMemory := int64(8 << 20)
		if h.maxUploadSize > 0 && h.maxUploadSize < maxMemory {
			maxMemory = h.maxUploadSize
		}
		return r.ParseMultipartForm(maxMemory)
	}
	return r.ParseForm()
}

func (h *Handler) actionFocusDate(r *http.Request) time.Time {
	raw := strings.TrimSpace(r.FormValue("return_date"))
	if raw == "" {
		raw = strings.TrimSpace(r.URL.Query().Get("date"))
	}
	if raw == "" {
		return normalizeDateForView(time.Now().In(h.location), h.location)
	}

	parsed, err := time.ParseInLocation("2006-01-02", raw, h.location)
	if err != nil {
		return normalizeDateForView(time.Now().In(h.location), h.location)
	}
	return normalizeDateForView(parsed, h.location)
}

func wantsAsyncResponse(r *http.Request) bool {
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Requested-With")), "fetch")
}

func wantsEventStream(r *http.Request) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(r.Header.Get("Accept"))), "text/event-stream")
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
	case errors.Is(err, domain.ErrInvalidTaskImportance):
		return fmt.Sprintf("重要等级只能在 %d 到 %d 之间", domain.MinTaskImportance, domain.MaxTaskImportance)
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
	case strings.Contains(err.Error(), "invalid target time"):
		return "延期时间格式不正确"
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
