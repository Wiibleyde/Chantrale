# Chantrale Bot — Project Conventions

The bot is written in Go with [disgo](https://github.com/disgoorg/disgo) (`github.com/disgoorg/disgo`), GORM on PostgreSQL, and a REST API in `internal/api/`. The module path is `LsmsBot`.

## Architecture: Feature Module Structure

Each feature lives in `internal/bot/features/{name}/` with these files:

| File | Responsibility |
|------|---------------|
| `register.go` | Wires handlers to the router — only file that imports the router |
| `command.go` | Slash command definitions (`var Commands`) and handler logic |
| `embeds.go` | Message builder functions (`BuildXComponents(...)` / `BuildXEmbed(...)`) |
| `buttons.go` | Button / select menu interaction handlers |
| `modals.go` | Modal submission handlers |
| `event.go` | Gateway event listeners (optional) |
| `scheduler.go` | Background goroutine logic (optional) |
| `queue.go` | In-memory queue/state management (optional) |
| `query.go` | Non-trivial database reads / aggregation (optional) |

Other single-purpose files are fine when a concern doesn't fit the table (e.g. `duty/tracker.go`, `bed/imageGenerator.go`, `statistics/criteria.go`).

**Do not put multiple concerns in the same file.** Keep builders in `embeds.go`, handlers in `command.go` / `buttons.go`, etc.

The package name must not collide with an internal package the feature imports (e.g. the `/stats` feature is `statistics` because `LsmsBot/internal/stats` already exists).

## Adding a New Feature

1. Create the `internal/bot/features/{name}/` directory with the files above.
2. Define the commands slice in `command.go`:

```go
var Commands = []discord.ApplicationCommandCreate{
	discord.SlashCommandCreate{
		Name:        "myfeature",
		Description: "Description en français",
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionSubCommand{
				Name:        "init",
				Description: "Initialiser le panneau",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{Name: "nom", Description: "Nom", Required: true},
				},
			},
		},
	},
}
```

3. Dispatch subcommands in `HandleCommand`:

```go
func HandleCommand(e *events.ApplicationCommandInteractionCreate) {
	data := e.SlashCommandInteractionData()
	if data.SubCommandName == nil {
		return
	}
	switch *data.SubCommandName {
	case "init":
		handleInit(e)
	}
}
```

4. Register handlers in `register.go`:

```go
func Register(r *router.Router) {
	r.OnCommand("myfeature", HandleCommand)
	r.OnButton("myfeatureBtn", HandleButton)
	r.OnButtonPrefix("myfeature--", HandleButtonPrefix)
	r.OnModal("myfeatureModal--", HandleModal)
}
```

5. In `internal/bot/bot.go`, add `myfeature.Register(r)` and append `myfeature.Commands` to `allCommands`. **Never modify the router itself.**

Gateway listeners that are not interactions (e.g. member updates) are added directly in `bot.go` with `client.AddEventListeners(bot.NewListenerFunc(myfeature.HandleX))`.

## Handler Signatures

All handlers take a single disgo event:

```go
// Slash commands
func HandleCommand(e *events.ApplicationCommandInteractionCreate)

// Buttons and select menus
func HandleButton(e *events.ComponentInteractionCreate)

// Modal submissions
func HandleModal(e *events.ModalSubmitInteractionCreate)

// Gateway events
func HandleGuildMemberUpdate(e *events.GuildMemberUpdate)
```

The REST client is reached through `e.Client().Rest`. Background code that needs it receives a `*bot.Client`.

## Router Registration Methods

| Method | Use case |
|--------|----------|
| `r.OnCommand(name, handler)` | Exact slash command name match |
| `r.OnButton(id, handler)` | Exact component `CustomID` match (buttons and select menus) |
| `r.OnButtonPrefix(prefix, handler)` | Prefix-based component routing |
| `r.OnModal(prefix, handler)` | Modal `CustomID` prefix routing |

## Message Building

### Components V2 (preferred for new messages)

Wrap content in `embeds.NewContainerV2(color, ...)`, which appends the standard footer (bot name + timestamp):

```go
func BuildMyComponents(name string) []discord.LayoutComponent {
	return []discord.LayoutComponent{
		embeds.NewContainerV2(0x0099FF,
			discord.NewTextDisplay("## 📋 Mon titre"),
			discord.NewLargeSeparator(),
			discord.NewTextDisplay(fmt.Sprintf("**Nom :** %s", name)),
			discord.NewSmallSeparator(),
			discord.NewActionRow(discord.NewPrimaryButton("Valider", "myfeatureBtn")),
		),
	}
}
```

Send / update with `discord.NewMessageCreateV2(components...)` and `discord.NewMessageUpdateV2(components...)`. Add `.WithEphemeral(true)` for an ephemeral V2 response.

### Classic embeds

Some panels (`bed`, `mortuary`, `doctor`) still use classic embeds. When you do, always start from `embeds.BaseEmbed()` — never construct a `discord.Embed` from scratch:

```go
embed := embeds.BaseEmbed()
embed.Title = "Mon titre"
embed.Description = "Ma description"
embed.Color = 0x0099FF
```

Builder functions are named `BuildXComponents(...)` / `BuildXEmbed(...)` and live in `embeds.go`.

**Standard color palette:**

| Meaning | Hex |
|---------|-----|
| Informational / management | `0x0099FF` |
| Positive / active / success | `0x00FF00` |
| Negative / leave / error | `0xFF0000` |
| In-progress / pending | `0xFF8800` |
| Summary / neutral / statistics | `0x5865F2` |
| Default / white | `0xFFFFFF` |

## Responding to Interactions

Use the helpers in `LsmsBot/internal/bot/helpers`:

```go
helpers.RespondEphemeral(e, "Message en français.")

// Permission gate (sends the ephemeral error itself)
if !helpers.RequirePermission(e, discord.PermissionManageChannels, "Vous n'avez pas la permission de gérer les canaux.") {
	return
}
```

For slow handlers (database aggregation, REST calls), defer first, then answer with a follow-up:

```go
if err := e.DeferCreateMessage(true); err != nil {
	logger.Error("Error deferring interaction", "error", err)
	return
}
// ...
helpers.RespondFollowupEphemeral(e.Client(), e.ApplicationID(), e.Token(), "Terminé.")
```

## Error Handling

Log the technical error, then respond to the user in French via an ephemeral message:

```go
if err != nil {
	logger.Error("Context message", "error", err)
	helpers.RespondEphemeral(e, "Message d'erreur en français pour l'utilisateur.")
	return
}
```

- Never expose raw error strings to the user.
- Always `return` after an error response.
- Use `logger.Error` for unexpected errors, `logger.Warn` for degraded-but-recoverable states.

## Logger Usage

Use the project's custom slog-based logger from `LsmsBot/internal/logger`. Pass structured key-value pairs after the message:

```go
logger.Debug("Processing request", "guildID", guildID)
logger.Info("Feature registered", "feature", "duty")
logger.Warn("Config value missing", "key", "LOGS_CHANNEL_ID")
logger.Error("Database query failed", "error", err, "guildID", guildID)
logger.Fatal("Cannot connect to database", "error", err) // logs then exits
```

Never use `fmt.Println` or `log.Print` for operational logging.

## User-Facing Language

**All user-facing text must be in French.** This includes:
- Slash command and option descriptions, choice names
- Message titles and text
- Ephemeral error/success messages
- Button labels
- Modal titles and field labels

Internal code (variable names, comments, log messages) may be in English.

## Database Patterns

The global `database.DB` (`*gorm.DB`, PostgreSQL) is the single database handle. Import `LsmsBot/internal/database` and the models from `LsmsBot/internal/database/models`.

```go
// Read
var dm models.DutyManager
err := database.DB.Where("guild_id = ?", guildID).First(&dm).Error

// Create
err := database.DB.Create(&dm).Error

// Delete
err := database.DB.Delete(&dm).Error

// Fetch all
var dms []models.DutyManager
err := database.DB.Find(&dms).Error
```

Check `.Error` directly on the returned `*gorm.DB` — do not chain calls and then check error later. Building a query conditionally before executing it is fine.

## Database Models

Models live in `internal/database/models/` and must be added to `AutoMigrate` in `internal/database/database.go`. Use `*string` (pointer) for nullable / optional fields:

```go
type MyModel struct {
	ID            uint   `gorm:"primaryKey;autoIncrement"`
	GuildID       string `gorm:"index"`
	OptionalField *string // nullable
}
```

## Statistics Events

User actions are recorded asynchronously in the `stat_events` table (`models.StatEvent`: guild, user, event type, JSONB payload, timestamp) via `LsmsBot/internal/stats`:

```go
stats.Record(guildID.String(), e.Member().User.ID.String(), "myfeature.action", map[string]any{
	"name": name,
})
```

- Event types are `feature.action` in snake_case (e.g. `bed.assign`, `duty.role_take`).
- Record only after the action has succeeded.
- Keep payload keys stable: the `/stats` feature (`internal/bot/features/statistics/`) reads them. Changing or removing a key/event type means updating its queries.

## Naming Conventions

| Symbol | Convention | Example |
|--------|-----------|---------|
| Package | lowercase, English, single word | `duty`, `radio`, `labo` |
| Exported functions | PascalCase, verb-first | `HandleCommand`, `BuildDutyComponents`, `Register` |
| Handler functions | `HandleX` | `HandleDutyButton` |
| Message builders | `BuildX` | `BuildDutyComponents`, `BuildBedEmbed` |
| Command var | `Commands` | `var Commands = []discord.ApplicationCommandCreate{…}` |
| Local variables | camelCase | `guildID`, `roleID`, `dutyManager` |
| Component CustomIDs | camelCase prefix, `--` before the payload | `"lsmsRadioEdit--<payload>"` |

## Component CustomID Encoding

Use `--` as a separator between the prefix and the payload. Encode arbitrary data (e.g. names) in Base64 URL encoding to avoid separator collisions, and read it back with `helpers.SuffixFromCustomID`:

```go
customID := "lsmsRadioEdit--" + base64.RawURLEncoding.EncodeToString([]byte(name))
```

## Discord Mentions

Reference users with `<@userID>`, roles with `<@&roleID>`, channels with `<#channelID>` and timestamps with `<t:unix:f>`:

```go
text := fmt.Sprintf("En service : <@%s>", userID)
fieldValue := fmt.Sprintf("<@&%s>", roleID)
```

## Concurrency

For in-memory state shared across goroutines, embed a `sync.Mutex` and call `mu.Lock()` / `defer mu.Unlock()` at the top of each method:

```go
type MyQueue struct {
	mu      sync.Mutex
	entries []*MyEntry
}

func (q *MyQueue) Add(e *MyEntry) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.entries = append(q.entries, e)
}
```
