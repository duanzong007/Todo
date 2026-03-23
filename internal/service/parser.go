package service

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"todo/internal/domain"
	"todo/internal/repository"
)

var errEmptyInput = errors.New("input cannot be empty")

var (
	pickupCodeRegex     = regexp.MustCompile(`(?:凭取件码|取件码|凭)[:：\s]*(?:为|是)?[:：\s]*([A-Za-z0-9-]+)`)
	cabinetNumberRegex  = regexp.MustCompile(`(\d+号柜)`)
	smsVendorMarkRegex  = regexp.MustCompile(`【[^】]+】`)
	explicitDateRegex   = regexp.MustCompile(`(?:(\d{4})[年/-])?(\d{1,2})[月/-](\d{1,2})(?:日|号)?`)
	clockTimeRegex      = regexp.MustCompile(`(?i)(凌晨|早上|上午|中午|下午|晚上)?\s*(\d{1,2})(?:[:：点时](\d{1,2}))?\s*(?:分)?`)
	relativeDateRegex   = regexp.MustCompile(`(今天|明天|后天)`)
	weekdayDateRegex    = regexp.MustCompile(`((?:下周|本周)?周[一二三四五六日天])`)
	dayPeriodRegex      = regexp.MustCompile(`(今天上午|今天下午|今天晚上|今天中午|今天早上|今早|今晚|上午|下午|晚上|中午)`)
	whitespaceRegex     = regexp.MustCompile(`\s+`)
	deadlineKeywordList = []string{"截止", "ddl", "due", "交", "提交", "上交", "完成", "提交论文", "交作业", "交论文", "交报告"}
)

type ParsedTask struct {
	SourceType     domain.SourceType
	SourceSummary  string
	SourceMetadata map[string]any
	Task           repository.TaskInput
}

type TextParser struct {
	location *time.Location
}

func NewTextParser(location *time.Location) *TextParser {
	return &TextParser{location: location}
}

func (p *TextParser) Parse(input string, now time.Time) (ParsedTask, error) {
	now = now.In(p.location)
	cleaned := normalizeWhitespace(input)
	if cleaned == "" {
		return ParsedTask{}, errEmptyInput
	}

	if parsed, ok := p.parsePickupSMS(cleaned); ok {
		return parsed, nil
	}

	match, ok := p.extractDate(cleaned, now)
	title := cleaned
	taskType := domain.TaskTypeTodo
	var scheduledFor *time.Time
	var deadline *time.Time

	metadata := map[string]any{
		"raw_input": cleaned,
		"parser":    "text",
	}

	if ok {
		title = cleanTaskTitle(cleaned, match.Raw)
		if title == "" {
			title = cleaned
		}

		metadata["date_phrase"] = match.Raw
		metadata["parsed_date"] = match.Date.Format("2006-01-02")

		if isDeadlineText(cleaned) {
			taskType = domain.TaskTypeDDL
			dateValue := endOfDeadlineDay(match.Date, p.location)
			if clock, found := extractClockTime(strings.Replace(cleaned, match.Raw, "", 1), p.location); found {
				dateValue = combineDateAndClock(match.Date, clock, p.location)
				title = cleanTaskTitle(title, clock.Raw)
				metadata["parsed_time"] = dateValue.In(p.location).Format("15:04")
			}
			deadline = &dateValue
		} else {
			taskType = domain.TaskTypeSchedule
			dateValue := match.Date
			scheduledFor = &dateValue
		}
	}

	return ParsedTask{
		SourceType:    domain.SourceTypeManualText,
		SourceSummary: title,
		SourceMetadata: map[string]any{
			"parser": "text",
		},
		Task: repository.TaskInput{
			Title:        title,
			Note:         "",
			Type:         taskType,
			Importance:   domain.DefaultTaskImportance,
			ScheduledFor: scheduledFor,
			Deadline:     deadline,
			Metadata:     metadata,
		},
	}, nil
}

func (p *TextParser) ParseSMSBatch(input string, now time.Time) ([]ParsedTask, error) {
	_ = now
	segments := splitSMSMessages(input)
	if len(segments) == 0 {
		return nil, errEmptyInput
	}

	tasks := make([]ParsedTask, 0, len(segments))
	for _, segment := range segments {
		cleaned := normalizeWhitespace(segment)
		if cleaned == "" {
			continue
		}
		parsed, ok := p.parsePickupSMS(cleaned)
		if !ok {
			continue
		}
		tasks = append(tasks, parsed)
	}

	if len(tasks) == 0 {
		return nil, fmt.Errorf("暂时只支持解析这些取件短信")
	}

	return tasks, nil
}

type dateMatch struct {
	Raw  string
	Date time.Time
}

type clockTime struct {
	Raw    string
	Hour   int
	Minute int
}

func (p *TextParser) parsePickupSMS(input string) (ParsedTask, bool) {
	if !strings.Contains(input, "取件码") && !strings.Contains(input, "驿站") && !strings.Contains(input, "快递") {
		return ParsedTask{}, false
	}

	title, metadata, ok := extractPickupTaskTitle(input)
	if !ok {
		return ParsedTask{}, false
	}
	metadata["raw_input"] = input
	metadata["parser"] = "pickup_sms"

	return ParsedTask{
		SourceType:    domain.SourceTypeSMSPaste,
		SourceSummary: title,
		SourceMetadata: map[string]any{
			"parser": "pickup_sms",
		},
		Task: repository.TaskInput{
			Title:      title,
			Note:       "",
			Type:       domain.TaskTypeTodo,
			Importance: 2,
			Metadata:   metadata,
		},
	}, true
}

func extractPickupTaskTitle(input string) (string, map[string]any, bool) {
	codeMatch := pickupCodeRegex.FindStringSubmatch(input)
	if len(codeMatch) < 2 {
		return "", nil, false
	}
	code := strings.TrimSpace(codeMatch[1])
	if code == "" {
		return "", nil, false
	}

	metadata := map[string]any{
		"pickup_code": code,
	}

	cabinetMatch := cabinetNumberRegex.FindStringSubmatch(input)
	if len(cabinetMatch) > 1 {
		cabinet := strings.TrimSpace(cabinetMatch[1])
		metadata["pickup_kind"] = "cabinet"
		metadata["cabinet_number"] = cabinet
		return cabinet + " " + code, metadata, true
	}

	if strings.Contains(input, "驿站") || strings.Contains(input, "取运单尾号") {
		metadata["pickup_kind"] = "station"
		return "驿站：" + code, metadata, true
	}

	return "", nil, false
}

func splitSMSMessages(input string) []string {
	normalized := strings.ReplaceAll(input, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	trimmed := strings.TrimSpace(normalized)
	if trimmed == "" {
		return nil
	}

	indices := smsVendorMarkRegex.FindAllStringIndex(trimmed, -1)
	if len(indices) <= 1 {
		lines := strings.Split(trimmed, "\n")
		if len(lines) <= 1 {
			return []string{trimmed}
		}

		messages := make([]string, 0, len(lines))
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			messages = append(messages, line)
		}
		if len(messages) > 0 {
			return messages
		}
		return []string{trimmed}
	}

	messages := make([]string, 0, len(indices))
	for index, bounds := range indices {
		start := bounds[0]
		end := len(trimmed)
		if index+1 < len(indices) {
			end = indices[index+1][0]
		}
		segment := strings.TrimSpace(trimmed[start:end])
		if segment == "" {
			continue
		}
		messages = append(messages, segment)
	}
	return messages
}

func (p *TextParser) extractDate(input string, now time.Time) (dateMatch, bool) {
	if matched := explicitDateRegex.FindStringSubmatch(input); len(matched) > 0 {
		dateValue, err := parseExplicitDateMatch(matched, now, p.location)
		if err == nil {
			return dateMatch{Raw: matched[0], Date: dateValue}, true
		}
	}

	if matched := relativeDateRegex.FindStringSubmatch(input); len(matched) > 0 {
		return dateMatch{Raw: matched[0], Date: parseRelativeDate(matched[0], now)}, true
	}

	if matched := weekdayDateRegex.FindStringSubmatch(input); len(matched) > 0 {
		dateValue, err := parseWeekdayMatch(matched[1], now)
		if err == nil {
			return dateMatch{Raw: matched[1], Date: dateValue}, true
		}
	}

	if matched := dayPeriodRegex.FindStringSubmatch(input); len(matched) > 0 {
		return dateMatch{Raw: matched[0], Date: normalizeDateInLocation(now, p.location)}, true
	}

	return dateMatch{}, false
}

func parseExplicitDateMatch(matched []string, now time.Time, location *time.Location) (time.Time, error) {
	year := now.Year()
	var err error
	if matched[1] != "" {
		year, err = parsePositiveInt(matched[1])
		if err != nil {
			return time.Time{}, err
		}
	}

	month, err := parsePositiveInt(matched[2])
	if err != nil {
		return time.Time{}, err
	}
	day, err := parsePositiveInt(matched[3])
	if err != nil {
		return time.Time{}, err
	}

	dateValue := time.Date(year, time.Month(month), day, 0, 0, 0, 0, location)
	if dateValue.Year() != year || dateValue.Month() != time.Month(month) || dateValue.Day() != day {
		return time.Time{}, fmt.Errorf("invalid date: %s-%02d-%02d", matched[1], month, day)
	}
	if matched[1] == "" && dateValue.Before(normalizeDateInLocation(now, location)) {
		dateValue = dateValue.AddDate(1, 0, 0)
	}
	return normalizeDateInLocation(dateValue, location), nil
}

func parseRelativeDate(keyword string, now time.Time) time.Time {
	switch keyword {
	case "今天":
		return normalizeDateInLocation(now, now.Location())
	case "明天":
		return normalizeDateInLocation(now.AddDate(0, 0, 1), now.Location())
	case "后天":
		return normalizeDateInLocation(now.AddDate(0, 0, 2), now.Location())
	default:
		return normalizeDateInLocation(now, now.Location())
	}
}

func parseWeekdayMatch(keyword string, now time.Time) (time.Time, error) {
	prefix := ""
	daySymbol := keyword
	if strings.HasPrefix(keyword, "下周") {
		prefix = "下周"
		daySymbol = strings.TrimPrefix(keyword, "下周")
	} else if strings.HasPrefix(keyword, "本周") {
		prefix = "本周"
		daySymbol = strings.TrimPrefix(keyword, "本周")
	} else if strings.HasPrefix(keyword, "周") {
		daySymbol = strings.TrimPrefix(keyword, "周")
	}

	targetWeekday, err := chineseWeekday(daySymbol)
	if err != nil {
		return time.Time{}, err
	}

	base := normalizeDateInLocation(now, now.Location())
	currentWeekday := weekdayToChineseIndex(base.Weekday())
	targetIndex := weekdayToChineseIndex(targetWeekday)

	switch prefix {
	case "下周":
		daysUntilMonday := (7 - currentWeekday) % 7
		if daysUntilMonday == 0 {
			daysUntilMonday = 7
		}
		nextMonday := base.AddDate(0, 0, daysUntilMonday)
		return nextMonday.AddDate(0, 0, targetIndex), nil
	case "本周":
		startOfWeek := base.AddDate(0, 0, -currentWeekday)
		return startOfWeek.AddDate(0, 0, targetIndex), nil
	default:
		diff := targetIndex - currentWeekday
		if diff < 0 {
			diff += 7
		}
		return base.AddDate(0, 0, diff), nil
	}
}

func cleanTaskTitle(input, matched string) string {
	title := strings.Replace(input, matched, "", 1)
	title = strings.TrimSpace(title)
	title = strings.Trim(title, "，,。；;：:()（）[]【】")
	title = strings.TrimPrefix(title, "要")
	title = strings.TrimPrefix(title, "得")
	title = strings.TrimSpace(title)
	if title == "" {
		return input
	}
	return title
}

func isDeadlineText(input string) bool {
	lower := strings.ToLower(input)
	for _, keyword := range deadlineKeywordList {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func normalizeWhitespace(input string) string {
	trimmed := strings.TrimSpace(input)
	return whitespaceRegex.ReplaceAllString(trimmed, " ")
}

func parsePositiveInt(value string) (int, error) {
	var number int
	_, err := fmt.Sscanf(value, "%d", &number)
	if err != nil {
		return 0, err
	}
	if number <= 0 {
		return 0, fmt.Errorf("invalid integer: %s", value)
	}
	return number, nil
}

func chineseWeekday(symbol string) (time.Weekday, error) {
	switch symbol {
	case "一":
		return time.Monday, nil
	case "二":
		return time.Tuesday, nil
	case "三":
		return time.Wednesday, nil
	case "四":
		return time.Thursday, nil
	case "五":
		return time.Friday, nil
	case "六":
		return time.Saturday, nil
	case "日", "天":
		return time.Sunday, nil
	default:
		return time.Sunday, fmt.Errorf("unsupported weekday symbol: %s", symbol)
	}
}

func weekdayToChineseIndex(day time.Weekday) int {
	switch day {
	case time.Monday:
		return 0
	case time.Tuesday:
		return 1
	case time.Wednesday:
		return 2
	case time.Thursday:
		return 3
	case time.Friday:
		return 4
	case time.Saturday:
		return 5
	default:
		return 6
	}
}

func normalizeDateInLocation(value time.Time, location *time.Location) time.Time {
	local := value.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}

func extractClockTime(input string, location *time.Location) (clockTime, bool) {
	matched := clockTimeRegex.FindStringSubmatch(input)
	if len(matched) == 0 {
		return clockTime{}, false
	}

	hour, err := parsePositiveInt(matched[2])
	if err != nil {
		return clockTime{}, false
	}
	minute := 0
	if matched[3] != "" {
		minute, err = parsePositiveIntAllowZero(matched[3])
		if err != nil {
			return clockTime{}, false
		}
	}

	switch matched[1] {
	case "凌晨":
		if hour == 12 {
			hour = 0
		}
	case "下午", "晚上":
		if hour < 12 {
			hour += 12
		}
	case "中午":
		if hour < 11 {
			hour += 12
		}
	}

	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return clockTime{}, false
	}

	_ = location
	return clockTime{Raw: matched[0], Hour: hour, Minute: minute}, true
}

func combineDateAndClock(dateValue time.Time, clock clockTime, location *time.Location) time.Time {
	localDate := normalizeDateInLocation(dateValue, location)
	return time.Date(localDate.Year(), localDate.Month(), localDate.Day(), clock.Hour, clock.Minute, 0, 0, location)
}

func endOfDeadlineDay(dateValue time.Time, location *time.Location) time.Time {
	localDate := normalizeDateInLocation(dateValue, location)
	return time.Date(localDate.Year(), localDate.Month(), localDate.Day(), 23, 59, 0, 0, location)
}

func parsePositiveIntAllowZero(value string) (int, error) {
	var number int
	_, err := fmt.Sscanf(value, "%d", &number)
	if err != nil {
		return 0, err
	}
	if number < 0 {
		return 0, fmt.Errorf("invalid integer: %s", value)
	}
	return number, nil
}
