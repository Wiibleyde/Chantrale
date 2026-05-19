package radio

import (
	"encoding/base64"
	"strings"

	"LsmsBot/internal/bot/helpers"
	"LsmsBot/internal/logger"
	"LsmsBot/internal/stats"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

func HandleRadioAddModal(e *events.ModalSubmitInteractionCreate) {
	customID := e.Data.CustomID
	parts := strings.SplitN(customID, "--", 3)
	if len(parts) < 3 {
		helpers.RespondEphemeral(e, "Erreur: identifiant invalide.")
		return
	}

	channelIDStr := parts[1]
	messageIDStr := parts[2]

	name := e.Data.Text("name")
	frequency := e.Data.Text("frequency")

	chanID, err := snowflake.Parse(channelIDStr)
	if err != nil {
		helpers.RespondEphemeral(e, "Erreur: identifiant de salon invalide.")
		return
	}
	msgID, err := snowflake.Parse(messageIDStr)
	if err != nil {
		helpers.RespondEphemeral(e, "Erreur: identifiant de message invalide.")
		return
	}

	msg, err := e.Client().Rest.GetMessage(chanID, msgID)
	if err != nil {
		logger.Error("Error fetching message", "error", err)
		helpers.RespondEphemeral(e, "Erreur lors de la récupération du message.")
		return
	}

	radios := ParseRadiosFromComponents(msg.Components)
	radios = append(radios, RadioEntry{Name: name, Frequency: frequency})

	components := BuildRadioComponents(radios)
	if _, err := e.Client().Rest.UpdateMessage(chanID, msgID, discord.NewMessageUpdateV2(components...)); err != nil {
		logger.Error("Error editing radio message", "error", err)
		helpers.RespondEphemeral(e, "Erreur lors de la modification du message.")
		return
	}

	guildID := ""
	if g := e.GuildID(); g != nil {
		guildID = g.String()
	}
	userID := ""
	if m := e.Member(); m != nil {
		userID = m.User.ID.String()
	}
	stats.Record(guildID, userID, "radio.add", map[string]any{
		"name":      name,
		"frequency": frequency,
	})

	helpers.RespondEphemeral(e, "Radio ajoutée avec succès.")
}

func HandleRadioEditModal(e *events.ModalSubmitInteractionCreate) {
	customID := e.Data.CustomID
	parts := strings.SplitN(customID, "--", 4)
	if len(parts) < 4 {
		helpers.RespondEphemeral(e, "Erreur: identifiant invalide.")
		return
	}

	channelIDStr := parts[1]
	messageIDStr := parts[2]
	encodedName := parts[3]

	nameBytes, err := base64.RawURLEncoding.DecodeString(encodedName)
	if err != nil {
		helpers.RespondEphemeral(e, "Erreur: nom de radio invalide.")
		return
	}
	originalName := string(nameBytes)

	newName := e.Data.Text("name")
	newFreq := e.Data.Text("frequency")

	chanID, err := snowflake.Parse(channelIDStr)
	if err != nil {
		helpers.RespondEphemeral(e, "Erreur: identifiant de salon invalide.")
		return
	}
	msgID, err := snowflake.Parse(messageIDStr)
	if err != nil {
		helpers.RespondEphemeral(e, "Erreur: identifiant de message invalide.")
		return
	}

	msg, err := e.Client().Rest.GetMessage(chanID, msgID)
	if err != nil {
		logger.Error("Error fetching message", "error", err)
		helpers.RespondEphemeral(e, "Erreur lors de la récupération du message.")
		return
	}

	radios := ParseRadiosFromComponents(msg.Components)
	oldFreq := ""
	for idx, r := range radios {
		if r.Name == originalName {
			oldFreq = r.Frequency
			radios[idx] = RadioEntry{Name: newName, Frequency: newFreq}
			break
		}
	}

	components := BuildRadioComponents(radios)
	if _, err := e.Client().Rest.UpdateMessage(chanID, msgID, discord.NewMessageUpdateV2(components...)); err != nil {
		logger.Error("Error editing radio message", "error", err)
		helpers.RespondEphemeral(e, "Erreur lors de la modification du message.")
		return
	}

	guildID := ""
	if g := e.GuildID(); g != nil {
		guildID = g.String()
	}
	userID := ""
	if m := e.Member(); m != nil {
		userID = m.User.ID.String()
	}
	stats.Record(guildID, userID, "radio.edit", map[string]any{
		"old_name":      originalName,
		"new_name":      newName,
		"old_frequency": oldFreq,
		"new_frequency": newFreq,
	})

	helpers.RespondEphemeral(e, "Radio modifiée avec succès.")
}
