package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	cfgFile  string
	logLevel string

	rootCmd = &cobra.Command{
		Use:   "pifrost",
		Short: "An external DNS provider for kube and pihole",
		Long:  "An external DNS provider for kubernetes and pihole",
	}
)

// Execute executes the root command.
func Execute() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if err := setUpLogs(os.Stdout, logLevel); err != nil {
			return err
		}
		return nil
	}

	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", logrus.InfoLevel.String(), "log level (debug, info, warn, error, fatal, panic")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(serverCmd)
}

func setUpLogs(out io.Writer, level string) error {
	logrus.SetOutput(out)
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return err
	}
	logrus.SetLevel(lvl)
	return nil
}
