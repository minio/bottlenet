package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
)

func bottlenetEntrypoint(ctx context.Context, args []string) error {
	if err := validateArgs(args); err != nil {
		return err
	}
	mainCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	//handle signals
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		cancel()
	}()

	mesh := !serverMode && !clientMode

	if mesh {
		c.clusterType = clusterTypeMesh
		meshCtx, cancel := context.WithCancel(mainCtx)
		defer cancel()

		if len(args) > 0 {
			return peer(meshCtx, args[0])
		}
		return coordinate(meshCtx)
	}
	return nil
}

func validateArgs(args []string) error {
	err := validatePort(bottlenetPort)
	if err != nil {
		return err
	}
	if clientMode {
		if len(args) != 1 {
			return fmt.Errorf("client-network nodes must provide a coordinator address")
		}
		return validateHostPort(args[0])
	}
	if serverMode {
		if len(args) > 1 {
			return fmt.Errorf("too many arguments passed for server-network node")
		}
		if len(args) == 1 {
			return validateHostPort(args[0])
		}
		return nil
	}
	if len(args) > 1 {
		return fmt.Errorf("extra argument for mesh network. expected 1 argument only")
	}
	return nil
}

func validateHostPort(addr string) error {
	_, _, err := net.SplitHostPort(addr)
	return err
}

func validatePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port '%d' out of range (0, 65535]", port)
	}
	return nil
}
