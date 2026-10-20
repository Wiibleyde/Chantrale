package statistics

import (
	"time"

	"LsmsBot/internal/bot/helpers"
	"LsmsBot/internal/logger"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

const leaderboardSize = 10

var Commands = []discord.ApplicationCommandCreate{
	discord.SlashCommandCreate{
		Name:        "stats",
		Description: "Consulter les statistiques du LSMS",
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionSubCommand{
				Name:        "top",
				Description: "Afficher le classement des membres",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:        "critere",
						Description: "Critère du classement",
						Required:    true,
						Choices:     criterionChoices(),
					},
					discord.ApplicationCommandOptionString{
						Name:        "periode",
						Description: "Période analysée (7 jours par défaut)",
						Required:    false,
						Choices:     periodChoices(),
					},
				},
			},
		},
	},
}

func criterionChoices() []discord.ApplicationCommandOptionChoiceString {
	choices := make([]discord.ApplicationCommandOptionChoiceString, len(criteria))
	for i, c := range criteria {
		choices[i] = discord.ApplicationCommandOptionChoiceString{Name: c.Label, Value: c.Key}
	}
	return choices
}

func periodChoices() []discord.ApplicationCommandOptionChoiceString {
	choices := make([]discord.ApplicationCommandOptionChoiceString, len(periods))
	for i, p := range periods {
		choices[i] = discord.ApplicationCommandOptionChoiceString{Name: p.Label, Value: p.Key}
	}
	return choices
}

func HandleCommand(e *events.ApplicationCommandInteractionCreate) {
	data := e.SlashCommandInteractionData()
	if data.SubCommandName == nil {
		return
	}
	switch *data.SubCommandName {
	case "top":
		handleTop(e)
	}
}

func handleTop(e *events.ApplicationCommandInteractionCreate) {
	data := e.SlashCommandInteractionData()

	c, ok := findCriterion(data.String("critere"))
	if !ok {
		helpers.RespondEphemeral(e, "Critère de classement inconnu.")
		return
	}
	periodKey, hasPeriod := data.OptString("periode")
	if !hasPeriod {
		periodKey = defaultPeriodKey
	}
	p, ok := findPeriod(periodKey)
	if !ok {
		helpers.RespondEphemeral(e, "Période inconnue.")
		return
	}

	if err := e.DeferCreateMessage(true); err != nil {
		logger.Error("Error deferring stats interaction", "error", err)
		return
	}

	now := time.Now()
	since := p.Since(now)
	guildID := e.GuildID().String()

	var entries []RankEntry
	var err error
	if c.Duration {
		entries, err = rankByServiceTime(guildID, c.RoleType, since, now)
	} else {
		entries, err = rankByEventCount(guildID, c.EventType, since)
	}
	if err != nil {
		logger.Error("Error computing stats leaderboard", "error", err, "guildID", guildID, "criterion", c.Key)
		helpers.RespondFollowupEphemeral(e.Client(), e.ApplicationID(), e.Token(), "Erreur lors du calcul des statistiques.")
		return
	}

	components := BuildLeaderboardComponents(c, p, since, entries)
	msg := discord.NewMessageCreateV2(components...).WithEphemeral(true)
	if _, err := e.Client().Rest.CreateFollowupMessage(e.ApplicationID(), e.Token(), msg); err != nil {
		logger.Error("Error sending stats leaderboard", "error", err)
	}
}
