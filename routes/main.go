package routes

import (
	"database/sql"
	"sync"

	"go.uber.org/zap"

	"github.com/codeArtisanry/go-boilerplate/config"
	controller "github.com/codeArtisanry/go-boilerplate/controllers/api/v1"

	"github.com/codeArtisanry/go-boilerplate/middlewares"
	"github.com/gofiber/fiber/v2"
)

var mu sync.Mutex

// Setup func
func Setup(app *fiber.App, db *sql.DB, logger *zap.Logger, config config.AppConfig) error {
	mu.Lock()

	app.Use(middlewares.LogHandler(logger))

	app.Static("/assets/", "./assets")

	app.Get("/docs", func(c *fiber.Ctx) error {
		return c.Render("./assets/index.html", fiber.Map{})
	})

	router := app.Group("/api")
	v1 := router.Group("/v1")

	middlewares := middlewares.NewMiddleware(config, logger)

	err := setupUserController(v1, db, logger, config, middlewares)
	if err != nil {
		return err
	}

	mu.Unlock()
	return nil
}

func setupUserController(router fiber.Router, db *sql.DB, logger *zap.Logger, config config.AppConfig, middlewares middlewares.Middleware) error {
	userController, err := controller.NewUserController(db, logger, config)
	if err != nil {
		return err
	}

	router.Post("/users", userController.Create)
	router.Delete("/users/:id", userController.Delete)
	router.Get("/users", userController.GetUsers)
	router.Get("/users/:id", userController.GetUserByIdOrEmail)
	router.Get("/users/email/:email", userController.GetUserByIdOrEmail)
	router.Put("/users/:id", userController.UpdateUser)

	return nil
}
