package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vexgo-org/vexgo/backend/internal/config"
)

// newServerCmd builds the `vexgo server` command. Flag defaults are the same
// constants config.Load falls back to, so help text cannot drift from
// resolution; a flag overrides the lower sources only when it is explicitly
// passed, which viper decides via the flag's Changed state.
func newServerCmd() (*cobra.Command, *runState) {
	var configFile string
	state := &runState{}

	server := &cobra.Command{
		Use:           "server",
		Short:         "Start VexGo server",
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

			cfg, err := resolveConfig(cmd, configFile)
			if err != nil {
				return err
			}
			state.cfg = cfg
			return nil
		},
	}

	server.Flags().StringVarP(&configFile, "config", "c", "", "path to configuration file (YAML format)")
	server.Flags().StringP("addr", "a", config.DefaultAddr, "address to listen on")
	server.Flags().IntP("port", "p", config.DefaultPort, "port to listen on")
	server.Flags().StringP("data", "d", config.DefaultDataDir, "data directory for storing SQLite database and media files")

	return server, state
}

// resolveConfig binds the parsed flags to viper and resolves the layered
// configuration. The data flag intentionally binds to the data_dir key so
// the config file and environment keep their historical names.
func resolveConfig(cmd *cobra.Command, configFile string) (*config.Config, error) {
	v := viper.New()
	for _, binding := range []struct{ key, flag string }{
		{"addr", "addr"},
		{"port", "port"},
		{"data_dir", "data"},
	} {
		if err := v.BindPFlag(binding.key, cmd.Flags().Lookup(binding.flag)); err != nil {
			return nil, fmt.Errorf("bind flag --%s: %w", binding.flag, err)
		}
	}
	return config.Load(v, configFile)
}

// loadDotEnv loads environment variables from a .env file (best-effort).
// godotenv never overrides variables already present in the environment.
// A missing file is the normal case; any other failure (permissions,
// malformed content) is logged as a warning with the underlying error
// instead of being silently swallowed.
func loadDotEnv() {
	err := godotenv.Load(".env")
	if err == nil {
		return
	}
	if errors.Is(err, fs.ErrNotExist) {
		slog.Info("no .env file found, will use environment variables from the system")
		return
	}
	slog.Warn("failed to load .env file, will use environment variables from the system", "err", err)
}
