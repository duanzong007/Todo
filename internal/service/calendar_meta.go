package service

import (
	"container/list"
	"fmt"
	"strings"
	"time"

	HolidayUtil "github.com/6tail/lunar-go/HolidayUtil"
	"github.com/6tail/lunar-go/calendar"
)

type CalendarMeta struct {
	WeekdayLabel string
	Tags         []string
}

func CalendarMetaForDate(value time.Time, location *time.Location) CalendarMeta {
	date := value
	if location != nil {
		date = value.In(location)
	}

	solar := calendar.NewSolarFromYmd(date.Year(), int(date.Month()), date.Day())
	lunar := solar.GetLunar()

	meta := CalendarMeta{
		WeekdayLabel: weekdayLabel(date.Weekday()),
	}

	seen := make(map[string]struct{})

	if holiday := HolidayUtil.GetHolidayByYmd(date.Year(), int(date.Month()), date.Day()); holiday != nil {
		name := strings.TrimSpace(holiday.GetName())
		if holiday.IsWork() && name != "" {
			name += "调休"
		}
		appendUniqueTag(&meta.Tags, seen, name)
	}

	appendUniqueTags(&meta.Tags, seen, listToStrings(solar.GetFestivals()))
	appendUniqueTags(&meta.Tags, seen, listToStrings(lunar.GetFestivals()))

	if jieQi := strings.TrimSpace(lunar.GetJieQi()); jieQi != "" {
		appendUniqueTag(&meta.Tags, seen, jieQi)
	}

	return meta
}

func appendUniqueTags(tags *[]string, seen map[string]struct{}, values []string) {
	for _, value := range values {
		appendUniqueTag(tags, seen, value)
	}
}

func appendUniqueTag(tags *[]string, seen map[string]struct{}, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return
	}
	if _, ok := seen[trimmed]; ok {
		return
	}
	seen[trimmed] = struct{}{}
	*tags = append(*tags, trimmed)
}

func listToStrings(values *list.List) []string {
	if values == nil || values.Len() == 0 {
		return nil
	}

	items := make([]string, 0, values.Len())
	for item := values.Front(); item != nil; item = item.Next() {
		items = append(items, fmt.Sprint(item.Value))
	}
	return items
}

func weekdayLabel(weekday time.Weekday) string {
	switch weekday {
	case time.Monday:
		return "星期一"
	case time.Tuesday:
		return "星期二"
	case time.Wednesday:
		return "星期三"
	case time.Thursday:
		return "星期四"
	case time.Friday:
		return "星期五"
	case time.Saturday:
		return "星期六"
	default:
		return "星期日"
	}
}
