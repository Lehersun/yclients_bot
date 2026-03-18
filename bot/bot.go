package bot

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
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

type IncomingMessage struct {
	Text         string
	ChatType     string
	BotUsername  string
	IsReplyToBot bool
}

// ReplyForText decides whether an incoming text message should trigger a reply.
func ReplyForText(text string) (string, bool) {
	if strings.EqualFold(strings.TrimSpace(text), "hello") {
		return "Hello!", true
	}

	return "", false
}

func NormalizeIncomingText(message IncomingMessage) (string, bool) {
	text := strings.TrimSpace(message.Text)
	if text == "" {
		return "", false
	}

	if message.ChatType == "" || message.ChatType == "private" {
		return text, true
	}

	if message.IsReplyToBot {
		return text, true
	}

	if message.BotUsername == "" {
		return "", false
	}

	fields := strings.Fields(text)
	if len(fields) == 0 {
		return "", false
	}

	mention := "@" + strings.ToLower(message.BotUsername)
	if strings.ToLower(fields[0]) != mention {
		return "", false
	}

	normalized := strings.TrimSpace(strings.Join(fields[1:], " "))
	if normalized == "" {
		return "", false
	}

	return normalized, true
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
	type slotTask struct {
		ServiceID   int
		CourtNumber int
		CourtLabel  string
		Duration    string
		Date        string
	}
	type slotResult struct {
		CourtNumber int
		CourtLabel  string
		Duration    string
		Slots       []time.Time
		Err         error
	}

	tasks := make([]slotTask, 0, len(services)*7)
	grouped := map[string]map[string]map[int]*courtAvailability{}

	for _, service := range services {
		courtNumber, courtLabel := normalizeCourtLabel(service.Title)
		if courtNumber == 0 {
			continue
		}
		duration := detectDuration(service.Title)

		for dayOffset := 0; dayOffset < 7; dayOffset++ {
			tasks = append(tasks, slotTask{
				ServiceID:   service.ID,
				CourtNumber: courtNumber,
				CourtLabel:  courtLabel,
				Duration:    duration,
				Date:        baseDate.AddDate(0, 0, dayOffset).Format("2006-01-02"),
			})
		}
	}

	results := make(chan slotResult, len(tasks))
	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup

	for _, task := range tasks {
		wg.Add(1)
		go func(task slotTask) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			slots, err := deps.Scheduler.SearchAvailableTimeSlots(ctx, yclients.SearchTimeSlotsParams{
				LocationID: deps.LocationID,
				Date:       task.Date,
				ServiceID:  task.ServiceID,
			})
			results <- slotResult{
				CourtNumber: task.CourtNumber,
				CourtLabel:  task.CourtLabel,
				Duration:    task.Duration,
				Slots:       slots,
				Err:         err,
			}
		}(task)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for result := range results {
		if result.Err != nil {
			return "", true, result.Err
		}

		for _, slot := range result.Slots {
			slot = slot.In(location)
			dateKey := slot.Format("02.01.2006")
			timeKey := slot.Format("15:04")

			if grouped[dateKey] == nil {
				grouped[dateKey] = map[string]map[int]*courtAvailability{}
			}
			if grouped[dateKey][timeKey] == nil {
				grouped[dateKey][timeKey] = map[int]*courtAvailability{}
			}
			if grouped[dateKey][timeKey][result.CourtNumber] == nil {
				grouped[dateKey][timeKey][result.CourtNumber] = &courtAvailability{
					Label:     result.CourtLabel,
					Durations: map[string]bool{},
				}
			}

			grouped[dateKey][timeKey][result.CourtNumber].Durations[result.Duration] = true
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
