package statistics

import "LsmsBot/internal/bot/router"

func Register(r *router.Router) {
	r.OnCommand("stats", HandleCommand)
}
