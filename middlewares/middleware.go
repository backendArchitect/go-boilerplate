package middlewares

import (
	"strings"

	"github.com/codeArtisanry/go-boilerplate/config"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"go.uber.org/zap"
)

type Middleware struct {
	config config.AppConfig
	logger *zap.Logger
}

func NewMiddleware(cfg config.AppConfig, logger *zap.Logger) Middleware {
	return Middleware{
		config: cfg,
		logger: logger,
	}
}

func JWTMiddleware(cfg config.AppConfig, logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(401).JSON(fiber.Map{"error": "missing or malformed JWT"})
		}

		tokenStr := strings.Split(authHeader, "Bearer ")[1]

		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			return []byte(cfg.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			return c.Status(401).JSON(fiber.Map{"error": "invalid or expired JWT"})
		}

		c.Locals("user_id", token.Claims.(jwt.MapClaims)["user_id"])
		return c.Next()
	}
}
