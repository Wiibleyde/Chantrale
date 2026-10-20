package bot

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"LsmsBot/internal/api"
	"LsmsBot/internal/bot/embeds"
	"LsmsBot/internal/bot/features/bed"
	"LsmsBot/internal/bot/features/doctor"
	"LsmsBot/internal/bot/features/duty"
	"LsmsBot/internal/bot/features/labo"
	"LsmsBot/internal/bot/features/mortuary"
	"LsmsBot/internal/bot/features/radio"
	"LsmsBot/internal/bot/features/statistics"
	"LsmsBot/internal/bot/router"
	"LsmsBot/internal/config"
	"LsmsBot/internal/logger"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/snowflake/v2"
)

func UndeployGlobalCommands() {
	cfg := config.Load()

	client, err := disgo.New(cfg.DiscordToken,
		bot.WithGatewayConfigOpts(
			gateway.WithIntents(gateway.IntentGuilds),
		),
	)
	if err != nil {
		logger.Fatal("Erreur lors de la création du client Discord", "error", err)
	}

	done := make(chan struct{})

	client.AddEventListeners(bot.NewListenerFunc(func(e *events.Ready) {
		defer close(done)
		if _, err := e.Client().Rest.SetGlobalCommands(e.Application.ID, []discord.ApplicationCommandCreate{}); err != nil {
			logger.Error("Impossible de supprimer les commandes globales", "error", err)
		} else {
			logger.Info("Commandes globales supprimées")
		}
	}))

	ctx := context.Background()
	if err := client.OpenGateway(ctx); err != nil {
		logger.Fatal("Erreur lors de l'ouverture de la connexion", "error", err)
	}
	defer client.Close(ctx)

	<-done
	logger.Info("Terminé")
}

func Run() {
	cfg := config.Load()

	client, err := disgo.New(cfg.DiscordToken,
		bot.WithGatewayConfigOpts(
			gateway.WithIntents(gateway.IntentGuilds, gateway.IntentGuildMembers),
		),
	)
	if err != nil {
		logger.Fatal("Error creating Discord client", "error", err)
	}

	labo.Queue.SetClient(client)

	client.AddEventListeners(bot.NewListenerFunc(duty.HandleGuildMemberUpdate))

	r := router.New()
	duty.Register(r)
	radio.Register(r)
	doctor.Register(r)
	labo.Register(r)
	bed.Register(r)
	mortuary.Register(r)
	statistics.Register(r)
	r.Attach(client)

	allCommands := []discord.ApplicationCommandCreate{}
	allCommands = append(allCommands, duty.Commands...)
	allCommands = append(allCommands, radio.Commands...)
	allCommands = append(allCommands, doctor.Commands...)
	allCommands = append(allCommands, labo.Commands...)
	allCommands = append(allCommands, bed.Commands...)
	allCommands = append(allCommands, mortuary.Commands...)
	allCommands = append(allCommands, statistics.Commands...)

	client.AddEventListeners(bot.NewListenerFunc(func(e *events.Ready) {
		logger.Info("Logged in", "user", e.User.Username)
		embeds.Init(e.User.EffectiveAvatarURL())
		duty.StartScheduler(client)
		duty.InitPresence(client)
		var guildSnowflakes []snowflake.ID
		for _, guildID := range cfg.GuildIDs {
			guildSnowflake := snowflake.MustParse(guildID)
			guildSnowflakes = append(guildSnowflakes, guildSnowflake)
			if _, err := e.Client().Rest.SetGuildCommands(e.Application.ID, guildSnowflake, allCommands); err != nil {
				logger.Error("Cannot set commands", "guild", guildID, "error", err)
			}
		}
		duty.WarmMemberRoleCache(client, guildSnowflakes)
		logger.Info("Commands registered")
	}))

	ctx := context.Background()
	if err := client.OpenGateway(ctx); err != nil {
		logger.Fatal("Error opening connection", "error", err)
	}
	defer client.Close(ctx)

	apiApp := api.Run(client)

	logger.Info("Bot is running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	logger.Info("Sending duty summary before shutdown...")
	duty.SendShutdownSummary(client)

	if apiApp != nil {
		if err := apiApp.Shutdown(); err != nil {
			logger.Error("Error shutting down API server", "error", err)
		}
	}

	logger.Info("Shutting down...")
}
