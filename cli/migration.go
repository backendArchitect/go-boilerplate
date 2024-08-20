package cli

import (
	"database/sql"
	"fmt"

	"github.com/codeArtisanry/go-boilerplate/config"
	"github.com/codeArtisanry/go-boilerplate/database"
	_ "github.com/lib/pq" // for postgres dialect
	migrate "github.com/rubenv/sql-migrate"
	"github.com/spf13/cobra"
)

// GetMigrationCommandDef initialize migration command
func GetMigrationCommandDef(cfg config.AppConfig) cobra.Command {
	migrateCmd := cobra.Command{
		Use:   "migrate [sub command]",
		Short: "To run db migrate",
		Long: `This command is used to run database migration.
	It has up and down sub commands`,
		Args: cobra.MinimumNArgs(1),
	}

	migrateUp := cobra.Command{
		Use:   "up",
		Short: "It will apply migration(s)",
		Long:  `It will run all remaining migration(s)`,
		Args:  cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch cfg.DB.Dialect {
			case database.POSTGRES:
				return runPostgresMigration(cfg, "UP")
			}
			return nil
		},
	}

	migrateDown := cobra.Command{
		Use:   "down",
		Short: "It will revert migration(s)",
		Long:  `It will run all remaining migration(s)`,
		Args:  cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Run test db migration
			switch cfg.DB.Dialect {
			case database.POSTGRES:
				return runPostgresMigration(cfg, "DOWN")
			}
			return nil
		},
	}
	migrateCmd.AddCommand(&migrateUp, &migrateDown)
	return migrateCmd
}

func runPostgresMigration(cfg config.AppConfig, migrationType string) error {
	migrations := migrate.FileMigrationSource{
		Dir: cfg.DB.MigrationDir,
	}

	db, err := sql.Open(database.POSTGRES, fmt.Sprintf("postgres://%s:%s@%s:%d/%s?%s", cfg.DB.Username, cfg.DB.Password, cfg.DB.Host, cfg.DB.Port, cfg.DB.Db, cfg.DB.QueryString))
	if err != nil {
		return err
	}

	if migrationType == "UP" {
		_, err = migrate.Exec(db, database.POSTGRES, migrations, migrate.Up)
		if err != nil {
			return err
		}
	} else {
		_, err = migrate.Exec(db, database.POSTGRES, migrations, migrate.Down)
		if err != nil {
			return err
		}
	}

	return nil
}
