package bot

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"yclients_bot/yclients"
)

type Scheduler interface {
	AvailableServices(ctx context.Context, locationID int) ([]yclients.Service, error)
	SearchAvailableTimeSlots(ctx context.Context, params yclients.SearchTimeSlotsParams) ([]time.Time, error)
}

type Dependencies struct {
	Scheduler  Scheduler
	LocationID int
	Location   *time.Location
	Now        func() time.Time
}

// ReplyForText decides whether an incoming text message should trigger a reply.
func ReplyForText(text string) (string, bool) {
	if strings.EqualFold(strings.TrimSpace(text), "hello") {
		return "Hello!", true
	}

	return "", false
}

func HandleText(ctx context.Context, text string, deps Dependencies) (string, bool, error) {
	if reply, ok := ReplyForText(text); ok {
		return reply, true, nil
	}

	if !strings.EqualFold(strings.TrimSpace(text), "schedule") {
		return "", false, nil
	}

	if deps.Scheduler == nil {
		return "Schedule is not configured.", true, nil
	}

	if deps.LocationID == 0 {
		return "", true, fmt.Errorf("location id is required")
	}

	location := deps.Location
	if location == nil {
		location = time.Local
	}

	nowFn := deps.Now
	if nowFn == nil {
		nowFn = time.Now
	}

	services, err := deps.Scheduler.AvailableServices(ctx, deps.LocationID)
	if err != nil {
		return "", true, err
	}

	baseDate := nowFn().In(location)
	grouped := map[string]map[string]map[int]*courtAvailability{}

	for _, service := range services {
		courtNumber, courtLabel := normalizeCourtLabel(service.Title)
		if courtNumber == 0 {
			continue
		}
		duration := detectDuration(service.Title)

		for dayOffset := 0; dayOffset < 7; dayOffset++ {
			date := baseDate.AddDate(0, 0, dayOffset).Format("2006-01-02")
			slots, err := deps.Scheduler.SearchAvailableTimeSlots(ctx, yclients.SearchTimeSlotsParams{
				LocationID: deps.LocationID,
				Date:       date,
				ServiceID:  service.ID,
			})
			if err != nil {
				return "", true, err
			}

			for _, slot := range slots {
				slot = slot.In(location)
				dateKey := slot.Format("02.01.2006")
				timeKey := slot.Format("15:04")

				if grouped[dateKey] == nil {
					grouped[dateKey] = map[string]map[int]*courtAvailability{}
				}
				if grouped[dateKey][timeKey] == nil {
					grouped[dateKey][timeKey] = map[int]*courtAvailability{}
				}
				if grouped[dateKey][timeKey][courtNumber] == nil {
					grouped[dateKey][timeKey][courtNumber] = &courtAvailability{
						Label:     courtLabel,
						Durations: map[string]bool{},
					}
				}

				grouped[dateKey][timeKey][courtNumber].Durations[duration] = true
			}
		}
	}

	if len(grouped) == 0 {
		return "No available slots in the next 7 days.", true, nil
	}

	return formatSchedule(grouped), true, nil
}

type courtAvailability struct {
	Label     string
	Durations map[string]bool
}

var courtPattern = regexp.MustCompile(`(?i)(?:корт\s*(\d+)|(\d+)\s*корт)`)

func normalizeCourtLabel(title string) (int, string) {
	match := courtPattern.FindStringSubmatch(title)
	if len(match) < 3 {
		return 0, ""
	}

	number := match[1]
	if number == "" {
		number = match[2]
	}

	var courtNumber int
	fmt.Sscanf(number, "%d", &courtNumber)
	if courtNumber == 0 {
		return 0, ""
	}

	return courtNumber, fmt.Sprintf("корт %d", courtNumber)
}

func detectDuration(title string) string {
	lower := strings.ToLower(title)
	if strings.Contains(lower, "2 часа") || strings.Contains(lower, "2часа") {
		return "на два"
	}
	return "на час"
}

func formatSchedule(grouped map[string]map[string]map[int]*courtAvailability) string {
	dateKeys := make([]string, 0, len(grouped))
	for dateKey := range grouped {
		dateKeys = append(dateKeys, dateKey)
	}
	sort.Slice(dateKeys, func(i, j int) bool {
		left, _ := time.Parse("02.01.2006", dateKeys[i])
		right, _ := time.Parse("02.01.2006", dateKeys[j])
		return left.Before(right)
	})

	lines := make([]string, 0)
	for _, dateKey := range dateKeys {
		lines = append(lines, dateKey+":")

		timeKeys := make([]string, 0, len(grouped[dateKey]))
		for timeKey := range grouped[dateKey] {
			timeKeys = append(timeKeys, timeKey)
		}
		sort.Strings(timeKeys)

		for _, timeKey := range timeKeys {
			courts := grouped[dateKey][timeKey]
			courtNumbers := make([]int, 0, len(courts))
			for courtNumber := range courts {
				courtNumbers = append(courtNumbers, courtNumber)
			}
			sort.Ints(courtNumbers)

			items := make([]string, 0, len(courtNumbers))
			for _, courtNumber := range courtNumbers {
				court := courts[courtNumber]
				durations := make([]string, 0, 2)
				if court.Durations["на час"] {
					durations = append(durations, "на час")
				}
				if court.Durations["на два"] {
					durations = append(durations, "на два")
				}
				items = append(items, fmt.Sprintf("%s (%s)", court.Label, strings.Join(durations, ", ")))
			}

			lines = append(lines, fmt.Sprintf("%s - %s", timeKey, strings.Join(items, ", ")))
		}
	}

	return strings.Join(lines, "\n")
}
