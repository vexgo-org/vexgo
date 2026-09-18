package cli

import (
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/vexgo-org/vexgo/backend/internal/config"
)

// newDevCmd builds the `vexgo dev` command. It is `vexgo server` plus a
// --theme-dir flag that overrides the active theme with a local directory so a
// developer can iterate on a theme without rebuilding the embedded default
// theme. The resolved config is handed back to main.go, which builds and runs
// the application exactly as for `server`.
func newDevCmd() (*cobra.Command, *runState) {
	var configFile string
	var themeDir string
	state := &runState{}

	dev := &cobra.Command{
		Use:           "dev",
		Short:         "Start VexGo development server",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// .env is only needed when actually building the server
			// configuration; help and version exit without reading it.
			loadDotEnv()

			cfg, err := resolveDevConfig(cmd, configFile, themeDir)
			if err != nil {
				return err
			}
			state.cfg = cfg
			return nil
		},
	}

	dev.Flags().StringVarP(&configFile, "config", "c", "", "path to configuration file (YAML format)")
	dev.Flags().StringP("addr", "a", config.DefaultAddr, "address to listen on")
	dev.Flags().IntP("port", "p", config.DefaultPort, "port to listen on")
	dev.Flags().StringP("data", "d", config.DefaultDataDir, "data directory for storing SQLite database and media files")
	dev.Flags().StringVar(&themeDir, "theme-dir", "", "local theme directory used for public page rendering")

	return dev, state
}

// resolveDevConfig binds the parsed flags to viper and resolves the layered
// configuration. --theme-dir is a dev-only flag with no config-file or
// environment equivalent, so it is applied on top of the resolved config and
// always wins. The data flag binds to the data_dir key so the config file and
// environment keep their historical names.
func resolveDevConfig(cmd *cobra.Command, configFile, themeDir string) (*config.Config, error) {
	// Resolve server configuration.
	cfg, err := resolveConfig(cmd, configFile)
	if err != nil {
		return nil, err
	}

	if themeDir != "" {
		cfg.ThemeDir = themeDir
		slog.Info("dev theme directory configured", "dir", themeDir)
	}
	return cfg, nil
}
