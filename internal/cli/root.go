// Package cli wires cobra commands. Presentation (tables, colors,
// prompts) lives here; all logic lives in the other packages.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/vaproh/uptrack/internal/color"
	"github.com/vaproh/uptrack/internal/config"
	"github.com/vaproh/uptrack/internal/platform"
)

// hostOf returns the current host for asset selection.
func hostOf() platform.Host {
	return platform.Detect()
}

// ctx carries the shared loaded state for one invocation.
type ctx struct {
	paths  config.Paths
	cfg    *config.Config
	state  *config.State
	assume bool
}

func loadCtx(assume bool) (*ctx, error) {
	paths, err := config.DefaultPaths()
	if err != nil {
		return nil, err
	}
	cfg, err := config.LoadConfig(paths.ConfigFile)
	if err != nil {
		return nil, err
	}
	st, err := config.LoadState(paths.StateFile)
	if err != nil {
		return nil, err
	}
	return &ctx{paths: paths, cfg: cfg, state: st, assume: assume}, nil
}

// NewRoot builds the command tree.
func NewRoot() *cobra.Command {
	var noColor bool
	var assume bool
	root := &cobra.Command{
		Use:   "uptrack",
		Short: "Track and install standalone Linux apps from GitHub releases",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if noColor {
				color.SetEnabled(false)
			}
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
	root.PersistentFlags().BoolVarP(&assume, "yes", "y", false, "skip confirmation prompts")

	root.AddCommand(newList(), newRemove(), newUninstall(), newUninstallSelf(), newAdd(), newCheck(), newOutdated(), newInfo(), newLog(), newInstall(), newSync(), newPin(), newUnpin(), newDoctor(), newVersion(), newSelfUpdate())
	return root
}

func runE(fn func(c *ctx, args []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		assume, err := cmd.Flags().GetBool("yes")
		if err != nil {
			return err
		}
		c, err := loadCtx(assume)
		if err != nil {
			return err
		}
		err = fn(c, args)
		if err != nil {
			fmt.Fprintln(cmd.ErrOrStderr(), color.Red("error: ")+err.Error())
			return err
		}
		return nil
	}
}
