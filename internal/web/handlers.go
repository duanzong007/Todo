package web

import (
	"errors"
	"fmt"
	"html/template"
	"io"
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

type Handler struct {
	service       *service.TaskService
	templates     *template.Template
	staticDir     string
	maxUploadSize int64
	location      *time.Location
}

type PageData struct {
	TodayLabel string
	Message    string
	Error      string
	TodayTasks []TaskCard
	DDLTasks   []TaskCard
	TodoTasks  []TaskCard
}

type TaskCard struct {
	ID            string
	Title         string
	StatusLine    string
	Note          string
	CanComplete   bool
	CanPostpone   bool
	PostponeValue string
}

func NewHandler(taskService *service.TaskService, templateDir, staticDir string, maxUploadSize int64, location *time.Location) (*Handler, error) {
	templates, err := template.ParseGlob(filepath.Join(templateDir, "*.html"))
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	return &Handler{
		service:       taskService,
		templates:     templates,
		staticDir:     staticDir,
		maxUploadSize: maxUploadSize,
		location:      location,
	}, nil
}

func (h *Handler) Router() http.Handler {
	router := chi.NewRouter()

	router.Get("/", h.handleIndex)
	router.Post("/tasks", h.handleCreateTask)
	router.Post("/tasks/{taskID}/complete", h.handleCompleteTask)
	router.Post("/tasks/{taskID}/postpone", h.handlePostponeTask)
	router.Post("/imports/ics", h.handleImportICS)

	fileServer := http.FileServer(http.Dir(h.staticDir))
	router.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	return router
}

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	if err := h.renderIndex(w, r, r.URL.Query().Get("msg"), r.URL.Query().Get("err")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.redirectHome(w, r, "", "请求解析失败")
		return
	}

	input := strings.TrimSpace(r.FormValue("input"))
	if input == "" {
		h.redirectHome(w, r, "", "输入不能为空")
		return
	}

	if _, err := h.service.CreateFromInput(r.Context(), input); err != nil {
		h.redirectHome(w, r, "", humanizeError(err))
		return
	}

	h.redirectHome(w, r, "任务已创建", "")
}

func (h *Handler) handleCompleteTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	if err := h.service.Complete(r.Context(), taskID); err != nil {
		h.redirectHome(w, r, "", humanizeError(err))
		return
	}

	h.redirectHome(w, r, "任务已完成", "")
}

func (h *Handler) handlePostponeTask(w http.ResponseWriter, r *http.Request) {
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

	if err := h.service.Postpone(r.Context(), taskID, targetDate); err != nil {
		h.redirectHome(w, r, "", humanizeError(err))
		return
	}

	h.redirectHome(w, r, "任务已延期", "")
}

func (h *Handler) handleImportICS(w http.ResponseWriter, r *http.Request) {
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

	inserted, err := h.service.ImportICS(r.Context(), header.Filename, body)
	if err != nil {
		h.redirectHome(w, r, "", humanizeError(err))
		return
	}

	h.redirectHome(w, r, fmt.Sprintf("ICS 导入完成，新增 %d 条日程", inserted), "")
}

func (h *Handler) renderIndex(w http.ResponseWriter, r *http.Request, message, errorMessage string) error {
	dashboard, now, err := h.service.Dashboard(r.Context())
	if err != nil {
		return err
	}

	pageData := PageData{
		TodayLabel: now.In(h.location).Format("2006-01-02"),
		Message:    message,
		Error:      errorMessage,
		TodayTasks: buildTaskCards(dashboard.Today, now, h.location),
		DDLTasks:   buildTaskCards(dashboard.DDL, now, h.location),
		TodoTasks:  buildTaskCards(dashboard.Todo, now, h.location),
	}

	return h.templates.ExecuteTemplate(w, "index.html", pageData)
}

func (h *Handler) redirectHome(w http.ResponseWriter, r *http.Request, message, errorMessage string) {
	values := url.Values{}
	if message != "" {
		values.Set("msg", message)
	}
	if errorMessage != "" {
		values.Set("err", errorMessage)
	}

	target := "/"
	if encoded := values.Encode(); encoded != "" {
		target = "/?" + encoded
	}

	http.Redirect(w, r, target, http.StatusSeeOther)
}

func buildTaskCards(tasks []domain.Task, now time.Time, location *time.Location) []TaskCard {
	var cards []TaskCard
	today := normalizeDateForView(now, location)

	for _, task := range tasks {
		card := TaskCard{
			ID:          task.ID.String(),
			Title:       task.Title,
			Note:        task.Note,
			CanComplete: task.SupportsCompletion(),
			CanPostpone: task.SupportsPostpone(),
		}

		switch task.Type {
		case domain.TaskTypeSchedule:
			card.StatusLine = "今天"
			if task.ScheduledFor != nil {
				card.PostponeValue = task.ScheduledFor.AddDate(0, 0, 1).Format("2006-01-02")
			}
		case domain.TaskTypeDDL:
			if task.Deadline != nil {
				deadline := normalizeDateForView(*task.Deadline, location)
				diffDays := int(deadline.Sub(today).Hours() / 24)
				switch {
				case diffDays < 0:
					card.StatusLine = fmt.Sprintf("已过期 %d 天", -diffDays)
				case diffDays == 0:
					card.StatusLine = "今天截止"
				default:
					card.StatusLine = fmt.Sprintf("DDL 还有 %d 天", diffDays)
				}
				card.PostponeValue = deadline.AddDate(0, 0, 1).Format("2006-01-02")
			}
		case domain.TaskTypeTodo:
			card.StatusLine = "持续提醒"
		}

		if card.PostponeValue == "" {
			card.PostponeValue = today.AddDate(0, 0, 1).Format("2006-01-02")
		}

		cards = append(cards, card)
	}

	return cards
}

func humanizeError(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, repository.ErrTaskNotFound):
		return "任务不存在"
	case errors.Is(err, repository.ErrUnsupportedOperation):
		return "这个任务不支持该操作"
	case errors.Is(err, repository.ErrInvalidTaskTransition):
		return "当前任务状态不允许该操作"
	case strings.Contains(err.Error(), "invalid task id"):
		return "任务 ID 无效"
	case strings.Contains(err.Error(), "invalid target date"):
		return "延期日期格式不正确"
	default:
		return err.Error()
	}
}

func normalizeDateForView(value time.Time, location *time.Location) time.Time {
	local := value.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}
