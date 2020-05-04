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
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var bottlenetCmd = &cobra.Command{
	Use: fmt.Sprintf("%s [IP...] [-a]", os.Args[0]),
	RunE: func(c *cobra.Command, args []string) error {
		return bottlenetEntrypoint(context.Background(), args)
	},
	DisableFlagsInUseLine: true,
	SilenceUsage:          true,
	SilenceErrors:         true,
	Long: `
Bottlenet finds bottlenecks in your cluster

Steps to find bottlenecks using bottlenet:
1. Run 1 instance of bottlenet on control node, where output will be collected:

    $>_ bottlenet 

2. Run 1 instance of bottlenet on each of the peer nodes:

    $>_ bottlenet CONTROL-SERVER:IP 

Once all the peer nodes have been added, press 'y' on the prompt (on control node) to start the tests

In order to bind bottlenet to specific interface and port

    $>_ bottlenet --adddress IP:PORT

Note: --address can be applied to both control and peer nodes

`,
}

var (
	address = ":7007"
)

func init() {
	bottlenetCmd.Flags().StringVarP(&address, "address", "a", address, "listen address")
}

// Execute runs the binary
func Execute() error {
	return bottlenetCmd.Execute()
}
