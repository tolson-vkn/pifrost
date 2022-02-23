package cmd

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/tolson-vkn/pifrost/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Long:  `Print version`,
	Run: func(cmd *cobra.Command, args []string) {
		printVersion()
	},
}

func printVersion() {
	logrus.WithFields(logrus.Fields{
		"version": version.Version,
		"commit":  version.GitCommit,
	}).Info("pifrost version")
}
