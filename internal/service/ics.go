package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"todo/internal/domain"
	"todo/internal/repository"
)

type ICSImporter struct {
	location    *time.Location
	horizonDays int
}

type ICSImportResult struct {
	Tasks         []repository.TaskInput
	SourceSummary string
	SourceMeta    map[string]any
}

type icsEvent struct {
	UID      string
	Summary  string
	Start    time.Time
	End      *time.Time
	AllDay   bool
	TimeZone string
	RRule    string
	Exdates  []time.Time
}

func NewICSImporter(location *time.Location, horizonDays int) *ICSImporter {
	return &ICSImporter{
		location:    location,
		horizonDays: horizonDays,
	}
}

func (i *ICSImporter) Parse(filename string, body []byte, now time.Time) (ICSImportResult, error) {
	lines := unfoldICSLines(string(body))
	events, err := parseICSEvents(lines, i.location)
	if err != nil {
		return ICSImportResult{}, err
	}

	startDate := normalizeDateInLocation(now, i.location)
	endDate := startDate.AddDate(0, 0, i.horizonDays)

	var tasks []repository.TaskInput
	for _, event := range events {
		occurrences := expandOccurrences(event, startDate, endDate, i.location)
		for _, occurrence := range occurrences {
			scheduledDate := normalizeDateInLocation(occurrence, i.location)
			metadata := map[string]any{
				"ics_uid":              event.UID,
				"ics_filename":         filename,
				"ics_occurrence_start": occurrence.In(i.location).Format(time.RFC3339),
				"ics_rrule":            event.RRule,
				"ics_timezone":         event.TimeZone,
				"ics_all_day":          event.AllDay,
			}
			tasks = append(tasks, repository.TaskInput{
				Title:        event.Summary,
				Note:         formatICSNote(event, occurrence, i.location),
				Type:         domain.TaskTypeSchedule,
				ScheduledFor: &scheduledDate,
				Metadata:     metadata,
			})
		}
	}

	return ICSImportResult{
		Tasks:         tasks,
		SourceSummary: fmt.Sprintf("ICS 导入: %s", filename),
		SourceMeta: map[string]any{
			"filename":     filename,
			"horizon_days": i.horizonDays,
			"event_count":  len(events),
		},
	}, nil
}

func unfoldICSLines(raw string) []string {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	parts := strings.Split(normalized, "\n")

	var lines []string
	for _, part := range parts {
		if part == "" {
			continue
		}

		if len(lines) > 0 && (strings.HasPrefix(part, " ") || strings.HasPrefix(part, "\t")) {
			lines[len(lines)-1] += strings.TrimLeft(part, " \t")
			continue
		}

		lines = append(lines, part)
	}
	return lines
}

func parseICSEvents(lines []string, fallbackLocation *time.Location) ([]icsEvent, error) {
	var (
		events  []icsEvent
		current *icsEvent
		inEvent bool
	)

	for _, line := range lines {
		switch line {
		case "BEGIN:VEVENT":
			current = &icsEvent{}
			inEvent = true
			continue
		case "END:VEVENT":
			if current != nil && !current.Start.IsZero() {
				if current.UID == "" {
					current.UID = hashString(current.Summary + current.Start.Format(time.RFC3339))
				}
				if current.Summary == "" {
					current.Summary = "未命名日程"
				}
				if current.TimeZone == "" {
					current.TimeZone = fallbackLocation.String()
				}
				events = append(events, *current)
			}
			current = nil
			inEvent = false
			continue
		}

		if !inEvent || current == nil {
			continue
		}

		name, params, value, err := parseICSProperty(line)
		if err != nil {
			return nil, err
		}

		switch name {
		case "UID":
			current.UID = value
		case "SUMMARY":
			current.Summary = unescapeICSText(value)
		case "RRULE":
			current.RRule = value
		case "DTSTART":
			dateValue, allDay, err := parseICSDateTime(value, params, fallbackLocation)
			if err != nil {
				return nil, err
			}
			current.Start = dateValue
			current.AllDay = allDay
			if tzid := params["TZID"]; tzid != "" {
				current.TimeZone = tzid
			} else {
				current.TimeZone = dateValue.Location().String()
			}
		case "DTEND":
			dateValue, _, err := parseICSDateTime(value, params, fallbackLocation)
			if err != nil {
				return nil, err
			}
			current.End = &dateValue
		case "EXDATE":
			values := strings.Split(value, ",")
			for _, item := range values {
				dateValue, _, err := parseICSDateTime(item, params, fallbackLocation)
				if err != nil {
					return nil, err
				}
				current.Exdates = append(current.Exdates, dateValue)
			}
		}
	}

	return events, nil
}

func parseICSProperty(line string) (string, map[string]string, string, error) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return "", nil, "", fmt.Errorf("invalid ICS line: %s", line)
	}

	head := parts[0]
	value := parts[1]
	headParts := strings.Split(head, ";")
	name := strings.ToUpper(headParts[0])
	params := make(map[string]string, len(headParts)-1)

	for _, segment := range headParts[1:] {
		param := strings.SplitN(segment, "=", 2)
		if len(param) != 2 {
			continue
		}
		params[strings.ToUpper(param[0])] = param[1]
	}

	return name, params, value, nil
}

func parseICSDateTime(value string, params map[string]string, fallback *time.Location) (time.Time, bool, error) {
	location := fallback
	if tzid := params["TZID"]; tzid != "" {
		loaded, err := time.LoadLocation(tzid)
		if err == nil {
			location = loaded
		}
	}

	if params["VALUE"] == "DATE" || len(value) == 8 {
		parsed, err := time.ParseInLocation("20060102", value, location)
		if err != nil {
			return time.Time{}, false, err
		}
		return parsed, true, nil
	}

	layouts := []string{"20060102T150405", "20060102T1504"}
	if strings.HasSuffix(value, "Z") {
		layouts = []string{"20060102T150405Z", "20060102T1504Z"}
		location = time.UTC
	}

	for _, layout := range layouts {
		var (
			parsed time.Time
			err    error
		)

		if location == time.UTC {
			parsed, err = time.Parse(layout, value)
		} else {
			parsed, err = time.ParseInLocation(layout, value, location)
		}
		if err == nil {
			return parsed.In(location), false, nil
		}
	}

	return time.Time{}, false, fmt.Errorf("unsupported ICS datetime: %s", value)
}

func expandOccurrences(event icsEvent, startDate, endDate time.Time, fallbackLocation *time.Location) []time.Time {
	if event.Start.IsZero() {
		return nil
	}

	if event.RRule == "" {
		if !event.Start.Before(startDate) && !event.Start.After(endDate) {
			return []time.Time{event.Start}
		}
		return nil
	}

	options := parseRRule(event.RRule)
	frequency := strings.ToUpper(options["FREQ"])
	if frequency == "" {
		if !event.Start.Before(startDate) && !event.Start.After(endDate) {
			return []time.Time{event.Start}
		}
		return nil
	}

	interval := 1
	if raw := options["INTERVAL"]; raw != "" {
		if parsed, err := parsePositiveInt(raw); err == nil && parsed > 0 {
			interval = parsed
		}
	}

	countLimit := 0
	if raw := options["COUNT"]; raw != "" {
		if parsed, err := parsePositiveInt(raw); err == nil && parsed > 0 {
			countLimit = parsed
		}
	}

	var until *time.Time
	if raw := options["UNTIL"]; raw != "" {
		if parsed, _, err := parseICSDateTime(raw, map[string]string{}, fallbackLocation); err == nil {
			untilValue := parsed
			until = &untilValue
		}
	}

	var occurrences []time.Time
	generatedCount := 0
	appendOccurrence := func(candidate time.Time) bool {
		if candidate.Before(event.Start) {
			return true
		}
		generatedCount++
		if countLimit > 0 && generatedCount > countLimit {
			return false
		}
		if until != nil && candidate.After(*until) {
			return false
		}
		if candidate.After(endDate) {
			return false
		}
		if isExcludedDate(candidate, event.Exdates, event.AllDay, fallbackLocation) {
			return true
		}
		if !candidate.Before(startDate) {
			occurrences = append(occurrences, candidate)
		}
		return true
	}

	switch frequency {
	case "DAILY":
		for step := 0; ; step += interval {
			candidate := event.Start.AddDate(0, 0, step)
			if !appendOccurrence(candidate) {
				break
			}
		}
	case "MONTHLY":
		for step := 0; ; step += interval {
			candidate := event.Start.AddDate(0, step, 0)
			if !appendOccurrence(candidate) {
				break
			}
		}
	case "WEEKLY":
		weekdays := parseBYDAY(options["BYDAY"], event.Start.Weekday())
		startOfWeek := normalizeDateInLocation(event.Start, fallbackLocation).AddDate(0, 0, -weekdayToChineseIndex(event.Start.In(fallbackLocation).Weekday()))

		for weekOffset := 0; ; weekOffset += interval {
			weekBase := startOfWeek.AddDate(0, 0, weekOffset*7)
			var candidates []time.Time
			for _, weekday := range weekdays {
				candidateDate := weekBase.AddDate(0, 0, weekdayToChineseIndex(weekday))
				candidate := time.Date(
					candidateDate.Year(),
					candidateDate.Month(),
					candidateDate.Day(),
					event.Start.In(fallbackLocation).Hour(),
					event.Start.In(fallbackLocation).Minute(),
					event.Start.In(fallbackLocation).Second(),
					0,
					event.Start.Location(),
				)
				candidates = append(candidates, candidate)
			}

			sort.Slice(candidates, func(a, b int) bool {
				return candidates[a].Before(candidates[b])
			})

			for _, candidate := range candidates {
				if candidate.Before(event.Start) {
					continue
				}
				if !appendOccurrence(candidate) {
					return occurrences
				}
			}
		}
	default:
		if !event.Start.Before(startDate) && !event.Start.After(endDate) {
			return []time.Time{event.Start}
		}
	}

	return occurrences
}

func parseRRule(raw string) map[string]string {
	options := map[string]string{}
	for _, segment := range strings.Split(raw, ";") {
		part := strings.SplitN(segment, "=", 2)
		if len(part) != 2 {
			continue
		}
		options[strings.ToUpper(part[0])] = part[1]
	}
	return options
}

func parseBYDAY(raw string, fallback time.Weekday) []time.Weekday {
	if raw == "" {
		return []time.Weekday{fallback}
	}

	var weekdays []time.Weekday
	for _, segment := range strings.Split(raw, ",") {
		switch strings.ToUpper(strings.TrimSpace(segment)) {
		case "MO":
			weekdays = append(weekdays, time.Monday)
		case "TU":
			weekdays = append(weekdays, time.Tuesday)
		case "WE":
			weekdays = append(weekdays, time.Wednesday)
		case "TH":
			weekdays = append(weekdays, time.Thursday)
		case "FR":
			weekdays = append(weekdays, time.Friday)
		case "SA":
			weekdays = append(weekdays, time.Saturday)
		case "SU":
			weekdays = append(weekdays, time.Sunday)
		}
	}

	if len(weekdays) == 0 {
		return []time.Weekday{fallback}
	}
	return weekdays
}

func isExcludedDate(candidate time.Time, exdates []time.Time, allDay bool, fallbackLocation *time.Location) bool {
	for _, excluded := range exdates {
		if allDay {
			if normalizeDateInLocation(candidate, fallbackLocation).Equal(normalizeDateInLocation(excluded, fallbackLocation)) {
				return true
			}
			continue
		}
		if candidate.Equal(excluded) {
			return true
		}
	}
	return false
}

func formatICSNote(event icsEvent, occurrence time.Time, location *time.Location) string {
	if event.AllDay {
		return "全天"
	}

	start := occurrence.In(location)
	if event.End == nil {
		return start.Format("15:04")
	}

	duration := event.End.Sub(event.Start)
	end := start.Add(duration)
	return fmt.Sprintf("%s - %s", start.Format("15:04"), end.Format("15:04"))
}

func unescapeICSText(value string) string {
	replacer := strings.NewReplacer(`\,`, ",", `\;`, ";", `\n`, "\n", `\\`, `\`)
	return replacer.Replace(value)
}

func hashString(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}
