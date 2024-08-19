package main

import (
	"flag"

	"cloud.google.com/go/profiler"
	"github.com/codeArtisanry/go-boilerplate/app"
	"github.com/codeArtisanry/go-boilerplate/logger"
)

func main() {
	configFile := flag.String("config", "config.yml", "User Config file from user")
	flag.Parse()
	app.Load(*configFile)
	if app.Http.Profiler.Enabled {
		_, _ = profiler.Start(profiler.Config{
			ApplicationName: app.Http.Server.Name,
			ServerAddress:   app.Http.Profiler.Server,
		})
	}
	logger, err := logger.NewRootLogger(cfg.Debug, cfg.IsDevelopment)
	if err != nil {
		panic(err)
	}
	app.Http.Server.Version = app.Version

}
