package cli

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/codeArtisanry/go-boilerplate/config"
	"github.com/codeArtisanry/go-boilerplate/database"
	"github.com/icrowley/fake"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func seed(cfg config.AppConfig, logger *zap.Logger) *cobra.Command {
	seedCmd := &cobra.Command{
		Use:   "seed [flags]",
		Short: "Seed the database",
		Long:  `This command seeds the database with sample data.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Info("Seeding the database")

			defaultCount := 3
			// Check if the number of arguments is valid
			if len(args) != 1 {
				args = append(args, fmt.Sprintf("%d", defaultCount))
				fmt.Errorf("invalid number of arguments: %d, Setting default to %d", len(args), defaultCount)
			}

			seedingCountInt, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("failed to convert seeding count to integer: %w", err)
			}
			logger.Info("Seeding count", zap.Int("count", seedingCountInt))

			// Connect to the database
			db, err := sql.Open(database.SQLITE3, cfg.DB.SQLiteFilePath)
			if err != nil {
				return err
			}
			defer db.Close()

			for i := 0; i < seedingCountInt; i++ {
				// Prepare the seed data
				user := struct {
					Name     string
					Email    string
					Password string
				}{
					fake.FullName(), fake.EmailAddress(), fake.SimplePassword(),
				}
				// Insert the seed data into the `users` table
				query := `INSERT INTO users (name, email, password, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`
				_, err := db.Exec(query, user.Name, user.Email, user.Password, time.Now(), time.Now())
				if err != nil {
					logger.Error("Failed to insert user", zap.String("email", user.Email), zap.Error(err))
					return fmt.Errorf("failed to seed user: %w", err)
				}
				logger.Info("Inserted user", zap.String("email", user.Email))
			}

			logger.Info("Database seeding completed successfully")
			return nil
		},
	}

	return seedCmd
}
