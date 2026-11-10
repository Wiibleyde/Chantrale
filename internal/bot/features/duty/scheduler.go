package duty

import (
	"time"

	"LsmsBot/internal/bot/helpers"
	"LsmsBot/internal/database"
	"LsmsBot/internal/database/models"
	"LsmsBot/internal/logger"
	"LsmsBot/internal/stats"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

var botStartTime = time.Now()

func StartScheduler(client *bot.Client) {
	go func() {
		for {
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day(), 5, 55, 0, 0, now.Location())
			if !now.Before(next) {
				next = next.Add(24 * time.Hour)
			}
			logger.Info("Next duty reset scheduled", "at", next.Format(time.RFC3339))
			time.Sleep(time.Until(next))
			logger.Info("Running daily duty reset at 05:55")
			runReset(client)
		}
	}()
}

func SendShutdownSummary(client *bot.Client) {
	logger.Info("Sending duty shutdown summary...")
	sendSummaryToAll(client)
}

func runReset(client *bot.Client) {
	var dms []models.DutyManager
	if err := database.DB.Find(&dms).Error; err != nil {
		logger.Error("Error fetching DutyManagers for reset", "error", err)
		return
	}

	for _, dm := range dms {
		stripDutyRoles(client, dm)
		setGuildCounts(dm.GuildID, 0, 0)
	}

	updateBotPresence(client)
}

func sendSummaryToAll(client *bot.Client) {
	var dms []models.DutyManager
	if err := database.DB.Find(&dms).Error; err != nil {
		logger.Error("Error fetching DutyManagers for summary", "error", err)
		return
	}

	for _, dm := range dms {
		onDuty, onCall, offRadio := popHistory(dm.GuildID)

		stats.Record(dm.GuildID, "", "duty.shutdown_summary", map[string]any{
			"on_duty_users":   onDuty,
			"on_call_users":   onCall,
			"off_radio_users": offRadio,
			"session_start":   botStartTime.Format(time.RFC3339),
		})

		if dm.LogsChannelID != nil {
			logsChannelID, err := snowflake.Parse(*dm.LogsChannelID)
			if err == nil {
				embed := BuildSummaryComponents(botStartTime, time.Now(), onDuty, onCall, offRadio)
				if _, err := client.Rest.CreateMessage(logsChannelID, discord.NewMessageCreateV2(embed...)); err != nil {
					logger.Error("Error sending duty summary", "guild", dm.GuildID, "error", err)
				}
			}
		}
	}
}

func stripDutyRoles(client *bot.Client, dm models.DutyManager) {
	guildID, err := snowflake.Parse(dm.GuildID)
	if err != nil {
		return
	}

	members, err := helpers.GuildMembers(client, guildID)
	if err != nil {
		logger.Error("Error fetching members for role strip", "guild", dm.GuildID, "error", err)
		return
	}

	for _, member := range members {
		for _, roleID := range member.RoleIDs {
			roleIDStr := roleID.String()
			if (dm.DutyRoleID != nil && *dm.DutyRoleID == roleIDStr) ||
				(dm.OnCallRoleID != nil && *dm.OnCallRoleID == roleIDStr) ||
				(dm.OffRadioRoleID != nil && *dm.OffRadioRoleID == roleIDStr) {
				if err := client.Rest.RemoveMemberRole(guildID, member.User.ID, roleID); err != nil {
					logger.Error("Error removing role during reset", "user", member.User.ID, "role", roleID, "error", err)
				}
			}
		}
	}
}
