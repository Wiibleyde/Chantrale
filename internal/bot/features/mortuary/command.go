package mortuary

import (
	"fmt"
	"sort"

	"LsmsBot/internal/bot/helpers"
	"LsmsBot/internal/database"
	"LsmsBot/internal/database/models"
	"LsmsBot/internal/logger"
	"LsmsBot/internal/stats"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

var Commands = []discord.ApplicationCommandCreate{
	discord.SlashCommandCreate{
		Name:        "mortuary",
		Description: "Gestion de la morgue",
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionSubCommand{
				Name:        "init",
				Description: "Initialiser le panneau de la morgue",
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "remove",
				Description: "Supprimer le panneau de la morgue",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{Name: "messageid", Description: "ID du message du panneau", Required: true},
				},
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "add",
				Description: "Ajouter un corps à un casier",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionInt{
						Name:        "casier",
						Description: "Numéro du casier",
						Required:    true,
						Choices:     lockerChoices(),
					},
					discord.ApplicationCommandOptionString{Name: "nom_prenom", Description: "Nom et prénom du défunt", Required: true},
					discord.ApplicationCommandOptionString{Name: "commentaire", Description: "Commentaire optionnel", Required: false},
				},
			},
		},
	},
}

func lockerChoices() []discord.ApplicationCommandOptionChoiceInt {
	choices := make([]discord.ApplicationCommandOptionChoiceInt, totalLockers)
	for i := range totalLockers {
		choices[i] = discord.ApplicationCommandOptionChoiceInt{
			Name:  fmt.Sprintf("Casier %d", i+1),
			Value: i + 1,
		}
	}
	return choices
}

func HandleCommand(e *events.ApplicationCommandInteractionCreate) {
	data := e.SlashCommandInteractionData()
	if data.SubCommandName == nil {
		return
	}
	switch *data.SubCommandName {
	case "init":
		handleInit(e)
	case "remove":
		handleRemove(e)
	case "add":
		handleAdd(e)
	}
}

func handleInit(e *events.ApplicationCommandInteractionCreate) {
	guildID := *e.GuildID()

	var existing []models.MortuaryManager
	if err := database.DB.Where("guild_id = ?", guildID.String()).Limit(1).Find(&existing).Error; err != nil {
		logger.Error("Error checking mortuary manager", "error", err)
		helpers.RespondEphemeral(e, "Erreur lors de la vérification du panneau.")
		return
	}
	if len(existing) > 0 {
		helpers.RespondEphemeral(e, "Un panneau de la morgue existe déjà dans ce serveur.")
		return
	}

	embed := BuildMortuaryEmbed(nil)
	channelID := e.Channel().ID()

	msg, err := e.Client().Rest.CreateMessage(channelID, discord.MessageCreate{
		Embeds: []discord.Embed{embed},
	})
	if err != nil {
		logger.Error("Error sending mortuary panel message", "error", err)
		helpers.RespondEphemeral(e, "Erreur lors de l'envoi du panneau de la morgue.")
		return
	}

	mm := models.MortuaryManager{
		GuildID:   guildID.String(),
		ChannelID: channelID.String(),
		MessageID: msg.ID.String(),
	}
	if err := database.DB.Create(&mm).Error; err != nil {
		logger.Error("Error saving mortuary manager", "error", err)
		helpers.RespondEphemeral(e, "Erreur lors de la sauvegarde en base de données.")
		return
	}

	stats.Record(guildID.String(), e.Member().User.ID.String(), "mortuary.panel_init", map[string]any{
		"channel_id": channelID.String(),
	})

	helpers.RespondEphemeral(e, "Panneau de la morgue initialisé avec succès.")
}

func handleAdd(e *events.ApplicationCommandInteractionCreate) {
	guildID := *e.GuildID()

	var mm models.MortuaryManager
	if err := database.DB.Where("guild_id = ?", guildID.String()).First(&mm).Error; err != nil {
		helpers.RespondEphemeral(e, "Aucun panneau de la morgue trouvé. Utilisez `/mortuary init` d'abord.")
		return
	}

	data := e.SlashCommandInteractionData()
	lockerNumber := data.Int("casier")
	name := data.String("nom_prenom")

	var comment *string
	if v, ok := data.OptString("commentaire"); ok && v != "" {
		comment = &v
	}

	var existingAssignments []models.MortuaryAssignment
	if err := database.DB.Where("guild_id = ? AND locker_number = ?", guildID.String(), lockerNumber).Limit(1).Find(&existingAssignments).Error; err != nil {
		logger.Error("Error checking mortuary assignment", "error", err)
		helpers.RespondEphemeral(e, "Erreur lors de la vérification du casier.")
		return
	}
	if len(existingAssignments) > 0 {
		helpers.RespondEphemeral(e, fmt.Sprintf("Le casier %d est déjà occupé par ||%s||.", lockerNumber, existingAssignments[0].Name))
		return
	}

	assignment := models.MortuaryAssignment{
		GuildID:      guildID.String(),
		LockerNumber: int(lockerNumber),
		Name:         name,
		Comment:      comment,
	}
	if err := database.DB.Create(&assignment).Error; err != nil {
		logger.Error("Error creating mortuary assignment", "error", err)
		helpers.RespondEphemeral(e, "Erreur lors de l'ajout du corps.")
		return
	}

	if err := updateMortuaryPanel(e.Client(), mm); err != nil {
		logger.Error("Error updating mortuary panel", "error", err)
		helpers.RespondEphemeral(e, "Corps ajouté mais erreur lors de la mise à jour du panneau.")
		return
	}

	stats.Record(guildID.String(), e.Member().User.ID.String(), "mortuary.assign", map[string]any{
		"locker_number": lockerNumber,
		"name":          name,
		"comment":       comment,
	})

	helpers.RespondEphemeral(e, fmt.Sprintf("Corps de **||%s||** ajouté au casier **%d**.", name, lockerNumber))
}

func handleRemove(e *events.ApplicationCommandInteractionCreate) {
	if !helpers.RequirePermission(e, discord.PermissionManageChannels, "Vous n'avez pas la permission de gérer les canaux.") {
		return
	}

	data := e.SlashCommandInteractionData()
	messageID := data.String("messageid")

	guildID := *e.GuildID()

	var mm models.MortuaryManager
	if err := database.DB.Where("guild_id = ? AND message_id = ?", guildID.String(), messageID).First(&mm).Error; err != nil {
		helpers.RespondEphemeral(e, "Panneau de la morgue introuvable.")
		return
	}

	helpers.DeleteMessageByIDs(e.Client(), mm.ChannelID, messageID)

	if err := database.DB.Where("guild_id = ?", guildID.String()).Delete(&models.MortuaryAssignment{}).Error; err != nil {
		logger.Error("Error deleting mortuary assignments", "error", err)
		helpers.RespondEphemeral(e, "Erreur lors de la suppression des assignations.")
		return
	}

	if err := database.DB.Delete(&mm).Error; err != nil {
		logger.Error("Error deleting MortuaryManager", "error", err)
		helpers.RespondEphemeral(e, "Erreur lors de la suppression.")
		return
	}

	stats.Record(guildID.String(), e.Member().User.ID.String(), "mortuary.panel_remove", map[string]any{
		"channel_id": mm.ChannelID,
	})

	helpers.RespondEphemeral(e, "Panneau de la morgue supprimé avec succès.")
}

func updateMortuaryPanel(client *bot.Client, mm models.MortuaryManager) error {
	var assignments []models.MortuaryAssignment
	if err := database.DB.Where("guild_id = ?", mm.GuildID).Find(&assignments).Error; err != nil {
		return err
	}

	embed := BuildMortuaryEmbed(assignments)
	components := buildMortuaryButtons(assignments)
	embedsList := []discord.Embed{embed}

	chanID, err := snowflake.Parse(mm.ChannelID)
	if err != nil {
		return err
	}
	msgID, err := snowflake.Parse(mm.MessageID)
	if err != nil {
		return err
	}

	_, err = client.Rest.UpdateMessage(chanID, msgID, discord.MessageUpdate{
		Embeds:     &embedsList,
		Components: &components,
	})
	return err
}

func buildMortuaryButtons(assignments []models.MortuaryAssignment) []discord.LayoutComponent {
	if len(assignments) == 0 {
		return []discord.LayoutComponent{}
	}

	sort.Slice(assignments, func(i, j int) bool {
		return assignments[i].LockerNumber < assignments[j].LockerNumber
	})

	var buttons []discord.InteractiveComponent
	for _, a := range assignments {
		label := fmt.Sprintf("Casier %d", a.LockerNumber)

		buttons = append(buttons, discord.ButtonComponent{
			Label:    label,
			Style:    discord.ButtonStyleSecondary,
			CustomID: fmt.Sprintf("lsmsMortuary--%d", a.LockerNumber),
		})
	}

	return helpers.BuildActionRows(buttons, 5)
}
