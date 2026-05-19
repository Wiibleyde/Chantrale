package duty

import (
	"sync"
	"time"

	"LsmsBot/internal/database"
	"LsmsBot/internal/database/models"
	"LsmsBot/internal/logger"
	"LsmsBot/internal/stats"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

// debounceState keeps prevRoles from the first event so the full diff is captured
// even when multiple role changes arrive in rapid succession.
type debounceState struct {
	timer     *time.Timer
	prevRoles []snowflake.ID
}

var (
	debounceMap = make(map[string]*debounceState)
	debounceMu  sync.Mutex

	embedDebounceMap = make(map[string]*time.Timer)
	embedDebounceMu  sync.Mutex

	// memberRoleCache stores the last known roles per member as a fallback
	// when disgo's internal cache doesn't have the old member state.
	memberRoleCache   = make(map[string][]snowflake.ID)
	memberRoleCacheMu sync.Mutex
)

func HandleGuildMemberUpdate(e *events.GuildMemberUpdate) {
	key := e.GuildID.String() + "-" + e.Member.User.ID.String()

	debounceMu.Lock()

	prevRoles := e.OldMember.RoleIDs

	// Fall back to our own role cache when disgo's cache has no old state.
	if len(prevRoles) == 0 {
		memberRoleCacheMu.Lock()
		if cached, ok := memberRoleCache[key]; ok {
			prevRoles = cached
		}
		memberRoleCacheMu.Unlock()
	}

	// Update our cache with the current roles.
	memberRoleCacheMu.Lock()
	memberRoleCache[key] = e.Member.RoleIDs
	memberRoleCacheMu.Unlock()

	if existing, ok := debounceMap[key]; ok {
		existing.timer.Stop()
		prevRoles = existing.prevRoles
	}

	client := e.Client()
	guildID := e.GuildID
	member := e.Member

	state := &debounceState{prevRoles: prevRoles}
	state.timer = time.AfterFunc(100*time.Millisecond, func() {
		debounceMu.Lock()
		delete(debounceMap, key)
		debounceMu.Unlock()

		handleMemberRoleChange(client, guildID, member, prevRoles)
	})
	debounceMap[key] = state
	debounceMu.Unlock()
}

func handleMemberRoleChange(client *bot.Client, guildID snowflake.ID, member discord.Member, prevRoles []snowflake.ID) {
	newRoles := member.RoleIDs

	var dms []models.DutyManager
	if err := database.DB.Where("guild_id = ?", guildID.String()).Find(&dms).Error; err != nil {
		logger.Error("Error fetching DutyManagers", "error", err)
		return
	}

	// No previous state at all — cannot determine which roles changed.
	// Refresh embeds so the display stays accurate, but skip logging.
	if len(prevRoles) == 0 {
		for _, dm := range dms {
			if dm.MessageID != nil {
				scheduleEmbedUpdate(client, guildID, dm)
			}
		}
		return
	}

	added := diffRoles(prevRoles, newRoles)
	removed := diffRoles(newRoles, prevRoles)
	changed := append(added, removed...)

	if len(changed) == 0 {
		return
	}

	for _, dm := range dms {
		affected := false
		for _, roleID := range changed {
			roleIDStr := roleID.String()
			if (dm.DutyRoleID != nil && *dm.DutyRoleID == roleIDStr) ||
				(dm.OnCallRoleID != nil && *dm.OnCallRoleID == roleIDStr) ||
				(dm.OffRadioRoleID != nil && *dm.OffRadioRoleID == roleIDStr) {
				affected = true
				break
			}
		}
		if !affected || dm.MessageID == nil {
			continue
		}

		displayName := memberDisplayName(member)
		if dm.LogsChannelID != nil {
			if logsChannelID, err := snowflake.Parse(*dm.LogsChannelID); err == nil {
				sendLogMessages(client, logsChannelID, guildID.String(), member.User.ID.String(), displayName, dm, added, removed)
			}
		}

		scheduleEmbedUpdate(client, guildID, dm)
	}
}

func sendLogMessages(
	client *bot.Client,
	logsChannelID snowflake.ID,
	guildID, memberID, displayName string,
	dm models.DutyManager,
	added, removed []snowflake.ID,
) {
	send := func(comps []discord.LayoutComponent) {
		if _, err := client.Rest.CreateMessage(logsChannelID, discord.NewMessageCreateV2(comps...)); err != nil {
			logger.Error("Error sending log embed", "error", err)
		}
	}

	for _, roleID := range added {
		roleIDStr := roleID.String()
		if dm.DutyRoleID != nil && *dm.DutyRoleID == roleIDStr {
			send(BuildDutyUpdateComponents(displayName, true))
			trackDuty(guildID, displayName)
			stats.Record(guildID, memberID, "duty.role_take", map[string]any{"role_type": "duty", "display_name": displayName, "source": "manual"})
		} else if dm.OnCallRoleID != nil && *dm.OnCallRoleID == roleIDStr {
			send(BuildOnCallUpdateComponents(displayName, true))
			trackOnCall(guildID, displayName)
			stats.Record(guildID, memberID, "duty.role_take", map[string]any{"role_type": "oncall", "display_name": displayName, "source": "manual"})
		} else if dm.OffRadioRoleID != nil && *dm.OffRadioRoleID == roleIDStr {
			send(BuildOffRadioUpdateComponents(displayName, true))
			trackOffRadio(guildID, displayName)
			stats.Record(guildID, memberID, "duty.role_take", map[string]any{"role_type": "offradio", "display_name": displayName, "source": "manual"})
		}
	}
	for _, roleID := range removed {
		roleIDStr := roleID.String()
		if dm.DutyRoleID != nil && *dm.DutyRoleID == roleIDStr {
			send(BuildDutyUpdateComponents(displayName, false))
			stats.Record(guildID, memberID, "duty.role_leave", map[string]any{"role_type": "duty", "display_name": displayName, "source": "manual"})
		} else if dm.OnCallRoleID != nil && *dm.OnCallRoleID == roleIDStr {
			send(BuildOnCallUpdateComponents(displayName, false))
			stats.Record(guildID, memberID, "duty.role_leave", map[string]any{"role_type": "oncall", "display_name": displayName, "source": "manual"})
		} else if dm.OffRadioRoleID != nil && *dm.OffRadioRoleID == roleIDStr {
			send(BuildOffRadioUpdateComponents(displayName, false))
			stats.Record(guildID, memberID, "duty.role_leave", map[string]any{"role_type": "offradio", "display_name": displayName, "source": "manual"})
		}
	}
}

// WarmMemberRoleCache pre-populates the role cache for all members of the
// given guilds so that the very first GuildMemberUpdate after startup has a
// known previous state to diff against.
func WarmMemberRoleCache(client *bot.Client, guildIDs []snowflake.ID) {
	for _, guildID := range guildIDs {
		members, err := client.Rest.GetMembers(guildID, 1000, 0)
		if err != nil {
			logger.Error("WarmMemberRoleCache: failed to fetch members", "guild", guildID.String(), "error", err)
			continue
		}
		memberRoleCacheMu.Lock()
		for _, m := range members {
			key := guildID.String() + "-" + m.User.ID.String()
			memberRoleCache[key] = m.RoleIDs
		}
		memberRoleCacheMu.Unlock()
	}
}

func scheduleEmbedUpdate(client *bot.Client, guildID snowflake.ID, dm models.DutyManager) {
	if dm.MessageID == nil {
		return
	}
	key := *dm.MessageID

	embedDebounceMu.Lock()
	if t, ok := embedDebounceMap[key]; ok {
		t.Stop()
	}
	embedDebounceMap[key] = time.AfterFunc(500*time.Millisecond, func() {
		embedDebounceMu.Lock()
		delete(embedDebounceMap, key)
		embedDebounceMu.Unlock()

		performEmbedUpdate(client, guildID, dm)
	})
	embedDebounceMu.Unlock()
}

func performEmbedUpdate(client *bot.Client, guildID snowflake.ID, dm models.DutyManager) {
	members, err := client.Rest.GetMembers(guildID, 1000, 0)
	if err != nil {
		logger.Error("Error fetching members for embed update", "guild", guildID.String(), "error", err)
		return
	}

	var onDuty, onCall, offRadio []string
	if dm.DutyRoleID != nil {
		if rid, err := snowflake.Parse(*dm.DutyRoleID); err == nil {
			onDuty = membersWithRole(members, rid)
		}
	}
	if dm.OnCallRoleID != nil {
		if rid, err := snowflake.Parse(*dm.OnCallRoleID); err == nil {
			onCall = membersWithRole(members, rid)
		}
	}
	if dm.OffRadioRoleID != nil {
		if rid, err := snowflake.Parse(*dm.OffRadioRoleID); err == nil {
			offRadio = membersWithRole(members, rid)
		}
	}

	setGuildCounts(guildID.String(), len(onDuty), len(onCall))

	components := BuildDutyComponents(onDuty, onCall, offRadio)
	chanID, err := snowflake.Parse(dm.ChannelID)
	if err != nil {
		logger.Error("Invalid channel ID in DutyManager", "channelID", dm.ChannelID, "error", err)
		return
	}
	msgSnowflake, err := snowflake.Parse(*dm.MessageID)
	if err != nil {
		logger.Error("Invalid message ID in DutyManager", "messageID", *dm.MessageID, "error", err)
		return
	}
	if _, err := client.Rest.UpdateMessage(chanID, msgSnowflake, discord.NewMessageUpdateV2(components...)); err != nil {
		logger.Error("Error editing duty message", "error", err)
	}

	updateBotPresence(client)
}

func diffRoles(a, b []snowflake.ID) []snowflake.ID {
	set := make(map[snowflake.ID]bool)
	for _, r := range a {
		set[r] = true
	}
	var diff []snowflake.ID
	for _, r := range b {
		if !set[r] {
			diff = append(diff, r)
		}
	}
	return diff
}
