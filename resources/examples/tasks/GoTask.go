package main

import (
	"log"
	"log/syslog"
	"os"
	"os/signal"
	"syscall"
	"time"

	jqutil "github.com/quipo/workerpoolmanager/utils"
)

func handleSigterm(done chan<- int, logger *syslog.Writer) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM)
	go func() {
		sig := <-c
		logger.Info("Received Interrupt")
		switch sig {
		case syscall.SIGTERM:
			logger.Info("Received SIGTERM")
		}
		done <- 1
	}()
}

var logger *log.Logger

func main() {
	logger, err := syslog.New(syslog.LOG_INFO, "[GoTask] ")
	if err != nil {
		log.Fatal("Cannot init Syslog logger:", err)
	}
	defer logger.Close()

	taskname := "Go"
	keepalive := jqutil.NewKeepAliveManager(taskname, "tcp://localhost:5591")

	done := make(chan int)
	handleSigterm(done, logger)

	terminate := false

	for !terminate {
		// do something, and send keep-alives regularly to communicate the health of this worker
		select {
		case <-time.After(2 * time.Second):
			logger.Info("Sending keep-alive")
			logger.Err("Sending keep-alive")
			keepalive.SetAlive()
		case <-done:
			logger.Err("Asked to terminate")
			terminate = true
		}
	}
}
