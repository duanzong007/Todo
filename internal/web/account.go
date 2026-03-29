package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"todo/internal/domain"
	"todo/internal/repository"
	"todo/internal/service"
)

type accountTaskFilterInput struct {
	Query       string
	Status      string
	Scope       string
	DateField   string
	Sort        string
	Limit       int
	DateFrom    string
	DateTo      string
	Types       []string
	Importances []string
}

func (h *Handler) buildAccountPageData(r *http.Request, user domain.User) (AccountPageData, error) {
	filterInput, repoFilter := h.parseAccountTaskFilter(r)

	tasks, err := h.taskService.ListManagedTasks(r.Context(), user.ID, repoFilter)
	if err != nil {
		return AccountPageData{}, err
	}

	shareUsers, err := h.authService.ListShareableUsers(r.Context(), user)
	if err != nil {
		return AccountPageData{}, err
	}

	return AccountPageData{
		CurrentUser: buildUserView(user),
		Message:     strings.TrimSpace(r.URL.Query().Get("msg")),
		Error:       strings.TrimSpace(r.URL.Query().Get("err")),
		ReturnQuery: encodeAccountReturnQuery(r.URL.Query()),
		Filter:      buildAccountTaskFilterView(filterInput),
		Tasks:       buildManagedTaskCards(tasks, user, h.location),
		ShareUsers:  buildShareableUserCards(shareUsers),
	}, nil
}

func (h *Handler) handleAccountTaskApply(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(r)
	if !ok {
		h.redirectToLogin(w, r, "", "请先登录")
		return
	}
	if err := h.parseRequestForm(r); err != nil {
		h.redirectToAccountPage(w, r, "", "请求解析失败")
		return
	}

	input := service.TaskManagementActionInput{
		Action:          strings.TrimSpace(r.FormValue("action")),
		SelectedTaskIDs: splitCSVValues(r.FormValue("selected_ids")),
		ReplaceTitle:    strings.TrimSpace(r.FormValue("replace_title")),
		Prefix:          strings.TrimSpace(r.FormValue("title_prefix")),
		Suffix:          strings.TrimSpace(r.FormValue("title_suffix")),
		Importance:      strings.TrimSpace(r.FormValue("importance")),
		ScheduleDate:    strings.TrimSpace(r.FormValue("schedule_date")),
		DeadlineDate:    strings.TrimSpace(r.FormValue("deadline_date")),
		DeadlineTime:    strings.TrimSpace(r.FormValue("deadline_time")),
		ShareUserIDs:    append([]string(nil), r.Form["share_user_id"]...),
	}

	result, err := h.taskService.ApplyManagementAction(r.Context(), user, input)
	if err != nil {
		h.redirectToAccountPage(w, r, "", humanizeError(err))
		return
	}

	h.publishDashboardUpdatesForUUIDs(result.AudienceUserIDs, requestClientID(r))
	h.redirectToAccountPage(w, r, result.Message, "")
}

func (h *Handler) redirectToAccountPage(w http.ResponseWriter, r *http.Request, message, errorMessage string) {
	target := "/me"
	returnQuery := strings.TrimSpace(r.FormValue("return_query"))
	if returnQuery == "" {
		returnQuery = encodeAccountReturnQuery(r.URL.Query())
	}

	values, err := url.ParseQuery(returnQuery)
	if err != nil {
		values = url.Values{}
	}

	if strings.TrimSpace(message) != "" {
		values.Set("msg", message)
	} else {
		values.Del("msg")
	}
	if strings.TrimSpace(errorMessage) != "" {
		values.Set("err", errorMessage)
	} else {
		values.Del("err")
	}

	encoded := values.Encode()
	if encoded != "" {
		target += "?" + encoded
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (h *Handler) parseAccountTaskFilter(r *http.Request) (accountTaskFilterInput, repository.TaskManagementFilter) {
	queryValues := r.URL.Query()
	input := accountTaskFilterInput{
		Query:       strings.TrimSpace(queryValues.Get("q")),
		Status:      normalizeAccountSingleValue(queryValues.Get("status"), "all", "active", "done"),
		Scope:       normalizeAccountSingleValue(queryValues.Get("scope"), "all", "mine", "shared"),
		DateField:   normalizeAccountSingleValue(queryValues.Get("date_field"), "", "planned", "created", "completed"),
		Sort:        normalizeAccountSingleValue(queryValues.Get("sort"), "updated_desc", "updated_desc", "created_desc", "importance_desc", "planned_asc"),
		Limit:       normalizeAccountLimit(queryValues.Get("limit")),
		DateFrom:    strings.TrimSpace(queryValues.Get("date_from")),
		DateTo:      strings.TrimSpace(queryValues.Get("date_to")),
		Types:       queryValues["type"],
		Importances: queryValues["importance"],
	}

	filter := repository.TaskManagementFilter{
		Query:     input.Query,
		Status:    input.Status,
		Scope:     input.Scope,
		DateField: input.DateField,
		Sort:      input.Sort,
		TimeZone:  h.location.String(),
		Limit:     input.Limit,
	}

	for _, rawType := range input.Types {
		switch strings.TrimSpace(rawType) {
		case string(domain.TaskTypeTodo):
			filter.Types = append(filter.Types, domain.TaskTypeTodo)
		case string(domain.TaskTypeSchedule):
			filter.Types = append(filter.Types, domain.TaskTypeSchedule)
		case string(domain.TaskTypeDDL):
			filter.Types = append(filter.Types, domain.TaskTypeDDL)
		}
	}

	for _, rawImportance := range input.Importances {
		value, err := strconv.Atoi(strings.TrimSpace(rawImportance))
		if err != nil {
			continue
		}
		if value < domain.MinTaskImportance || value > domain.MaxTaskImportance {
			continue
		}
		filter.Importance = append(filter.Importance, value)
	}

	if input.DateFrom != "" {
		if parsed, err := time.ParseInLocation("2006-01-02", input.DateFrom, h.location); err == nil {
			value := normalizeDateForView(parsed, h.location)
			filter.DateFrom = &value
			input.DateFrom = value.Format("2006-01-02")
		} else {
			input.DateFrom = ""
		}
	}

	if input.DateTo != "" {
		if parsed, err := time.ParseInLocation("2006-01-02", input.DateTo, h.location); err == nil {
			value := normalizeDateForView(parsed, h.location)
			filter.DateTo = &value
			input.DateTo = value.Format("2006-01-02")
		} else {
			input.DateTo = ""
		}
	}

	return input, filter
}

func buildAccountTaskFilterView(input accountTaskFilterInput) AccountTaskFilterView {
	return AccountTaskFilterView{
		Query:    input.Query,
		Summary:  buildAccountFilterSummary(input),
		DateFrom: input.DateFrom,
		DateTo:   input.DateTo,
		StatusOptions: []AccountFilterOption{
			{Value: "all", Label: "全部状态", Selected: input.Status == "all"},
			{Value: "active", Label: "待确认", Selected: input.Status == "active"},
			{Value: "done", Label: "已完成", Selected: input.Status == "done"},
		},
		ScopeOptions: []AccountFilterOption{
			{Value: "all", Label: "全部任务", Selected: input.Scope == "all"},
			{Value: "mine", Label: "我创建的", Selected: input.Scope == "mine"},
			{Value: "shared", Label: "共享给我的", Selected: input.Scope == "shared"},
		},
		DateFieldOptions: []AccountFilterOption{
			{Value: "", Label: "不按日期筛选", Selected: input.DateField == ""},
			{Value: "planned", Label: "按日程/DDL 日期", Selected: input.DateField == "planned"},
			{Value: "created", Label: "按创建日期", Selected: input.DateField == "created"},
			{Value: "completed", Label: "按完成日期", Selected: input.DateField == "completed"},
		},
		SortOptions: []AccountFilterOption{
			{Value: "updated_desc", Label: "最近更新", Selected: input.Sort == "updated_desc"},
			{Value: "created_desc", Label: "最近创建", Selected: input.Sort == "created_desc"},
			{Value: "importance_desc", Label: "重要等级优先", Selected: input.Sort == "importance_desc"},
			{Value: "planned_asc", Label: "时间靠前优先", Selected: input.Sort == "planned_asc"},
		},
		LimitOptions: []AccountFilterOption{
			{Value: "10", Label: "10 条", Selected: input.Limit == 10},
			{Value: "20", Label: "20 条", Selected: input.Limit == 20},
			{Value: "40", Label: "40 条", Selected: input.Limit == 40},
			{Value: "100", Label: "100 条", Selected: input.Limit == 100},
		},
		TypeOptions: []AccountCheckOption{
			{Value: string(domain.TaskTypeTodo), Label: "Todo", Checked: containsString(input.Types, string(domain.TaskTypeTodo))},
			{Value: string(domain.TaskTypeSchedule), Label: "日程", Checked: containsString(input.Types, string(domain.TaskTypeSchedule))},
			{Value: string(domain.TaskTypeDDL), Label: "DDL", Checked: containsString(input.Types, string(domain.TaskTypeDDL))},
		},
		ImportanceOptions: []AccountCheckOption{
			{Value: "1", Label: "1 星", Checked: containsString(input.Importances, "1")},
			{Value: "2", Label: "2 星", Checked: containsString(input.Importances, "2")},
			{Value: "3", Label: "3 星", Checked: containsString(input.Importances, "3")},
			{Value: "4", Label: "4 星", Checked: containsString(input.Importances, "4")},
			{Value: "5", Label: "5 星", Checked: containsString(input.Importances, "5")},
		},
	}
}

func buildAccountFilterSummary(input accountTaskFilterInput) string {
	parts := []string{}

	if trimmed := strings.TrimSpace(input.Query); trimmed != "" {
		parts = append(parts, "搜索："+trimmed)
	}

	switch input.Status {
	case "active":
		parts = append(parts, "只看待确认")
	case "done":
		parts = append(parts, "只看已完成")
	default:
		parts = append(parts, "全部状态")
	}

	switch input.Scope {
	case "mine":
		parts = append(parts, "我创建的")
	case "shared":
		parts = append(parts, "共享给我的")
	default:
		parts = append(parts, "全部任务")
	}

	if len(input.Types) > 0 {
		typeLabels := make([]string, 0, len(input.Types))
		for _, item := range input.Types {
			switch strings.TrimSpace(item) {
			case string(domain.TaskTypeTodo):
				typeLabels = append(typeLabels, "Todo")
			case string(domain.TaskTypeSchedule):
				typeLabels = append(typeLabels, "日程")
			case string(domain.TaskTypeDDL):
				typeLabels = append(typeLabels, "DDL")
			}
		}
		if len(typeLabels) > 0 {
			parts = append(parts, "类型："+strings.Join(typeLabels, " / "))
		}
	}

	if len(input.Importances) > 0 {
		parts = append(parts, "星级已筛选")
	}

	if input.DateField != "" && (input.DateFrom != "" || input.DateTo != "") {
		dateLabel := "日期"
		switch input.DateField {
		case "planned":
			dateLabel = "日程/DDL"
		case "created":
			dateLabel = "创建"
		case "completed":
			dateLabel = "完成"
		}
		if input.DateFrom != "" && input.DateTo != "" {
			parts = append(parts, fmt.Sprintf("%s：%s 到 %s", dateLabel, input.DateFrom, input.DateTo))
		} else if input.DateFrom != "" {
			parts = append(parts, fmt.Sprintf("%s：从 %s 起", dateLabel, input.DateFrom))
		} else if input.DateTo != "" {
			parts = append(parts, fmt.Sprintf("%s：到 %s 止", dateLabel, input.DateTo))
		}
	}

	switch input.Sort {
	case "created_desc":
		parts = append(parts, "最近创建")
	case "importance_desc":
		parts = append(parts, "重要等级优先")
	case "planned_asc":
		parts = append(parts, "时间靠前优先")
	default:
		parts = append(parts, "最近更新")
	}

	parts = append(parts, fmt.Sprintf("显示 %d 条", input.Limit))
	return strings.Join(parts, " · ")
}

func buildManagedTaskCards(tasks []repository.ManagedTask, currentUser domain.User, location *time.Location) []ManagedTaskCard {
	cards := make([]ManagedTaskCard, 0, len(tasks))
	for _, managed := range tasks {
		task := managed.Task
		card := ManagedTaskCard{
			ID:           task.ID.String(),
			Title:        task.Title,
			KindLabel:    kindLabel(task.Type),
			KindClass:    string(task.Type),
			Importance:   task.Importance,
			Note:         task.Note,
			IsOwner:      managed.OwnerID == currentUser.ID,
			SharedWithMe: managed.SharedWithMe,
		}

		if task.Status == domain.TaskStatusDone {
			card.StatusLabel = "已完成"
			card.StatusClass = "done"
		} else {
			card.StatusLabel = "待确认"
			card.StatusClass = "active"
		}

		if card.IsOwner {
			card.OwnerLine = "创建者：我"
		} else {
			card.OwnerLine = fmt.Sprintf("来自：%s @%s", managed.OwnerDisplayName, managed.OwnerUsername)
		}

		switch {
		case card.IsOwner && managed.ShareCount > 0:
			card.SharedLine = fmt.Sprintf("已共享给 %d 人", managed.ShareCount)
		case managed.SharedWithMe:
			card.SharedLine = "共享协作中"
		default:
			card.SharedLine = "仅自己可见"
		}

		switch task.Type {
		case domain.TaskTypeSchedule:
			card.ScheduleMode = "date"
			if task.ScheduledFor != nil {
				value := normalizeDateForView(*task.ScheduledFor, location)
				card.ScheduleValue = value.Format("2006-01-02")
				card.DateLine = "日期 · " + value.Format("2006-01-02")
			}
		case domain.TaskTypeDDL:
			card.ScheduleMode = "datetime"
			if task.Deadline != nil {
				value := task.Deadline.In(location)
				card.DeadlineDate = value.Format("2006-01-02")
				card.DeadlineTime = value.Format("15:04")
				card.DateLine = "截止 · " + value.Format("2006-01-02 15:04")
			}
		default:
			card.ScheduleMode = "none"
		}

		if card.DateLine == "" {
			switch {
			case task.CompletedAt != nil:
				card.DateLine = "完成 · " + task.CompletedAt.In(location).Format("2006-01-02 15:04")
			default:
				card.DateLine = "创建 · " + task.CreatedAt.In(location).Format("2006-01-02 15:04")
			}
		}

		cards = append(cards, card)
	}
	return cards
}

func buildShareableUserCards(users []domain.User) []ShareableUserCard {
	items := make([]ShareableUserCard, 0, len(users))
	for _, user := range users {
		items = append(items, ShareableUserCard{
			ID:          user.ID.String(),
			DisplayName: user.DisplayName,
			Username:    user.Username,
		})
	}
	return items
}

func encodeAccountReturnQuery(values url.Values) string {
	copied := url.Values{}
	for key, items := range values {
		if key == "msg" || key == "err" {
			continue
		}
		for _, item := range items {
			copied.Add(key, item)
		}
	}
	return copied.Encode()
}

func normalizeAccountSingleValue(raw string, fallback string, allowed ...string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	for _, item := range allowed {
		if value == item {
			return value
		}
	}
	return fallback
}

func normalizeAccountLimit(raw string) int {
	switch strings.TrimSpace(raw) {
	case "20":
		return 20
	case "40":
		return 40
	case "100":
		return 100
	default:
		return 10
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

func splitCSVValues(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		values = append(values, trimmed)
	}
	return values
}
