/*
 * Bottlenet (C) 2020 MinIO, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

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
