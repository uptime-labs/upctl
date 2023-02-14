package main

import (
	"fmt"
	"github.com/uptime-labs/upctl/cmd"
	"os"
	"os/signal"
	"syscall"
)

func main() {

	go catchSignal()

	cmd.Execute()
}

func catchSignal() {

	terminateSignals := make(chan os.Signal, 1)

	signal.Notify(terminateSignals, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM) //NOTE:: syscall.SIGKILL we cannot catch kill -9 as its force kill signal.

	for { //We are looping here because config reload can happen multiple times.
		select {
		case s := <-terminateSignals:
			fmt.Println("Got one of stop signals, shutting down, SIGNAL NAME :", s)
			cmd.StopProgress()
			os.Exit(0)
			break //break is not necessary to add here as if server is closed our main function will end.
		}
	}
}
