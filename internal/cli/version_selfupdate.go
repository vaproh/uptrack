package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/vaproh/uptrack/internal/color"
	"github.com/vaproh/uptrack/internal/gh"
	"github.com/vaproh/uptrack/internal/prompt"
	"github.com/vaproh/uptrack/internal/selfupdate"
	"github.com/vaproh/uptrack/internal/version"
)

// Version is stamped at release build time via ldflags.
var Version = "dev"

func newVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the uptrack version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(Version)
		},
	}
}

func newSelfUpdate() *cobra.Command {
	return &cobra.Command{
		Use:   "self-update",
		Short: "Update uptrack itself to the latest release",
		RunE: runE(func(c *ctx, args []string) error {
			exe, err := os.Executable()
			if err != nil {
				return fmt.Errorf("locate own binary: %w", err)
			}
			exe, err = filepath.EvalSymlinks(exe)
			if err != nil {
				return fmt.Errorf("resolve own binary: %w", err)
			}
			client := gh.New("", gh.TokenFromEnv())
			rel, err := client.Latest(context.Background(), selfupdate.Owner, selfupdate.Repo)
			if err != nil {
				return err
			}
			if rel == nil {
				return fmt.Errorf("no uptrack releases found")
			}
			if Version != "dev" && version.Compare(Version, rel.TagName) >= 0 {
				fmt.Printf("uptrack %s is the latest\n", color.Cyan(rel.TagName))
				return nil
			}
			if !c.assume && !prompt.Confirm(fmt.Sprintf("update uptrack %s to %s?", Version, rel.TagName)) {
				fmt.Println(color.Dim("aborted"))
				return nil
			}
			tag, updated, err := selfupdate.Update(context.Background(), client, exe, Version)
			if err != nil {
				return err
			}
			if !updated {
				fmt.Printf("uptrack %s is the latest\n", color.Cyan(tag))
				return nil
			}
			fmt.Printf("updated uptrack to %s\n", color.Green(tag))
			return nil
		}),
	}
}
