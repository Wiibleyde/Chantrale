package statistics

import (
	"fmt"
	"strings"
	"time"

	"LsmsBot/internal/bot/embeds"

	"github.com/disgoorg/disgo/discord"
)

const barLength = 10

var medals = []string{"🥇", "🥈", "🥉"}

func BuildLeaderboardComponents(c Criterion, p Period, since time.Time, entries []RankEntry) []discord.LayoutComponent {
	periodText := "Période : " + p.Description
	if !since.IsZero() {
		periodText += fmt.Sprintf(" (depuis <t:%d:f>)", since.Unix())
	}

	if len(entries) == 0 {
		return []discord.LayoutComponent{
			embeds.NewContainerV2(0x5865F2,
				discord.NewTextDisplay("## 🏆 Classement — "+c.Title),
				discord.NewTextDisplay("-# "+periodText),
				discord.NewLargeSeparator(),
				discord.NewTextDisplay("Aucune donnée sur cette période."),
			),
		}
	}

	var total int64
	for _, entry := range entries {
		total += entry.Value
	}

	shown := entries
	if len(shown) > leaderboardSize {
		shown = shown[:leaderboardSize]
	}
	best := shown[0].Value

	var sb strings.Builder
	for i, entry := range shown {
		rank := fmt.Sprintf("`#%d`", i+1)
		if i < len(medals) {
			rank = medals[i]
		}
		fmt.Fprintf(&sb, "%s <@%s> — **%s**\n-# %s\n", rank, entry.UserID, formatValue(c, entry.Value), progressBar(entry.Value, best))
	}

	members := "membre"
	if len(entries) > 1 {
		members = "membres"
	}
	summary := fmt.Sprintf("**Total :** %s · **%d** %s", formatValue(c, total), len(entries), members)

	return []discord.LayoutComponent{
		embeds.NewContainerV2(0x5865F2,
			discord.NewTextDisplay("## 🏆 Classement — "+c.Title),
			discord.NewTextDisplay("-# "+periodText),
			discord.NewLargeSeparator(),
			discord.NewTextDisplay(strings.TrimSuffix(sb.String(), "\n")),
			discord.NewSmallSeparator(),
			discord.NewTextDisplay(summary),
		),
	}
}

func formatValue(c Criterion, value int64) string {
	if c.Duration {
		return formatDuration(time.Duration(value) * time.Second)
	}
	unit := c.Singular
	if value > 1 {
		unit = c.Plural
	}
	return fmt.Sprintf("%d %s", value, unit)
}

func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if hours == 0 {
		return fmt.Sprintf("%d min", minutes)
	}
	return fmt.Sprintf("%d h %02d min", hours, minutes)
}

func progressBar(value, best int64) string {
	filled := barLength
	if best > 0 {
		filled = int((value*barLength + best - 1) / best)
	}
	return strings.Repeat("▰", filled) + strings.Repeat("▱", barLength-filled)
}
