package taskmanager

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// SignalHandler wrap the command channel
type SignalHandler struct {
	CommandChannel chan Command
}

// Run the Signal handler, to intercept interrupts and shut down processes cleanly
func (handler *SignalHandler) Run() {
	logger := log.New(os.Stdout, "[SignalHandler] ", log.Ldate|log.Ltime)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Kill, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		interr := <-c
		logger.Println("Received Interrupt")
		switch interr {
		//case syscall.SIGINT:
		//	logger.Println("Received SIGINT")
		case syscall.SIGTERM:
			logger.Println("Received SIGTERM")
		case syscall.SIGHUP:
			logger.Println("Received SIGHUP")
		case syscall.SIGQUIT:
			logger.Println("Received SIGQUIT")
		case os.Kill:
			logger.Println("Received kill")
		}

		cmd := Command{Type: "stop", ReplyChannel: make(chan CommandReply, 1)}
		logger.Println("Sending stop to all task managers:")
		logger.Println(cmd.Send(handler.CommandChannel))

		os.Exit(1)
	}()

	for {
		logger.Println("Waiting for interrupt")
		time.Sleep(1 * time.Minute)
	}
}
