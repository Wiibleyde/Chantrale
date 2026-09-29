package labo

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"LsmsBot/internal/bot/helpers"
	"LsmsBot/internal/logger"
	"LsmsBot/internal/stats"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

var Commands = []discord.ApplicationCommandCreate{
	discord.SlashCommandCreate{
		Name:        "labo",
		Description: "Effectuer une analyse de laboratoire",
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionSubCommand{
				Name:        "bloodgroup",
				Description: "Analyse de groupe sanguin",
				Options:     commonLaboOptions(bloodgroupChoices()),
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "diseases",
				Description: "Analyse de maladies",
				Options:     commonLaboOptions(diseasesChoices()),
			},
		},
	},
}

func commonLaboOptions(resultChoices []discord.ApplicationCommandOptionChoiceString) []discord.ApplicationCommandOption {
	return []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{Name: "nom_prenom", Description: "Nom et prénom du patient", Required: true},
		discord.ApplicationCommandOptionString{Name: "resultat", Description: "Résultat de l'analyse (optionnel)", Required: false, Choices: resultChoices},
		discord.ApplicationCommandOptionString{Name: "time", Description: "Durée de l'analyse en minutes (optionnel, 1-3 chiffres)", Required: false},
	}
}

func bloodgroupChoices() []discord.ApplicationCommandOptionChoiceString {
	return []discord.ApplicationCommandOptionChoiceString{
		{Name: "O+", Value: "O+"}, {Name: "A+", Value: "A+"}, {Name: "B+", Value: "B+"}, {Name: "AB+", Value: "AB+"},
		{Name: "O-", Value: "O-"}, {Name: "A-", Value: "A-"}, {Name: "B-", Value: "B-"}, {Name: "AB-", Value: "AB-"},
	}
}

func diseasesChoices() []discord.ApplicationCommandOptionChoiceString {
	return []discord.ApplicationCommandOptionChoiceString{
		{Name: "Négatif", Value: "Négatif"}, {Name: "Positif", Value: "Positif"},
	}
}

func HandleCommand(e *events.ApplicationCommandInteractionCreate) {
	data := e.SlashCommandInteractionData()
	if data.SubCommandName == nil {
		return
	}
	subName := *data.SubCommandName

	if err := e.DeferCreateMessage(true); err != nil {
		logger.Error("Error deferring", "error", err)
		return
	}

	patientName := data.String("nom_prenom")

	analyseType := subName

	analyseTime := defaultTime(subName)
	if timeStr, ok := data.OptString("time"); ok {
		if t, err := strconv.Atoi(timeStr); err == nil && t > 0 && t < 999 {
			analyseTime = t
		}
	}

	var result string
	if res, ok := data.OptString("resultat"); ok {
		result = res
	} else {
		result = randomResult(subName)
	}

	member := e.Member()
	if member == nil {
		return
	}

	channelID := e.Channel().ID()
	entry := &LaboEntry{
		GuildID:   e.GuildID().String(),
		ChannelID: channelID,
		UserID:    member.User.ID,
		StartTime: time.Now(),
		Name:      patientName,
		Type:      analyseType,
		Result:    result,
		Time:      analyseTime,
	}

	waitComponents := BuildLaboWaitingComponents(entry)

	msg, err := e.Client().Rest.CreateMessage(channelID, discord.NewMessageCreateV2(waitComponents...))
	if err != nil {
		logger.Error("Error sending labo message", "error", err)
		helpers.RespondFollowupEphemeral(e.Client(), e.ApplicationID(), e.Token(), "Erreur lors de l'envoi du message d'analyse.")
		return
	}

	entry.MessageID = msg.ID
	Queue.Add(entry)

	_, resultPreset := data.OptString("resultat")
	stats.Record(e.GuildID().String(), member.User.ID.String(), "labo.test_start", map[string]any{
		"test_type":        analyseType,
		"patient_name":     patientName,
		"duration_minutes": analyseTime,
		"result_preset":    resultPreset,
	})

	helpers.RespondFollowupEphemeral(e.Client(), e.ApplicationID(), e.Token(), fmt.Sprintf("Analyse lancée. Résultat dans %d minute(s).", analyseTime))
}

func defaultTime(subCmd string) int {
	switch subCmd {
	case "bloodgroup":
		return 5
	case "diseases":
		return 10
	default:
		return 5
	}
}

func randomResult(subCmd string) string {
	type weighted struct {
		value  string
		weight int
	}

	var pool []weighted

	switch subCmd {
	case "bloodgroup":
		pool = []weighted{
			{"O+", 34}, {"A+", 28}, {"B+", 20}, {"AB+", 2},
			{"O-", 7}, {"A-", 6}, {"B-", 2}, {"AB-", 1},
		}
	case "diseases":
		return "Négatif"
	default:
		return "Négatif"
	}

	total := 0
	for _, w := range pool {
		total += w.weight
	}

	r := rand.Intn(total)
	cumulative := 0
	for _, w := range pool {
		cumulative += w.weight
		if r < cumulative {
			return w.value
		}
	}

	return pool[len(pool)-1].value
}
