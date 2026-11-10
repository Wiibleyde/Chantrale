package helpers

import (
	"context"
	"sync"
	"time"

	"LsmsBot/internal/logger"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

// loadedGuilds tracks the guilds whose full member list has been received
// over the gateway and is therefore complete in disgo's member cache.
var (
	loadedGuilds   = make(map[snowflake.ID]struct{})
	loadedGuildsMu sync.Mutex
)

func setMembersLoaded(guildID snowflake.ID, loaded bool) {
	loadedGuildsMu.Lock()
	defer loadedGuildsMu.Unlock()
	if loaded {
		loadedGuilds[guildID] = struct{}{}
	} else {
		delete(loadedGuilds, guildID)
	}
}

func membersLoaded(guildID snowflake.ID) bool {
	loadedGuildsMu.Lock()
	defer loadedGuildsMu.Unlock()
	_, ok := loadedGuilds[guildID]
	return ok
}

// requestAllMembers fetches every member of the guild through the gateway
// (opcode 8, no REST call) so they end up in the member cache. It runs in its
// own goroutine to never block the gateway read loop.
func requestAllMembers(client *bot.Client, guildID snowflake.ID) {
	setMembersLoaded(guildID, false)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		members, err := client.MemberChunkingManager.RequestAllMembers(ctx, guildID)
		if err != nil {
			logger.Warn("Cannot load guild members over the gateway, falling back to REST", "guild", guildID.String(), "error", err)
			return
		}
		setMembersLoaded(guildID, true)
		logger.Info("Guild members cached", "guild", guildID.String(), "count", len(members))
	}()
}

// HandleGuildReady loads the member list of a guild once it becomes ready.
func HandleGuildReady(e *events.GuildReady) {
	requestAllMembers(e.Client(), e.GuildID)
}

// HandleGuildAvailable reloads the member list of a guild that came back
// after an outage.
func HandleGuildAvailable(e *events.GuildAvailable) {
	requestAllMembers(e.Client(), e.GuildID)
}

// HandleGuildUnavailable invalidates the member cache of an unavailable guild.
func HandleGuildUnavailable(e *events.GuildUnavailable) {
	setMembersLoaded(e.GuildID, false)
}

// HandleGuildLeave invalidates the member cache of a guild the bot left.
func HandleGuildLeave(e *events.GuildLeave) {
	setMembersLoaded(e.GuildID, false)
}

// GuildMembers returns all members of a guild. It reads disgo's member cache
// (kept up to date by gateway events) and only falls back to a REST call
// while the cache is not loaded yet.
func GuildMembers(client *bot.Client, guildID snowflake.ID) ([]discord.Member, error) {
	if membersLoaded(guildID) {
		members := make([]discord.Member, 0, client.Caches.MembersLen(guildID))
		for m := range client.Caches.Members(guildID) {
			members = append(members, m)
		}
		return members, nil
	}
	return client.Rest.GetMembers(guildID, 1000, 0)
}
