// Package cli defines the vexgo command-line interface. The root command
// starts the server; cobra owns flag parsing and viper resolves the layered
// configuration (explicitly passed flags > config file > environment
// variables > defaults).
package cli

import (
	"github.com/vexgo-org/vexgo/backend/internal/config"
)

// runState carries the configuration resolved by RunE out of the command.
type runState struct {
	cfg *config.Config
}

// Execute runs the vexgo command line with the given arguments and returns
// the resolved configuration. A nil *Config with a nil error means the
// command printed help or version information and there is nothing to run;
// a non-nil error means argument parsing or configuration resolution failed.
// version is the build version string, injected via ldflags.
func Execute(version string, args []string) (*config.Config, error) {
	root := newRootCmd(version)
	root.SetArgs(args)

	server, state := newServerCmd()
	root.AddCommand(server)

	if err := root.Execute(); err != nil {
		return nil, err
	}
	return state.cfg, nil
}
