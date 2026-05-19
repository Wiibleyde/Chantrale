package duty

import (
	"fmt"
	"strings"
	"time"

	"LsmsBot/internal/bot/embeds"

	"github.com/disgoorg/disgo/discord"
)

func BuildDutyComponents(onDuty, onCall, offRadio []string) []discord.LayoutComponent {
	dutyList := formatList(onDuty, "Personne n'est en service :(")
	onCallList := formatList(onCall, "Personne n'est en semi service :(")
	offRadioList := formatList(offRadio, "Personne n'est off radio")

	return []discord.LayoutComponent{
		embeds.NewContainerV2(0x0099FF,
			discord.NewTextDisplay("## 📋 Gestionnaire de service"),
			discord.NewLargeSeparator(),
			discord.NewTextDisplay(fmt.Sprintf("**🟢 En service (%d) :**\n%s", len(onDuty), dutyList)),
			discord.NewTextDisplay(fmt.Sprintf("**🟡 En semi service (%d) :**\n%s", len(onCall), onCallList)),
			discord.NewTextDisplay(fmt.Sprintf("**🔴 Off radio (%d) :**\n%s", len(offRadio), offRadioList)),
			discord.NewSmallSeparator(),
			discord.NewActionRow(
				discord.NewPrimaryButton("Prendre/Quitter le service", "handleLsmsDuty"),
				discord.NewSecondaryButton("Prendre/Quitter le semi service", "handleLsmsOnCall"),
				discord.NewDangerButton("Off radio", "handleLsmsOffRadio"),
			),
		),
	}
}

func BuildDutyUpdateComponents(displayName string, take bool) []discord.LayoutComponent {
	if take {
		return []discord.LayoutComponent{
			embeds.NewContainerV2(0x00FF00,
				discord.NewTextDisplay("## ✅ Prise de service"),
				discord.NewTextDisplay(fmt.Sprintf("**%s** a pris le service.", displayName)),
			),
		}
	}
	return []discord.LayoutComponent{
		embeds.NewContainerV2(0xFF0000,
			discord.NewTextDisplay("## 🔴 Fin de service"),
			discord.NewTextDisplay(fmt.Sprintf("**%s** a quitté le service.", displayName)),
		),
	}
}

func BuildOnCallUpdateComponents(displayName string, take bool) []discord.LayoutComponent {
	if take {
		return []discord.LayoutComponent{
			embeds.NewContainerV2(0x00FF00,
				discord.NewTextDisplay("## ✅ Début du semi service"),
				discord.NewTextDisplay(fmt.Sprintf("**%s** a pris le semi service.", displayName)),
			),
		}
	}
	return []discord.LayoutComponent{
		embeds.NewContainerV2(0xFF0000,
			discord.NewTextDisplay("## 🔴 Fin du semi service"),
			discord.NewTextDisplay(fmt.Sprintf("**%s** a quitté le semi service.", displayName)),
		),
	}
}

func BuildOffRadioUpdateComponents(displayName string, take bool) []discord.LayoutComponent {
	if take {
		return []discord.LayoutComponent{
			embeds.NewContainerV2(0xFF8800,
				discord.NewTextDisplay("## 📻 Passage off radio"),
				discord.NewTextDisplay(fmt.Sprintf("**%s** est passé off radio.", displayName)),
			),
		}
	}
	return []discord.LayoutComponent{
		embeds.NewContainerV2(0x00FF00,
			discord.NewTextDisplay("## ✅ Fin du off radio"),
			discord.NewTextDisplay(fmt.Sprintf("**%s** est revenu sur la radio.", displayName)),
		),
	}
}

func BuildSummaryComponents(from, to time.Time, onDuty, onCall, offRadio []string) []discord.LayoutComponent {
	return []discord.LayoutComponent{
		embeds.NewContainerV2(0x5865F2,
			discord.NewTextDisplay("## 📊 Récapitulatif du service"),
			discord.NewTextDisplay(fmt.Sprintf("Période du <t:%d:f> au <t:%d:f>", from.Unix(), to.Unix())),
			discord.NewSmallSeparator(),
			discord.NewTextDisplay("**Service :**\n"+formatList(onDuty, "Aucun :(")),
			discord.NewTextDisplay("**Semi service :**\n"+formatList(onCall, "Aucun :(")),
			discord.NewTextDisplay("**Off radio :**\n"+formatList(offRadio, "Aucun")),
		),
	}
}

func formatList(names []string, empty string) string {
	if len(names) == 0 {
		return empty
	}

	var sb strings.Builder
	for _, name := range names {
		sb.WriteString("- ");sb.WriteString(name);sb.WriteString("\n")
	}
	return sb.String()
}
