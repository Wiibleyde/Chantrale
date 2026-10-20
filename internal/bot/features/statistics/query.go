package statistics

import (
	"encoding/json"
	"sort"
	"time"

	"LsmsBot/internal/database"
	"LsmsBot/internal/database/models"
)

// maxSession caps a single duty session so a missed leave event (e.g. bot
// offline when the role was removed) does not inflate totals indefinitely.
const maxSession = 12 * time.Hour

// RankEntry is one leaderboard row. Value is a count, or seconds for duration criteria.
type RankEntry struct {
	UserID string
	Value  int64
}

func rankByEventCount(guildID, eventType string, since time.Time) ([]RankEntry, error) {
	query := database.DB.Model(&models.StatEvent{}).
		Select("user_id, COUNT(*) AS value").
		Where("guild_id = ? AND event_type = ? AND user_id <> ''", guildID, eventType)
	if !since.IsZero() {
		query = query.Where("created_at >= ?", since)
	}

	var entries []RankEntry
	if err := query.Group("user_id").Order("value DESC").Scan(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

// rankByServiceTime pairs duty.role_take / duty.role_leave events per member
// and sums the resulting sessions, clipped to [since, now].
func rankByServiceTime(guildID, roleType string, since, now time.Time) ([]RankEntry, error) {
	query := database.DB.Where("guild_id = ? AND event_type IN ? AND user_id <> ''", guildID, []string{"duty.role_take", "duty.role_leave"})
	if !since.IsZero() {
		// Look back far enough to catch sessions that started before the window.
		query = query.Where("created_at >= ?", since.Add(-maxSession))
	}

	var events []models.StatEvent
	if err := query.Order("created_at ASC, id ASC").Find(&events).Error; err != nil {
		return nil, err
	}

	open := make(map[string]time.Time)
	totals := make(map[string]time.Duration)
	for _, ev := range events {
		var payload struct {
			RoleType string `json:"role_type"`
		}
		if err := json.Unmarshal(ev.Payload, &payload); err != nil || payload.RoleType != roleType {
			continue
		}

		start, isOpen := open[ev.UserID]
		if ev.EventType == "duty.role_take" {
			// Button and member-update listeners may both record the same take.
			if !isOpen {
				open[ev.UserID] = ev.CreatedAt
			}
			continue
		}
		if !isOpen {
			continue
		}
		delete(open, ev.UserID)
		totals[ev.UserID] += clipSession(start, ev.CreatedAt, since)
	}
	for userID, start := range open {
		totals[userID] += clipSession(start, now, since)
	}

	entries := make([]RankEntry, 0, len(totals))
	for userID, d := range totals {
		if seconds := int64(d.Seconds()); seconds > 0 {
			entries = append(entries, RankEntry{UserID: userID, Value: seconds})
		}
	}
	sort.Slice(entries, func(a, b int) bool { return entries[a].Value > entries[b].Value })
	return entries, nil
}

func clipSession(start, end, since time.Time) time.Duration {
	if end.Sub(start) > maxSession {
		end = start.Add(maxSession)
	}
	if start.Before(since) {
		start = since
	}
	if !end.After(start) {
		return 0
	}
	return end.Sub(start)
}
