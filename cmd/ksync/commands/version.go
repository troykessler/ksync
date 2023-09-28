package commands

import (
	"fmt"
	"github.com/spf13/cobra"
	"runtime/debug"
	"strings"
)

var version string

func init() {
	rootCmd.AddCommand(versionCmd)
}

func printVersion() {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		panic("failed to get ksync version")
	}

	// only use version from build info if no version was given with
	// -ldflags "-X 'github.com/KYVENetwork/ksync/cmd/ksync/commands.version=v1.2.3'"
	if version == "" {
		version = strings.TrimSpace(buildInfo.Main.Version)
	}

	fmt.Println(version)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of KSYNC",
	Run: func(cmd *cobra.Command, args []string) {
		printVersion()
	},
}
