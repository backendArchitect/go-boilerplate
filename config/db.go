package config

// DBConfig type of db config object
type DBConfig struct {
	Host           string `envconfig:"DB_HOST" validate:"required" default:"localhost"`
	Port           int    `envconfig:"DB_PORT" validate:"required" default:"5432"`
	Username       string `envconfig:"DB_USERNAME" validate:"required" default:"user"`
	Password       string `envconfig:"DB_PASSWORD" validate:"required" default:"password"`
	Db             string `envconfig:"DB_NAME" validate:"required" default:"mydb"`
	QueryString    string `envconfig:"DB_QUERYSTRING" default:""`
	MigrationDir   string `required:"true" envconfig:"MIGRATION_DIR" validate:"required" default:"database/migrations"`
	Dialect        string `required:"true" envconfig:"DB_DIALECT" validate:"required" default:"sqlite3"`
	SQLiteFilePath string `envconfig:"SQLITE_FILEPATH" default:"database/go-boilerplate.db	"`
}
