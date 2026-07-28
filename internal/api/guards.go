package api

import "github.com/gofiber/fiber/v2"

// guarded prepends route-scoped middleware to a handler.
//
// The legacy monitoring and bearer-authenticated control-plane routes use
// different authentication schemes. Mounting the legacy X-API-Key middleware
// globally would attach it to the bearer-authenticated routes too, so each
// legacy route names its own guard instead. That keeps the two schemes
// independent regardless of registration order.
func guarded(guards []fiber.Handler, handler fiber.Handler) []fiber.Handler {
	chain := make([]fiber.Handler, 0, len(guards)+1)
	chain = append(chain, guards...)
	return append(chain, handler)
}
