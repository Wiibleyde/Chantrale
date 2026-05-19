# Chantrale Bot — Project Conventions

## Architecture: Feature Module Structure

Each feature lives in `internal/bot/features/{name}/` with these files:

| File | Responsibility |
|------|---------------|
| `register.go` | Wires handlers to the router — only file that imports the router |
| `command.go` | Slash command definitions (`var Commands`) and handler logic |
| `embeds.go` | Embed builder functions (`BuildXEmbed(...)`) |
| `buttons.go` | Button / component interaction handlers |
| `modals.go` | Modal submission handlers |
| `event.go` | Guild event listeners (optional) |
| `scheduler.go` | Background goroutine logic (optional) |
| `queue.go` | In-memory queue/state management (optional) |

**Do not put multiple concerns in the same file.** Keep embed builders in `embeds.go`, handlers in `command.go` / `buttons.go`, etc.

## Adding a New Feature

1. Create the `internal/bot/features/{name}/` directory with the files above.
2. Define the commands slice in `command.go`:

```go
var Commands = []*discordgo.ApplicationCommand{
    {
        Name:        "myfeature",
        Description: "Description en français",
        Options: []*discordgo.ApplicationCommandOption{ /* subcommands */ },
    },
}
```

3. Register handlers in `register.go`:

```go
func Register(r *router.Router) {
    r.OnCommand("myfeature", HandleCommand)
    r.OnButton("myfeatureBtn", HandleButton)
    r.OnButtonPrefix("myfeature--", HandleButtonPrefix)
    r.OnModal("myfeatureModal--", HandleModal)
}
```

4. In `internal/bot/bot.go`, add `myfeature.Register(r)` and append `myfeature.Commands` to the guild command list. **Never modify the router itself.**

## Handler Signatures

All Discord event handlers use these exact signatures:

```go
// Slash commands and interactions
func HandleCommand(s *discordgo.Session, i *discordgo.InteractionCreate)

// Guild member updates
func HandleMemberUpdate(s *discordgo.Session, m *discordgo.GuildMemberUpdate)
```

## Router Registration Methods

| Method | Use case |
|--------|----------|
| `r.OnCommand(name, handler)` | Exact slash command name match |
| `r.OnButton(id, handler)` | Exact button `CustomID` match |
| `r.OnButtonPrefix(prefix, handler)` | Prefix-based button routing |
| `r.OnModal(prefix, handler)` | Modal `CustomID` routing |

## Embed System

Always start from `embeds.BaseEmbed()` — never construct a `discordgo.MessageEmbed` from scratch:

```go
embed := embeds.BaseEmbed()
embed.Title = "Mon titre"
embed.Description = "Ma description"
embed.Color = 0x0099FF
```

**Standard color palette:**

| Meaning | Hex |
|---------|-----|
| Informational / management | `0x0099FF` |
| Positive / active / success | `0x00FF00` |
| Negative / leave / error | `0xFF0000` |
| In-progress / pending | `0xFF8800` |
| Summary / neutral | `0x5865F2` |
| Default / white | `0xFFFFFF` |

Embed builder functions are named `BuildXEmbed(...)` and live in `embeds.go`.

## Error Handling

Use this exact pattern — log the technical error, then respond to the user in French via an ephemeral message:

```go
if err != nil {
    logger.Error("Context message", "error", err)
    respondEphemeral(s, i, "Message d'erreur en français pour l'utilisateur.")
    return
}
```

- Never expose raw error strings to the user.
- Always `return` after an error response.
- Use `logger.Error` for unexpected errors, `logger.Warn` for degraded-but-recoverable states.

## Logger Usage

Use the project's custom slog-based logger from `internal/logger/`. Pass structured key-value pairs after the message:

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
- Slash command descriptions
- Embed titles and descriptions
- Ephemeral error/success messages
- Button labels
- Modal titles and field labels

Internal code (variable names, comments, log messages) may be in English.

## Database Patterns

The global `database.DB` (`*gorm.DB`) is the single database handle. Import `LsmsBot/internal/database`.

```go
// Read
var dm database.DutyManager
err := database.DB.Where("guild_id = ?", guildID).First(&dm).Error

// Create
err := database.DB.Create(&dm).Error

// Delete
err := database.DB.Delete(&dm).Error

// Fetch all
var dms []database.DutyManager
err := database.DB.Find(&dms).Error
```

Check `.Error` directly on the returned `*gorm.DB` — do not chain calls and then check error later.

## Database Models

Models live in `internal/database/models/`. Use `*string` (pointer) for nullable / optional fields:

```go
type MyModel struct {
    ID      uint    `gorm:"primaryKey;autoIncrement"`
    GuildID string  `gorm:"index"`
    OptionalField *string // nullable
}
```

## Naming Conventions

| Symbol | Convention | Example |
|--------|-----------|---------|
| Package | lowercase, English, single word | `duty`, `radio`, `labo` |
| Exported functions | PascalCase, verb-first | `HandleCommand`, `BuildEmbed`, `Register` |
| Handler functions | `HandleX` | `HandleDutyButton` |
| Embed builders | `BuildX` | `BuildDutyEmbed` |
| Command var | `Commands` | `var Commands = []*discordgo.ApplicationCommand{…}` |
| Local variables | camelCase | `guildID`, `roleID`, `dutyManager` |
| Button CustomIDs | snake_case with `--` separator | `"lsmsRadio--editFreq"` |

## Button CustomID Encoding

Use `--` as a separator between the prefix and the payload. Encode arbitrary data (e.g. names) in Base64 to avoid separator collisions:

```go
customID := fmt.Sprintf("myfeature--%s", base64.StdEncoding.EncodeToString([]byte(name)))
```

## Discord Mentions in Embeds

Reference users with `<@userID>` and roles with `<@&roleID>`:

```go
embed.Description = fmt.Sprintf("En service : <@%s>", userID)
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
