package cmd

import (
	"fmt"
	"os"
	"context"
	
	"github.com/spf13/cobra"
)

var bottlenetCmd = &cobra.Command{
	Use: fmt.Sprintf("%s [IP...] [-c|-s]", os.Args[0]),
	RunE: func(c *cobra.Command, args []string) error {
		return bottlenetEntrypoint(context.Background(), args)
	},
	DisableFlagsInUseLine: true,
	SilenceUsage:          true,
	SilenceErrors:         true,
}

var (
	clientMode    bool   = false
	serverMode    bool   = false
	bottlenetPort int = 7007
)

func init() {
	bottlenetCmd.Flags().BoolVarP(&clientMode, "client-network", "c", false, "bottlenet on the client node")
	bottlenetCmd.Flags().BoolVarP(&serverMode, "server-network", "s", false, "bottlenet on the server node")
	bottlenetCmd.Flags().IntVarP(&bottlenetPort, "port", "p", bottlenetPort, "listen on this port")
}

func Execute() error {
	return bottlenetCmd.Execute()
}
