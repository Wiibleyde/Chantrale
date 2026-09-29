package labo

import (
	"fmt"

	"LsmsBot/internal/bot/embeds"

	"github.com/disgoorg/disgo/discord"
)

func GetAnalyseTypeName(t string) string {
	switch t {
	case "bloodgroup":
		return "Groupe Sanguin"
	case "diseases":
		return "Maladies"
	default:
		return t
	}
}

func BuildLaboWaitingComponents(entry *LaboEntry) []discord.LayoutComponent {
	return []discord.LayoutComponent{
		embeds.NewContainerV2(0xFF8800,
			discord.NewTextDisplay("## ⏳ Analyse en cours"),
			discord.NewSmallSeparator(),
			discord.NewTextDisplay(fmt.Sprintf("**Type :** %s", GetAnalyseTypeName(entry.Type))),
			discord.NewTextDisplay(fmt.Sprintf("**Patient :** %s", entry.Name)),
			discord.NewTextDisplay(fmt.Sprintf("**Demandeur :** <@%s>", entry.UserID)),
			discord.NewTextDisplay(fmt.Sprintf("**Durée estimée :** %d minute(s)", entry.Time)),
			discord.NewLargeSeparator(),
			discord.NewActionRow(
				discord.NewDangerButton("Annuler", "laboCancelButton"),
			),
		),
	}
}

func BuildLaboResultComponents(entry *LaboEntry) []discord.LayoutComponent {
	return []discord.LayoutComponent{
		embeds.NewContainerV2(0x00FF00,
			discord.NewTextDisplay("## ✅ Analyse terminée"),
			discord.NewSmallSeparator(),
			discord.NewTextDisplay(fmt.Sprintf("**Type :** %s", GetAnalyseTypeName(entry.Type))),
			discord.NewTextDisplay(fmt.Sprintf("**Résultat :** %s", entry.Result)),
			discord.NewTextDisplay(fmt.Sprintf("**Patient :** %s", entry.Name)),
			discord.NewTextDisplay(fmt.Sprintf("**Demandeur :** <@%s>", entry.UserID)),
		),
	}
}

func BuildLaboCancelledComponents(entry *LaboEntry) []discord.LayoutComponent {
	return []discord.LayoutComponent{
		embeds.NewContainerV2(0xFF0000,
			discord.NewTextDisplay("## ❌ Analyse annulée"),
			discord.NewSmallSeparator(),
			discord.NewTextDisplay(fmt.Sprintf("**Type :** %s", GetAnalyseTypeName(entry.Type))),
			discord.NewTextDisplay(fmt.Sprintf("**Patient :** %s", entry.Name)),
			discord.NewTextDisplay(fmt.Sprintf("**Demandeur :** <@%s>", entry.UserID)),
		),
	}
}
