// Package cli defines the vexgo command-line interface. The root command
// starts the server; cobra owns flag parsing and viper resolves the layered
// configuration (explicitly passed flags > config file > environment
// variables > defaults).
package cli

import (
	"fmt"
	"log/slog"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/vexgo-org/vexgo/backend/internal/config"
)

// Execute runs the vexgo command line with the given arguments and returns
// the resolved configuration. A nil *Config with a nil error means the
// command printed help or version information and there is nothing to run;
// a non-nil error means argument parsing or configuration resolution failed.
// version is the build version string, injected via ldflags.
func Execute(version string, args []string) (*config.Config, error) {
	root, state := newRootCmd(version)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		return nil, err
	}
	return state.cfg, nil
}

// runState carries the configuration resolved by RunE out of the command.
type runState struct {
	cfg *config.Config
}

// newRootCmd builds the vexgo root command. Flag defaults are the same
// constants config.Load falls back to, so help text cannot drift from
// resolution; a flag overrides the lower sources only when it is explicitly
// passed, which viper decides via the flag's Changed state.
func newRootCmd(version string) (*cobra.Command, *runState) {
	state := &runState{}
	var configFile string

	root := &cobra.Command{
		Use:   "vexgo",
		Short: "Self-hosted blog CMS server",
		// Tolerate stray positional arguments (for example "vexgo serve").
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion, _ := cmd.Flags().GetBool("version"); showVersion {
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "vexgo %s\n", version)
				return err
			}

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

	root.Flags().StringVarP(&configFile, "config", "c", "", "path to configuration file (YAML format)")
	root.Flags().StringP("addr", "a", config.DefaultAddr, "address to listen on")
	root.Flags().IntP("port", "p", config.DefaultPort, "port to listen on")
	root.Flags().StringP("data", "d", config.DefaultDataDir, "data directory for storing SQLite database and media files")
	root.Flags().BoolP("version", "V", false, "print version and exit")

	return root, state
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
func loadDotEnv() {
	if err := godotenv.Load(".env"); err != nil {
		slog.Info("no .env file found, will use environment variables from the system")
	}
}
