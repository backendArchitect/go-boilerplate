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

func Setup(app *fiber.App, db *sql.DB, logger *zap.Logger, config config.AppConfig) error {
	userController, err := controller.NewUserController(db, logger, config)
	if err != nil {
		return err
	}

	mu.Lock()

	app.Use(middlewares.LogHandler(logger))

	app.Static("/assets/", "./assets")

	app.Get("/docs", func(c *fiber.Ctx) error {
		return c.Render("./assets/index.html", fiber.Map{})
	})

	router := app.Group("/api")
	v1 := router.Group("/v1")

	middlewares := middlewares.JWTMiddleware(config, logger)

	err = setupUserController(v1, middlewares, userController)
	if err != nil {
		return err
	}
	// Public routes
	router.Post("/users/register", userController.Register)
	router.Post("/users/login", userController.Login)

	mu.Unlock()
	return nil
}

func setupUserController(router fiber.Router, middlewares fiber.Handler, userController *controller.UserController) error {
	// Protected routes
	protected := router.Group("", middlewares)
	protected.Delete("/users/:id", userController.Delete)
	protected.Get("/users", userController.GetUsers)
	protected.Get("/users/:id", userController.GetUserByIdOrEmail)
	protected.Get("/users/email/:email", userController.GetUserByIdOrEmail)
	protected.Put("/users/:id", userController.UpdateUser)

	return nil
}
