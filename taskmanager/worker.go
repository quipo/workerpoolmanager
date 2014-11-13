package taskmanager

import (
	"fmt"
	"log"
	"os"
	"syscall"
	"time"
)

// Worker contains meta data about a worker process for a certain task
type Worker struct {
	Pid         int
	Taskname    string
	StartedAt   time.Time
	LastAliveAt time.Time
	Logger      *log.Logger
}

// Implement String() interface
func (worker *Worker) String() string {
	return fmt.Sprintf(
		"Task: %s,\tPid: %d,\tStarted: %s,\tLast alive at: %s (%d seconds ago)",
		worker.Taskname,
		worker.Pid,
		worker.StartedAt.Format(time.RFC3339),
		worker.LastAliveAt.Format(time.RFC3339),
		int(time.Since(worker.LastAliveAt).Seconds()))
}

// signal example: https://github.com/rcrowley/goagain/blob/master/goagain.go

// Stop Attempts to terminat a worker gracefully by sending SIGTERM.
// If after a grace period it hasn't terminated, it is killed with SIGKILL
func (worker *Worker) Stop(gracePeriod time.Duration, confirmChannel chan<- Command) {
	confirmCommand := Command{
		Type:         "stoppedworker",
		Name:         "pid",
		Value:        worker.Pid,
		TaskName:     worker.Taskname,
		ReplyChannel: make(chan CommandReply, 1),
	}

	//err := syscall.Kill(worker.Pid, syscall.SIGTERM)
	proc, err := os.FindProcess(worker.Pid)
	if err != nil {
		worker.Logger.Printf("worker.Stop(): Cannot find worker process %d: %s\n", worker.Pid, err)
		confirmChannel <- confirmCommand
		return
	}

	// attempt stopping the worker with a SIGTERM first
	err = proc.Signal(syscall.SIGTERM)
	if err != nil {
		worker.Logger.Printf("worker.Stop(): Error sending SIGTERM to worker process %d: %s\n", worker.Pid, err)
	}

	// wait until the process returns, or timeout + kill after a grace period
	done := make(chan error)
	go func() {
		// call wait() to avoid leaving zombies around
		_, err := proc.Wait()
		done <- err
	}()
	select {
	case <-time.After(gracePeriod):
		if err := proc.Kill(); err != nil {
			worker.Logger.Println("worker.Stop(): Failed to kill process", worker.Pid, err)
		}
		<-done // allow goroutine to exit
		worker.Logger.Printf("Worker process still around after %s, killed pid %d\n", gracePeriod, worker.Pid)
	case err := <-done:
		if err != nil {
			worker.Logger.Printf("Worker process %d terminated with error: %s\n", worker.Pid, err)
		} else {
			worker.Logger.Printf("Worker process %d terminated gracefully\n", worker.Pid)
		}
	}

	confirmChannel <- confirmCommand
}

// IsProcessAlive checks if the process is still around
//@see http://stackoverflow.com/questions/15204162/check-if-a-process-exists-in-go-way
func (worker *Worker) IsProcessAlive() bool {
	proc, err := os.FindProcess(worker.Pid)
	if err != nil {
		// on unix, FindProcess always returns true
		return false
	}
	return nil == proc.Signal(syscall.Signal(0))
	//if syscall.ESRCH == proc.Signal(syscall.Signal(0)) {
	//	processIsDead
	//}
}

// CleanupProcessIfDead removes internal references to dead workers
func (worker *Worker) CleanupProcessIfDead(confirmChannel chan<- Command) {
	if !worker.IsProcessAlive() {
		log.Printf("Worker process %d not found! Zombie/Dead process. Removing reference.\n", worker.Pid)
		confirmChannel <- Command{
			Type:         "stoppedworker",
			Name:         "pid",
			Value:        worker.Pid,
			TaskName:     worker.Taskname,
			ReplyChannel: make(chan CommandReply, 1),
		}
	}
}
