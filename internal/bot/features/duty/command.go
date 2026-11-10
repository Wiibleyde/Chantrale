package duty

import (
	"LsmsBot/internal/bot/helpers"
	"LsmsBot/internal/database"
	"LsmsBot/internal/database/models"
	"LsmsBot/internal/logger"
	"LsmsBot/internal/stats"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

var Commands = []discord.ApplicationCommandCreate{
	discord.SlashCommandCreate{
		Name:        "duty",
		Description: "Gérer le gestionnaire de service",
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionSubCommand{
				Name:        "add",
				Description: "Ajouter un gestionnaire de service",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionRole{Name: "duty", Description: "Rôle de service", Required: true},
					discord.ApplicationCommandOptionRole{Name: "oncall", Description: "Rôle de semi service", Required: true},
					discord.ApplicationCommandOptionRole{Name: "offradio", Description: "Rôle off radio", Required: true},
					discord.ApplicationCommandOptionChannel{Name: "logchannel", Description: "Canal de logs", Required: true},
				},
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "remove",
				Description: "Supprimer un gestionnaire de service",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{Name: "messageid", Description: "ID du message du gestionnaire", Required: true},
				},
			},
		},
	},
}

func HandleCommand(e *events.ApplicationCommandInteractionCreate) {
	data := e.SlashCommandInteractionData()
	if data.SubCommandName == nil {
		return
	}
	switch *data.SubCommandName {
	case "add":
		handleAdd(e)
	case "remove":
		handleRemove(e)
	}
}

func handleAdd(e *events.ApplicationCommandInteractionCreate) {
	if !helpers.RequirePermission(e, discord.PermissionManageChannels, "Vous n'avez pas la permission de gérer les canaux.") {
		return
	}

	data := e.SlashCommandInteractionData()
	dutyRole, _ := data.OptRole("duty")
	onCallRole, _ := data.OptRole("oncall")
	offRadioRole, _ := data.OptRole("offradio")
	logChannel, _ := data.OptChannel("logchannel")

	client := e.Client()
	guildID := *e.GuildID()

	botMember, err := client.Rest.GetMember(guildID, client.ID())
	if err != nil {
		helpers.RespondEphemeral(e, "Erreur lors de la récupération des informations du bot.")
		return
	}

	guild, err := client.Rest.GetGuild(guildID, false)
	if err != nil {
		helpers.RespondEphemeral(e, "Erreur lors de la récupération du serveur.")
		return
	}

	botHighest := highestRolePosition(guild.Roles, botMember.RoleIDs)

	for _, r := range []discord.Role{dutyRole, onCallRole, offRadioRole} {
		if r.Position >= botHighest {
			helpers.RespondEphemeral(e, "Un ou plusieurs rôles sont au-dessus ou au même niveau que le rôle le plus haut du bot.")
			return
		}
	}

	members, err := helpers.GuildMembers(client, guildID)
	if err != nil {
		helpers.RespondEphemeral(e, "Erreur lors de la récupération des membres.")
		return
	}

	onDuty := membersWithRole(members, dutyRole.ID)
	onCall := membersWithRole(members, onCallRole.ID)
	offRadio := membersWithRole(members, offRadioRole.ID)

	components := BuildDutyComponents(onDuty, onCall, offRadio)

	channelID := e.Channel().ID()
	msg, err := client.Rest.CreateMessage(channelID, discord.NewMessageCreateV2(components...))
	if err != nil {
		logger.Error("Error sending duty message", "error", err)
		helpers.RespondEphemeral(e, "Erreur lors de l'envoi du message.")
		return
	}

	dutyRoleID := dutyRole.ID.String()
	onCallRoleID := onCallRole.ID.String()
	offRadioRoleID := offRadioRole.ID.String()
	logsChannelID := logChannel.ID.String()
	msgID := msg.ID.String()

	dm := models.DutyManager{
		GuildID:        guildID.String(),
		ChannelID:      channelID.String(),
		MessageID:      &msgID,
		DutyRoleID:     &dutyRoleID,
		OnCallRoleID:   &onCallRoleID,
		OffRadioRoleID: &offRadioRoleID,
		LogsChannelID:  &logsChannelID,
	}

	if err := database.DB.Create(&dm).Error; err != nil {
		logger.Error("Error saving DutyManager", "error", err)
		helpers.RespondEphemeral(e, "Erreur lors de l'enregistrement en base de données.")
		return
	}

	stats.Record(guildID.String(), e.Member().User.ID.String(), "duty.setup", map[string]any{
		"channel_id":      channelID.String(),
		"duty_role_id":    dutyRoleID,
		"oncall_role_id":  onCallRoleID,
		"offradio_role_id": offRadioRoleID,
		"logs_channel_id": logsChannelID,
	})

	helpers.RespondEphemeral(e, "Gestionnaire de service créé avec succès.")
}

func handleRemove(e *events.ApplicationCommandInteractionCreate) {
	if !helpers.RequirePermission(e, discord.PermissionManageChannels, "Vous n'avez pas la permission de gérer les canaux.") {
		return
	}

	data := e.SlashCommandInteractionData()
	messageID := data.String("messageid")

	guildID := *e.GuildID()

	var dm models.DutyManager
	if err := database.DB.Where("guild_id = ? AND message_id = ?", guildID.String(), messageID).First(&dm).Error; err != nil {
		helpers.RespondEphemeral(e, "Gestionnaire de service introuvable.")
		return
	}

	helpers.DeleteMessageByIDs(e.Client(), dm.ChannelID, messageID)

	if err := database.DB.Delete(&dm).Error; err != nil {
		logger.Error("Error deleting DutyManager", "error", err)
		helpers.RespondEphemeral(e, "Erreur lors de la suppression.")
		return
	}

	stats.Record(guildID.String(), e.Member().User.ID.String(), "duty.remove", map[string]any{
		"channel_id": dm.ChannelID,
		"message_id": messageID,
	})

	helpers.RespondEphemeral(e, "Gestionnaire de service supprimé avec succès.")
}

func memberDisplayName(m discord.Member) string {
	if m.Nick != nil {
		return *m.Nick
	}
	return m.User.Username
}

func membersWithRole(members []discord.Member, roleID snowflake.ID) []string {
	var result []string
	for _, m := range members {
		for _, r := range m.RoleIDs {
			if r == roleID {
				result = append(result, memberDisplayName(m))
				break
			}
		}
	}
	return result
}

func highestRolePosition(roles []discord.Role, memberRoleIDs []snowflake.ID) int {
	highest := 0
	for _, r := range roles {
		for _, id := range memberRoleIDs {
			if r.ID == id && r.Position > highest {
				highest = r.Position
			}
		}
	}
	return highest
}
