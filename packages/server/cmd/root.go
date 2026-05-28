package cmd

import (
	"orinforum/server"

	"github.com/spf13/cobra"
)

var env string

var RootCmd = &cobra.Command{
	Use:   "orinforum",
	Short: "OrinForum API server",
	Run: func(cmd *cobra.Command, args []string) {
		server.Start(server.Env(env))
	},
}

func init() {
	RootCmd.PersistentFlags().StringVarP(&env, "env", "e", "dev", "environment: dev | prod")
}
