package helpers

import (
	"LsmsBot/internal/api/dto"
	bothelpers "LsmsBot/internal/bot/helpers"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/snowflake/v2"
)

// MembersWithRole returns all guild members that have the given role.
// Returns an empty (non-nil) slice when no members match.
func MembersWithRole(client *bot.Client, guildID snowflake.ID, roleID snowflake.ID) ([]dto.MemberDTO, error) {
	members, err := bothelpers.GuildMembers(client, guildID)
	if err != nil {
		return nil, err
	}

	result := []dto.MemberDTO{}
	for _, m := range members {
		for _, r := range m.RoleIDs {
			if r == roleID {
				displayName := m.User.Username
				if m.Nick != nil {
					displayName = *m.Nick
				}
				result = append(result, dto.MemberDTO{
					ID:          m.User.ID.String(),
					Username:    m.User.Username,
					DisplayName: displayName,
				})
				break
			}
		}
	}
	return result, nil
}
