package cli

import (
	"os/exec"
	"runtime"

	"github.com/codeArtisanry/go-boilerplate/config"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// build function automates the build process
func build(cfg config.AppConfig, logger *zap.Logger) *cobra.Command { // Return a pointer to cobra.Command
	buildCmd := &cobra.Command{ // Initialize as a pointer
		Use:   "build",
		Short: "Build the application",
		Long:  `This command automates the building of the application.`,
		RunE: func(cmd *cobra.Command, args []string) error { // RunE to return error
			// Determine the operating system
			goos := runtime.GOOS
			goarch := runtime.GOARCH

			logger.Info("Building for OS: %s, ARCH: %s\n", zap.String("OS", goos), zap.String("ARCH", goarch))

			// Run the go build command
			buildCmd := exec.Command("go", "build", "-o", cfg.AppName)
			output, err := buildCmd.CombinedOutput()

			if err != nil {
				logger.Error("Error building the application", zap.Error(err))
				logger.Info(string(output))
				return err // Return the error
			}

			logger.Info("Application built successfully")
			return nil // Return nil on success
		},
	}
	return buildCmd
}
