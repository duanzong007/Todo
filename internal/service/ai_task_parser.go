package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"todo/internal/domain"
	"todo/internal/repository"
)

const aiTaskChatCompletionsPath = "/chat/completions"

type AITaskParser struct {
	endpoint string
	apiKey   string
	model    string
	client   *http.Client
	location *time.Location
}

type aiTaskChatRequest struct {
	Model          string              `json:"model"`
	Messages       []aiTaskChatMessage `json:"messages"`
	Temperature    float64             `json:"temperature"`
	MaxTokens      int                 `json:"max_tokens"`
	ResponseFormat map[string]string   `json:"response_format,omitempty"`
}

type aiTaskChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type aiTaskChatResponse struct {
	Choices []struct {
		Message aiTaskChatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type aiTaskParsedPayload struct {
	Type          string   `json:"type"`
	Title         string   `json:"title"`
	Importance    int      `json:"importance"`
	ScheduleMode  string   `json:"schedule_mode"`
	ScheduledFor  string   `json:"scheduled_for"`
	BatchStart    string   `json:"batch_start"`
	BatchEnd      string   `json:"batch_end"`
	BatchWeekdays []string `json:"batch_weekdays"`
	Deadline      string   `json:"deadline"`
	Note          string   `json:"note"`
}

type AITaskPrefill struct {
	Task          repository.TaskInput
	ScheduleMode  string
	BatchStart    *time.Time
	BatchEnd      *time.Time
	BatchWeekdays []string
}

func NewAITaskParser(rawEndpoint, apiKey, model string, location *time.Location) (*AITaskParser, error) {
	endpoint, err := normalizeAITaskEndpoint(rawEndpoint)
	if err != nil {
		return nil, err
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("ai task api key is empty")
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = "deepseek-v3"
	}
	if location == nil {
		location = time.Local
	}

	return &AITaskParser{
		endpoint: endpoint,
		apiKey:   apiKey,
		model:    model,
		client: &http.Client{
			Timeout: 12 * time.Second,
		},
		location: location,
	}, nil
}

func (p *AITaskParser) Parse(ctx context.Context, input string, now time.Time) (AITaskPrefill, error) {
	if p == nil || p.client == nil || p.endpoint == "" || p.apiKey == "" {
		return AITaskPrefill{}, fmt.Errorf("AI 解析没有配置")
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return AITaskPrefill{}, fmt.Errorf("AI 输入不能为空")
	}
	if now.IsZero() {
		now = time.Now().In(p.location)
	}

	requestBody := aiTaskChatRequest{
		Model:       p.model,
		Temperature: 0,
		MaxTokens:   700,
		ResponseFormat: map[string]string{
			"type": "json_object",
		},
		Messages: []aiTaskChatMessage{
			{Role: "system", Content: p.systemPrompt(now.In(p.location))},
			{Role: "user", Content: input},
		},
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return AITaskPrefill{}, fmt.Errorf("build ai task request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return AITaskPrefill{}, fmt.Errorf("build ai task request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+p.apiKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := p.client.Do(request)
	if err != nil {
		return AITaskPrefill{}, fmt.Errorf("请求 AI 解析失败: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return AITaskPrefill{}, fmt.Errorf("读取 AI 解析结果失败: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return AITaskPrefill{}, fmt.Errorf("AI 解析接口状态异常: %s", response.Status)
	}

	var chatResponse aiTaskChatResponse
	if err := json.Unmarshal(responseBody, &chatResponse); err != nil {
		return AITaskPrefill{}, fmt.Errorf("解析 AI 响应失败: %w", err)
	}
	if chatResponse.Error != nil && strings.TrimSpace(chatResponse.Error.Message) != "" {
		return AITaskPrefill{}, fmt.Errorf("AI 解析失败: %s", strings.TrimSpace(chatResponse.Error.Message))
	}
	if len(chatResponse.Choices) == 0 {
		return AITaskPrefill{}, fmt.Errorf("AI 没有返回解析结果")
	}

	content := strings.TrimSpace(chatResponse.Choices[0].Message.Content)
	if content == "" {
		return AITaskPrefill{}, fmt.Errorf("AI 没有返回解析结果")
	}
	return p.parseContent(content)
}

func (p *AITaskParser) Endpoint() string {
	if p == nil {
		return ""
	}
	return p.endpoint
}

func (p *AITaskParser) systemPrompt(now time.Time) string {
	return fmt.Sprintf(`你是 Todo 应用的任务预填助手。现在时间是 %s，时区是 %s。
把用户的一句话解析成一条任务预填信息，只输出 JSON object，不要 Markdown，不要解释。
JSON 字段固定为：
{
  "type": "todo|schedule|ddl",
  "title": "任务标题",
  "importance": 1,
  "schedule_mode": "single|batch",
  "scheduled_for": "YYYY-MM-DD 或 null",
  "batch_start": "YYYY-MM-DD 或 null",
  "batch_end": "YYYY-MM-DD 或 null",
  "batch_weekdays": ["mon","tue","wed","thu","fri","sat","sun"],
  "deadline": "YYYY-MM-DDTHH:mm 或 null",
  "note": ""
}
规则：
1. todo 表示没有明确日期、时间、截止要求的普通待办。
2. schedule 表示某天要做、某天安排、会议、课程、考试等日程。
3. 单次日程用 schedule_mode=single，scheduled_for 填日期；如果出现具体几点，把时间写进 note，不要填 deadline。
4. 重复日程用 schedule_mode=batch，例如“下周每天背单词”“未来七天每天运动”；batch_start/batch_end 填区间，batch_weekdays 填要创建的星期。每天就是七个值全填。
5. ddl 表示明确有截止、之前、前、到期、deadline、DDL 等语义的任务，deadline 填本地时间；只有日期没有时间时用 23:59。
6. importance 默认 2；“重要/紧急”用 4；“非常重要/最高/必须”用 5；“不急/有空”用 1。
7. title 去掉日期、截止、重复、重要等级等控制信息，保留真正要做的事情。`, now.Format("2006-01-02 15:04"), p.location.String())
}

func (p *AITaskParser) parseContent(content string) (AITaskPrefill, error) {
	content = strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(content, "```"), "```json"))
	var payload aiTaskParsedPayload
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return AITaskPrefill{}, fmt.Errorf("AI 返回格式不正确")
	}

	title := strings.TrimSpace(payload.Title)
	if title == "" {
		return AITaskPrefill{}, fmt.Errorf("AI 没有解析出标题")
	}

	if payload.Importance == 0 {
		payload.Importance = 2
	}
	importance, err := normalizeImportanceValue(payload.Importance)
	if err != nil {
		return AITaskPrefill{}, err
	}

	taskType := strings.TrimSpace(strings.ToLower(payload.Type))
	if taskType == "schedule_batch" {
		taskType = string(domain.TaskTypeSchedule)
		payload.ScheduleMode = "batch"
	}

	input := repository.TaskInput{
		Title:      title,
		Note:       strings.TrimSpace(payload.Note),
		Type:       domain.TaskType(taskType),
		Importance: importance,
		Metadata: map[string]any{
			"creator": "ai_prefill",
		},
	}

	switch input.Type {
	case domain.TaskTypeTodo:
		return AITaskPrefill{Task: input}, nil
	case domain.TaskTypeSchedule:
		if strings.EqualFold(strings.TrimSpace(payload.ScheduleMode), "batch") {
			return p.parseScheduleBatchPayload(input, payload)
		}
		rawDate := strings.TrimSpace(payload.ScheduledFor)
		if rawDate == "" || strings.EqualFold(rawDate, "null") {
			return AITaskPrefill{}, fmt.Errorf("AI 没有解析出日程日期")
		}
		scheduledFor, err := time.ParseInLocation("2006-01-02", rawDate, p.location)
		if err != nil {
			return AITaskPrefill{}, fmt.Errorf("AI 返回的日程日期格式不正确")
		}
		input.ScheduledFor = &scheduledFor
		return AITaskPrefill{Task: input, ScheduleMode: "single"}, nil
	case domain.TaskTypeDDL:
		rawDeadline := strings.TrimSpace(payload.Deadline)
		if rawDeadline == "" || strings.EqualFold(rawDeadline, "null") {
			return AITaskPrefill{}, fmt.Errorf("AI 没有解析出截止时间")
		}
		deadline, err := parseAITaskDeadline(rawDeadline, p.location)
		if err != nil {
			return AITaskPrefill{}, fmt.Errorf("AI 返回的截止时间格式不正确")
		}
		input.Deadline = &deadline
		return AITaskPrefill{Task: input}, nil
	default:
		return AITaskPrefill{}, fmt.Errorf("AI 返回的任务类型不正确")
	}
}

func (p *AITaskParser) parseScheduleBatchPayload(input repository.TaskInput, payload aiTaskParsedPayload) (AITaskPrefill, error) {
	rawStart := strings.TrimSpace(payload.BatchStart)
	rawEnd := strings.TrimSpace(payload.BatchEnd)
	if rawStart == "" || rawEnd == "" || strings.EqualFold(rawStart, "null") || strings.EqualFold(rawEnd, "null") {
		return AITaskPrefill{}, fmt.Errorf("AI 没有解析出批量日程日期范围")
	}
	start, err := time.ParseInLocation("2006-01-02", rawStart, p.location)
	if err != nil {
		return AITaskPrefill{}, fmt.Errorf("AI 返回的批量起始日期格式不正确")
	}
	end, err := time.ParseInLocation("2006-01-02", rawEnd, p.location)
	if err != nil {
		return AITaskPrefill{}, fmt.Errorf("AI 返回的批量截止日期格式不正确")
	}
	if end.Before(start) {
		return AITaskPrefill{}, fmt.Errorf("AI 返回的批量截止日期不能早于起始日期")
	}

	weekdays := normalizeAIWeekdayValues(payload.BatchWeekdays)
	if len(weekdays) == 0 {
		weekdays = weekdayValuesBetween(start, end)
	}
	if len(weekdays) == 0 {
		return AITaskPrefill{}, fmt.Errorf("AI 没有解析出批量日程星期")
	}

	return AITaskPrefill{
		Task:          input,
		ScheduleMode:  "batch",
		BatchStart:    &start,
		BatchEnd:      &end,
		BatchWeekdays: weekdays,
	}, nil
}

func normalizeAIWeekdayValues(values []string) []string {
	seen := map[string]bool{}
	ordered := make([]string, 0, 7)
	for _, raw := range values {
		value := strings.TrimSpace(strings.ToLower(raw))
		switch value {
		case "monday", "mon", "周一", "星期一", "1":
			value = "mon"
		case "tuesday", "tue", "周二", "星期二", "2":
			value = "tue"
		case "wednesday", "wed", "周三", "星期三", "3":
			value = "wed"
		case "thursday", "thu", "周四", "星期四", "4":
			value = "thu"
		case "friday", "fri", "周五", "星期五", "5":
			value = "fri"
		case "saturday", "sat", "周六", "星期六", "6":
			value = "sat"
		case "sunday", "sun", "周日", "周天", "星期日", "星期天", "7", "0":
			value = "sun"
		default:
			continue
		}
		if !seen[value] {
			seen[value] = true
			ordered = append(ordered, value)
		}
	}
	return ordered
}

func weekdayValuesBetween(start, end time.Time) []string {
	labels := map[time.Weekday]string{
		time.Monday:    "mon",
		time.Tuesday:   "tue",
		time.Wednesday: "wed",
		time.Thursday:  "thu",
		time.Friday:    "fri",
		time.Saturday:  "sat",
		time.Sunday:    "sun",
	}
	seen := map[string]bool{}
	values := make([]string, 0, 7)
	for current := start; !current.After(end); current = current.AddDate(0, 0, 1) {
		value := labels[current.Weekday()]
		if !seen[value] {
			seen[value] = true
			values = append(values, value)
		}
	}
	return values
}

func parseAITaskDeadline(raw string, location *time.Location) (time.Time, error) {
	if deadline, err := time.ParseInLocation("2006-01-02T15:04", raw, location); err == nil {
		return deadline, nil
	}
	if deadline, err := time.Parse(time.RFC3339, raw); err == nil {
		return deadline.In(location), nil
	}
	dateOnly, err := time.ParseInLocation("2006-01-02", raw, location)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(dateOnly.Year(), dateOnly.Month(), dateOnly.Day(), 23, 59, 0, 0, location), nil
}

func normalizeAITaskEndpoint(rawEndpoint string) (string, error) {
	trimmed := strings.TrimSpace(rawEndpoint)
	if trimmed == "" {
		return "", fmt.Errorf("ai task api url is empty")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("parse ai task api url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("ai task api url must use http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("ai task api url host is empty")
	}
	path := strings.TrimRight(parsed.Path, "/")
	if path == "" {
		parsed.Path = aiTaskChatCompletionsPath
	} else if !strings.HasSuffix(path, aiTaskChatCompletionsPath) {
		parsed.Path = path + aiTaskChatCompletionsPath
	} else {
		parsed.Path = path
	}
	return parsed.String(), nil
}
