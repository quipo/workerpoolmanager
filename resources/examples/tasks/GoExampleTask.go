package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	jqutil "github.com/quipo/workerpoolmanager/utils"
)

func handleSigterm(done chan<- int, logger *log.Logger) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-c
		switch sig {
		case syscall.SIGTERM:
			logger.Println("Received SIGTERM")
		case syscall.SIGINT:
			logger.Println("Received SIGINT")
		default:
			logger.Println("Received Interrupt")
		}
		done <- 1
	}()
}

var logger *log.Logger

func main() {
	mypid := os.Getpid()
	logger := log.New(os.Stdout, fmt.Sprintf("[GoExampleTask][%d] ", mypid), log.Ldate|log.Ltime)

	taskname := "GoExample"
	keepalive := jqutil.NewKeepAliveManager(taskname, "tcp://localhost:5591")

	done := make(chan int)
	handleSigterm(done, logger)

	terminate := false

	for !terminate {
		// do something, and send keep-alives regularly to communicate the health of this worker
		select {
		case <-time.After(2 * time.Second):
			logger.Println("Sending keep-alive")
			keepalive.SetAlive()
		case <-done:
			logger.Println("Asked to terminate")
			terminate = true
		}
	}

	os.Exit(0)
}
