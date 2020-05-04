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
	"errors"
	"fmt"
	"io"
)

func peer(ctx context.Context, coordinator string) error {
	peerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	connbrk := make(chan error)
	if err := doJoin(peerCtx, coordinator, connbrk); err != nil {
		return err
	}

	go func() {
		if err := <-connbrk; err != nil {
			if errors.Is(err, context.Canceled) {
				fmt.Println("connection closed")
			} else if !errors.Is(err, io.ErrUnexpectedEOF) {
				fmt.Println(err.Error())
			}
		}
		cancel()
	}()

	return serveBottlenet(peerCtx, nil)
}
