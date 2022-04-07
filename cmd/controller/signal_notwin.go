//go:build !windows
// +build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

func initKeyGenSignalListener(trigger func()) {
	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, syscall.SIGUSR1)
	go func() {
		for {
			<-sigChannel
			trigger()
		}
	}()
}
