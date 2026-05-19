package labo

import (
	"fmt"
	"sync"
	"time"

	"LsmsBot/internal/logger"
	"LsmsBot/internal/stats"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

type LaboEntry struct {
	GuildID   string
	ChannelID snowflake.ID
	MessageID snowflake.ID
	UserID    snowflake.ID
	StartTime time.Time
	Name      string
	Type      string
	Result    string
	Time      int
}

type LaboQueue struct {
	mu      sync.Mutex
	entries []*LaboEntry
	ticker  *time.Ticker
	stopCh  chan struct{}
	client  *bot.Client
}

var Queue = &LaboQueue{}

func (q *LaboQueue) SetClient(c *bot.Client) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.client = c
}

func (q *LaboQueue) Add(entry *LaboEntry) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.entries = append(q.entries, entry)

	if q.ticker == nil {
		q.ticker = time.NewTicker(10 * time.Second)
		q.stopCh = make(chan struct{})
		go q.run()
	}
}

func (q *LaboQueue) run() {
	for {
		select {
		case <-q.stopCh:
			return
		case <-q.ticker.C:
			q.mu.Lock()
			now := time.Now()
			var remaining []*LaboEntry
			var toNotify []*LaboEntry

			for _, e := range q.entries {
				elapsed := now.Sub(e.StartTime)
				if elapsed >= time.Duration(e.Time)*time.Minute {
					toNotify = append(toNotify, e)
				} else {
					remaining = append(remaining, e)
				}
			}
			q.entries = remaining

			if len(q.entries) == 0 {
				q.ticker.Stop()
				q.ticker = nil
				close(q.stopCh)
				q.stopCh = nil
				q.mu.Unlock()
				for _, e := range toNotify {
					if q.client != nil {
						notifyCompletion(q.client, e)
					}
				}
				return
			}

			client := q.client
			q.mu.Unlock()

			for _, e := range toNotify {
				if client != nil {
					notifyCompletion(client, e)
				}
			}
		}
	}
}

func (q *LaboQueue) CancelByMessageID(messageID snowflake.ID) (bool, *LaboEntry) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for idx, e := range q.entries {
		if e.MessageID == messageID {
			q.entries = append(q.entries[:idx], q.entries[idx+1:]...)
			if len(q.entries) == 0 && q.ticker != nil {
				q.ticker.Stop()
				q.ticker = nil
				close(q.stopCh)
				q.stopCh = nil
			}
			return true, e
		}
	}
	return false, nil
}

func notifyCompletion(client *bot.Client, entry *LaboEntry) {
	stats.Record(entry.GuildID, entry.UserID.String(), "labo.test_complete", map[string]any{
		"test_type":        entry.Type,
		"patient_name":     entry.Name,
		"result":           entry.Result,
		"duration_minutes": entry.Time,
	})

	resultComponents := BuildLaboResultComponents(entry)
	if _, err := client.Rest.UpdateMessage(entry.ChannelID, entry.MessageID, discord.NewMessageUpdateV2(resultComponents...)); err != nil {
		logger.Error("Error editing labo message", "error", err)
	}

	ping, err := client.Rest.CreateMessage(entry.ChannelID, discord.MessageCreate{
		Content: fmt.Sprintf("<@%s> Votre analyse est terminée !", entry.UserID),
	})
	if err != nil {
		logger.Error("Error sending ping", "error", err)
		return
	}

	time.AfterFunc(60*time.Second, func() {
		if err := client.Rest.DeleteMessage(entry.ChannelID, ping.ID); err != nil {
			logger.Error("Error deleting ping", "error", err)
		}
	})
}
