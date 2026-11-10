package duty

import (
	"context"
	"fmt"
	"sync"

	"LsmsBot/internal/bot/helpers"
	"LsmsBot/internal/database"
	"LsmsBot/internal/database/models"
	"LsmsBot/internal/logger"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/snowflake/v2"
)

var (
	guildCounts   = make(map[string][2]int) // [onDuty, onCall] per guild
	guildCountsMu sync.Mutex

	// lastPresence is the last status sent to the gateway, to avoid sending
	// identical presence updates (they count against the gateway rate limit).
	lastPresence   string
	lastPresenceMu sync.Mutex
)

func setGuildCounts(guildID string, onDuty, onCall int) {
	guildCountsMu.Lock()
	defer guildCountsMu.Unlock()
	guildCounts[guildID] = [2]int{onDuty, onCall}
}

func updateBotPresence(client *bot.Client) {
	guildCountsMu.Lock()
	totalOnDuty, totalOffRadio := 0, 0
	for _, counts := range guildCounts {
		totalOnDuty += counts[0]
		totalOffRadio += counts[1]
	}
	guildCountsMu.Unlock()

	status := fmt.Sprintf("%d en service • %d en astreinte", totalOnDuty, totalOffRadio)

	lastPresenceMu.Lock()
	defer lastPresenceMu.Unlock()
	if status == lastPresence {
		return
	}
	if err := client.SetPresence(context.Background(), gateway.WithWatchingActivity(status)); err != nil {
		logger.Error("Error updating bot presence", "error", err)
		return
	}
	lastPresence = status
}

func InitPresence(client *bot.Client) {
	var dms []models.DutyManager
	if err := database.DB.Find(&dms).Error; err != nil {
		logger.Error("Error loading DutyManagers for presence init", "error", err)
		return
	}

	for _, dm := range dms {
		guildID, err := snowflake.Parse(dm.GuildID)
		if err != nil {
			continue
		}
		members, err := helpers.GuildMembers(client, guildID)
		if err != nil {
			logger.Warn("Error fetching members for presence init", "guild", dm.GuildID, "error", err)
			continue
		}
		onDuty := 0
		if dm.DutyRoleID != nil {
			if rid, err := snowflake.Parse(*dm.DutyRoleID); err == nil {
				onDuty = len(membersWithRole(members, rid))
			}
		}
		onCall := 0
		if dm.OnCallRoleID != nil {
			if rid, err := snowflake.Parse(*dm.OnCallRoleID); err == nil {
				onCall = len(membersWithRole(members, rid))
			}
		}
		setGuildCounts(dm.GuildID, onDuty, onCall)
	}

	updateBotPresence(client)
}
