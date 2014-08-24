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
	Logger         *log.Logger
}

// Run the Signal handler, to intercept interrupts and shut down processes cleanly
func (handler *SignalHandler) Run() {
	if handler.Logger == nil {
		handler.Logger = log.New(os.Stdout, "[SignalHandler] ", log.Ldate|log.Ltime)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Kill, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGQUIT, os.Interrupt)
	go func() {
		interr := <-c
		handler.Logger.Println("Received Interrupt")
		switch interr {
		case syscall.SIGINT:
			handler.Logger.Println("Received SIGINT")
		case syscall.SIGTERM:
			handler.Logger.Println("Received SIGTERM")
		case syscall.SIGHUP:
			handler.Logger.Println("Received SIGHUP")
		case syscall.SIGQUIT:
			handler.Logger.Println("Received SIGQUIT")
		case os.Kill:
			handler.Logger.Println("Received kill")
		case os.Interrupt:
			handler.Logger.Println("Received interrupt")
		}

		cmd := Command{Type: "stop", ReplyChannel: make(chan CommandReply, 1)}
		handler.Logger.Println("Sending stop to all task managers:")
		handler.Logger.Println(cmd.Send(handler.CommandChannel))

		os.Exit(1)
	}()

	for {
		handler.Logger.Println("Waiting for interrupt")
		time.Sleep(1 * time.Minute)
	}
}
