package cmd

import (
	"context"
	"fmt"
)

func peer(ctx context.Context, coordinator string) error {
	peerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	connbrk := make(chan struct{})
	if err := doJoin(peerCtx, coordinator, connbrk); err != nil {
		fmt.Println(err)
		return err
	}

	go func() {
		<-connbrk
		cancel()
	}()

	return serveBottlenet(peerCtx, nil)
}
