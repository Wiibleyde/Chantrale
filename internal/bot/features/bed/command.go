package bed

import (
	"bytes"
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
		Name:        "beds",
		Description: "Gestion des lits de l'hôpital",
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionSubCommand{
				Name:        "init",
				Description: "Initialiser le panneau des lits",
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "remove",
				Description: "Supprimer le panneau des lits",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{Name: "messageid", Description: "ID du message du panneau", Required: true},
				},
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "add",
				Description: "Ajouter un patient à un lit",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:        "lit",
						Description: "Lit à attribuer",
						Required:    true,
						Choices:     bedChoices(),
					},
					discord.ApplicationCommandOptionString{Name: "nom_prenom", Description: "Nom et prénom du patient", Required: true},
					discord.ApplicationCommandOptionString{Name: "description", Description: "Description optionnelle (affichée au-dessus de l'image)", Required: false},
					discord.ApplicationCommandOptionBool{Name: "garde_a_vue", Description: "Le patient est-il en garde à vue ?", Required: false},
					discord.ApplicationCommandOptionBool{Name: "deces", Description: "Le patient est-il décédé ?", Required: false},
				},
			},
		},
	},
}

func bedChoices() []discord.ApplicationCommandOptionChoiceString {
	choices := make([]discord.ApplicationCommandOptionChoiceString, len(BedConfigs))
	for i, bed := range BedConfigs {
		choices[i] = discord.ApplicationCommandOptionChoiceString{
			Name:  "Lit " + bed.Letter,
			Value: bed.Letter,
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

	var existing []models.BedManager
	if err := database.DB.Where("guild_id = ?", guildID.String()).Limit(1).Find(&existing).Error; err != nil {
		logger.Error("Error checking bed manager", "error", err)
		helpers.RespondEphemeral(e, "Erreur lors de la vérification du panneau.")
		return
	}
	if len(existing) > 0 {
		helpers.RespondEphemeral(e, "Un panneau des lits existe déjà dans ce serveur.")
		return
	}

	imgBytes, err := GenerateBedImage(nil)
	if err != nil {
		logger.Error("Error generating bed image", "error", err)
		helpers.RespondEphemeral(e, "Erreur lors de la génération de l'image des lits.")
		return
	}

	embed := BuildBedEmbed(nil)
	channelID := e.Channel().ID()

	msg, err := e.Client().Rest.CreateMessage(channelID, discord.MessageCreate{
		Embeds: []discord.Embed{embed},
		Files:  []*discord.File{discord.NewFile("beds.png", "", bytes.NewReader(imgBytes))},
	})
	if err != nil {
		logger.Error("Error sending bed panel message", "error", err)
		helpers.RespondEphemeral(e, "Erreur lors de l'envoi du panneau des lits.")
		return
	}

	bm := models.BedManager{
		GuildID:   guildID.String(),
		ChannelID: channelID.String(),
		MessageID: msg.ID.String(),
	}
	if err := database.DB.Create(&bm).Error; err != nil {
		logger.Error("Error saving bed manager", "error", err)
		helpers.RespondEphemeral(e, "Erreur lors de la sauvegarde en base de données.")
		return
	}

	stats.Record(guildID.String(), e.Member().User.ID.String(), "bed.panel_init", map[string]any{
		"channel_id": channelID.String(),
	})

	helpers.RespondEphemeral(e, "Panneau des lits initialisé avec succès.")
}

func handleAdd(e *events.ApplicationCommandInteractionCreate) {
	guildID := *e.GuildID()

	var bm models.BedManager
	if err := database.DB.Where("guild_id = ?", guildID.String()).First(&bm).Error; err != nil {
		helpers.RespondEphemeral(e, "Aucun panneau des lits trouvé. Utilisez `/beds init` d'abord.")
		return
	}

	data := e.SlashCommandInteractionData()
	bedLetter := data.String("lit")
	patientName := data.String("nom_prenom")

	var underArrest, death bool
	if v, ok := data.OptBool("garde_a_vue"); ok {
		underArrest = v
	}
	if v, ok := data.OptBool("deces"); ok {
		death = v
	}

	var description *string
	if v, ok := data.OptString("description"); ok && v != "" {
		description = &v
	}

	var existingAssignments []models.BedAssignment
	if err := database.DB.Where("guild_id = ? AND bed_letter = ?", guildID.String(), bedLetter).Limit(1).Find(&existingAssignments).Error; err != nil {
		logger.Error("Error checking bed assignment", "error", err)
		helpers.RespondEphemeral(e, "Erreur lors de la vérification du lit.")
		return
	}
	if len(existingAssignments) > 0 {
		helpers.RespondEphemeral(e, fmt.Sprintf("Le lit %s est déjà occupé par %s.", bedLetter, existingAssignments[0].Name))
		return
	}

	assignment := models.BedAssignment{
		GuildID:     guildID.String(),
		BedLetter:   bedLetter,
		Name:        patientName,
		Description: description,
		UnderArrest: underArrest,
		Death:       death,
	}
	if err := database.DB.Create(&assignment).Error; err != nil {
		logger.Error("Error creating bed assignment", "error", err)
		helpers.RespondEphemeral(e, "Erreur lors de l'ajout du patient.")
		return
	}

	if err := updateBedPanel(e.Client(), bm); err != nil {
		logger.Error("Error updating bed panel", "error", err)
		helpers.RespondEphemeral(e, "Patient ajouté mais erreur lors de la mise à jour du panneau.")
		return
	}

	stats.Record(guildID.String(), e.Member().User.ID.String(), "bed.assign", map[string]any{
		"bed_letter":   bedLetter,
		"patient_name": patientName,
		"description":  description,
		"under_arrest": underArrest,
		"death":        death,
	})

	helpers.RespondEphemeral(e, fmt.Sprintf("Patient **%s** ajouté au lit **%s**.", patientName, bedLetter))
}

func handleRemove(e *events.ApplicationCommandInteractionCreate) {
	if !helpers.RequirePermission(e, discord.PermissionManageChannels, "Vous n'avez pas la permission de gérer les canaux.") {
		return
	}

	data := e.SlashCommandInteractionData()
	messageID := data.String("messageid")

	guildID := *e.GuildID()

	var bm models.BedManager
	if err := database.DB.Where("guild_id = ? AND message_id = ?", guildID.String(), messageID).First(&bm).Error; err != nil {
		helpers.RespondEphemeral(e, "Panneau des lits introuvable.")
		return
	}

	helpers.DeleteMessageByIDs(e.Client(), bm.ChannelID, messageID)

	if err := database.DB.Where("guild_id = ?", guildID.String()).Delete(&models.BedAssignment{}).Error; err != nil {
		logger.Error("Error deleting bed assignments", "error", err)
		helpers.RespondEphemeral(e, "Erreur lors de la suppression des assignations.")
		return
	}

	if err := database.DB.Delete(&bm).Error; err != nil {
		logger.Error("Error deleting BedManager", "error", err)
		helpers.RespondEphemeral(e, "Erreur lors de la suppression.")
		return
	}

	stats.Record(guildID.String(), e.Member().User.ID.String(), "bed.panel_remove", map[string]any{
		"channel_id": bm.ChannelID,
	})

	helpers.RespondEphemeral(e, "Panneau des lits supprimé avec succès.")
}

func updateBedPanel(client *bot.Client, bm models.BedManager) error {
	var assignments []models.BedAssignment
	if err := database.DB.Where("guild_id = ?", bm.GuildID).Find(&assignments).Error; err != nil {
		return err
	}

	imgBytes, err := GenerateBedImage(assignments)
	if err != nil {
		return err
	}

	embed := BuildBedEmbed(assignments)
	components := buildBedButtons(assignments)
	embeds := []discord.Embed{embed}
	emptyAttachments := []discord.AttachmentUpdate{}

	chanID, err := snowflake.Parse(bm.ChannelID)
	if err != nil {
		return err
	}
	msgID, err := snowflake.Parse(bm.MessageID)
	if err != nil {
		return err
	}

	_, err = client.Rest.UpdateMessage(chanID, msgID, discord.MessageUpdate{
		Embeds:      &embeds,
		Files:       []*discord.File{discord.NewFile("beds.png", "", bytes.NewReader(imgBytes))},
		Attachments: &emptyAttachments,
		Components:  &components,
	})
	return err
}

func buildBedButtons(assignments []models.BedAssignment) []discord.LayoutComponent {
	if len(assignments) == 0 {
		return []discord.LayoutComponent{}
	}

	sort.Slice(assignments, func(i, j int) bool {
		return assignments[i].BedLetter < assignments[j].BedLetter
	})

	var buttons []discord.InteractiveComponent
	for _, a := range assignments {
		label := fmt.Sprintf("Lit %s", a.BedLetter)

		style := discord.ButtonStyleSecondary
		if a.UnderArrest || a.Death {
			style = discord.ButtonStyleDanger
		}

		buttons = append(buttons, discord.ButtonComponent{
			Label:    label,
			Style:    style,
			CustomID: "lsmsBed--" + a.BedLetter,
		})
	}

	return helpers.BuildActionRows(buttons, 5)
}
