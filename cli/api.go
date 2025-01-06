package cli

import (
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"go.uber.org/zap"

	"github.com/codeArtisanry/go-boilerplate/config"
	"github.com/codeArtisanry/go-boilerplate/database"
	"github.com/codeArtisanry/go-boilerplate/routes"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/spf13/cobra"
)

// GetAPICommandDef runs app
func GetAPICommandDef(cfg config.AppConfig, logger *zap.Logger) cobra.Command {
	apiCommand := cobra.Command{
		Use:   "api",
		Short: "To start api",
		Long:  `To start api server`,
		RunE: func(cmd *cobra.Command, args []string) error {

			// Create fiber app
			app := fiber.New(fiber.Config{})

			// seup cors
			app.Use(cors.New(cors.Config{
				AllowOrigins: "*",
				AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
			}))

			dbConn := &database.DBConn{
				DatabaseConn: &database.SQLite3{}, // Replace with your database type
			}

			db, err := dbConn.DatabaseConn.Connect(*cfg.DB)
			if err != nil {
				logger.Panic("database connection failed", zap.Error(err))
			}
			defer func() {
				if err := db.Close(); err != nil {
					logger.Error("error closing database", zap.Error(err))
				}
			}()
			logger.Info("database connection established")

			// setup routes
			err = routes.Setup(app, db, logger, cfg)
			if err != nil {
				return err
			}

			interrupt := make(chan os.Signal, 1)
			signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				if err := app.Listen("0.0.0.0:8080"); err != nil {
					logger.Panic(err.Error())
				}
			}()

			<-interrupt
			logger.Info("gracefully shutting down...")
			if err := app.Shutdown(); err != nil {
				logger.Panic("error while shutdown server", zap.Error(err))
			}
			logger.Info("server stopped to receive new requests or connection.")

			return nil
		},
	}

	return apiCommand
}

var Http *config.AppConfig

func Location(c *fiber.Ctx, cfg *config.AppConfig) (string, error) {
	ip := IP(c)
	_, err := cfg.GeoIP.GetLocation(ip)
	if err != nil {
		return "127.0.0.1", err
	}
	return "127.0.0.1", err
}

var fetchIpFromString = regexp.MustCompile(`(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})`)
var possibleHeaderes = []string{
	"X-Original-Forwarded-For",
	"X-Forwarded-For",
	"X-Real-Ip",
	"X-Client-Ip",
	"Forwarded-For",
	"Forwarded",
	"Remote-Addr",
	"Client-Ip",
	"CF-Connecting-IP",
}

// determine user ip
func IP(c *fiber.Ctx) string {
	headerValue := []byte{}
	if Http.Server.Config().ProxyHeader == "*" {
		for _, headerName := range possibleHeaderes {
			headerValue = c.Request().Header.Peek(headerName)
			if len(headerValue) > 3 {
				return string(fetchIpFromString.Find(headerValue))
			}
		}
	}
	headerValue = []byte(c.IP())
	if len(headerValue) <= 3 {
		headerValue = []byte("0.0.0.0")
	}

	// find ip address in string
	return string(fetchIpFromString.Find(headerValue))
}
