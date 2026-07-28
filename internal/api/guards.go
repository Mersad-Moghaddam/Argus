package api

import "github.com/gofiber/fiber/v2"

// guarded prepends route-scoped middleware to a handler.
//
// The legacy website API and the project API use different authentication
// schemes but share the /api prefix. Mounting the legacy X-API-Key middleware
// with fiber's Group("", mw) would attach it to the whole /api subtree — and
// therefore to the bearer-authenticated project endpoints too — so each legacy
// route names its own guard instead. That keeps the two schemes independent
// regardless of registration order.
func guarded(guards []fiber.Handler, handler fiber.Handler) []fiber.Handler {
	chain := make([]fiber.Handler, 0, len(guards)+1)
	chain = append(chain, guards...)
	return append(chain, handler)
}
