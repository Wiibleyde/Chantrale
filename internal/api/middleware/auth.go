package middleware

import (
	"crypto/subtle"

	"github.com/gofiber/fiber/v3"
)

// Auth returns a Fiber middleware that validates the X-API-Key request header.
// Requests missing or providing an incorrect key receive a 401 JSON response.
func Auth(apiKey string) fiber.Handler {
	return func(c fiber.Ctx) error {
		if subtle.ConstantTimeCompare([]byte(c.Get("X-API-Key")), []byte(apiKey)) != 1 {
			return fiber.NewError(fiber.StatusUnauthorized, "Clé d'API invalide ou manquante.")
		}
		return c.Next()
	}
}
