package taskmanager

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

const minStallDetectionFrequency int64 = 5000 // check at least every 5 seconds

// WorkerInfo is a wrapper for the process pid and the channels to communicate with the task manager
type WorkerInfo struct {
	Pid              int
	KeepAliveChannel chan KeepAlive
	CommandChannel   chan Command
}

// Worker manages a single worker process
type Worker struct {
	Taskname            string         `json:"taskname"`
	Command             string         `json:"command,omitempty"`
	Args                []string       `json:"args,omitempty"`
	CaptureOutput       bool           `json:"capture_output,omitempty"`
	StallTimeout        int64          `json:"stall_timeout,omitempty"`
	GracePeriod         int64          `json:"grace_period,omitempty"` // grace period (ms) before killing a worker after being asked to stop
	Pid                 int            `json:"pid,omitempty"`
	StartedAt           time.Time      `json:"started_at,omitempty"`
	LastAliveAt         time.Time      `json:"last_alive_at,omitempty"`
	Logger              *log.Logger    `json:"-"` // don't export
	TaskFeedbackChannel chan<- Command `json:"-"` // don't export
	CommandsChannel     chan Command   `json:"-"` // don't export
	keepAliveChannel    chan KeepAlive
	exitChannel         chan Command
}

func (w *Worker) init() {
	w.Pid = -1 //initialise
	if nil == w.Logger {
		prefix := fmt.Sprintf("[TaskManager][%s]", w.Taskname)
		w.Logger = log.New(os.Stdout, prefix, log.Ldate|log.Ltime)
	}
	w.StartedAt = time.Now()
	w.LastAliveAt = time.Now()
	w.exitChannel = make(chan Command)
	w.CommandsChannel = make(chan Command)
	w.keepAliveChannel = make(chan KeepAlive)
}

// Start spawns a new worker process
func (w *Worker) Start() (*WorkerInfo, error) {
	w.init()

	//task.logger.Println("StartWorker()", task.Cmd, task.Args)
	cmd := exec.Command(w.Command, w.Args...)
	if w.CaptureOutput {
		//w.Logger.Println("Capturing output")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stdout
	}
	err := cmd.Start()
	if err != nil {
		//w.TaskFeedbackChannel <- w.stoppedCommand(err, nil, false)
		close(w.exitChannel)
		close(w.CommandsChannel)
		close(w.keepAliveChannel)
		return nil, err
	}

	w.Logger.Printf("Started new worker (pid %d) %s with args %+v\n", cmd.Process.Pid, w.Command, w.Args)
	w.Pid = cmd.Process.Pid
	w.Logger.SetPrefix(fmt.Sprintf("%s[%d] ", strings.TrimSpace(w.Logger.Prefix()), w.Pid))

	// wait on the process on a separate goroutine
	go w.waitOnProcess(cmd)

	info := &WorkerInfo{
		Pid:              w.Pid,
		KeepAliveChannel: w.keepAliveChannel,
		CommandChannel:   w.CommandsChannel,
	}

	go w.run()

	return info, nil
}

// waitOnProcess blocks the current goroutine until the worker process returns
func (w *Worker) waitOnProcess(cmd *exec.Cmd) {
	// the exit channel for this worker might be closed - ignore error
	defer func() {
		r := recover()
		if nil != r {
			w.Logger.Println("Recovered in waitOnProcess() after panic:", r)
		}
	}()

	//cmd.Process.Release()
	// signal when the process exits
	state, err := cmd.Process.Wait()
	err = w.getSyscallError(err)
	if nil != err {
		w.Logger.Printf("worker %s (%d) stopped (%s) Error: %v", w.Taskname, w.Pid, state.String(), err)
	} else {
		w.Logger.Printf("worker %s (%d) stopped (%s)", w.Taskname, w.Pid, state.String())
	}
	w.exitChannel <- w.stoppedCommand(err, state, false)
}

// getStallDetectionticker Periodically detect stalled workers
// (at least at twice the frequency of the stall timeout)
func (w *Worker) getStallDetectionticker() *time.Ticker {
	stallDetectionFrequency := w.StallTimeout / 2
	if stallDetectionFrequency > minStallDetectionFrequency {
		stallDetectionFrequency = minStallDetectionFrequency
	}
	return time.NewTicker(time.Duration(stallDetectionFrequency) * time.Millisecond)
}

// run is the core event loop
func (w *Worker) run() {
	stallDetectionTicker := w.getStallDetectionticker()

Loop:
	for {
		select {
		case cmd, ok := <-w.CommandsChannel:
			if !ok {
				// closed channel => stop and cleanup
				w.Logger.Println("Parent task manager closed command Channel")
				w.cleanTermination()
				break Loop
			}
			w.runCommand(cmd)
		case <-w.keepAliveChannel:
			//w.Logger.Println("Received keepalive")
			w.LastAliveAt = time.Now()
		case cmd := <-w.exitChannel:
			//w.Logger.Println("TERMINATED, forwarding stoppedworker msg to task manager")
			// forward stopped event and exit
			w.TaskFeedbackChannel <- cmd
			//break Loop => don't break here, let the task manager close the command channel
		case <-stallDetectionTicker.C:
			if w.HasStalled() {
				w.Logger.Println("Detected stalled worker", w.Pid)
				w.cleanTermination()
				//break Loop => don't break here, let the task manager close the command channel
			}
		}
	}

	stallDetectionTicker.Stop()
	close(w.exitChannel)
	//close(w.CommandsChannel)
	close(w.keepAliveChannel)
}

// invoked when the process stalled or when the command channel has been closed by the task manager
func (w *Worker) cleanTermination() {
	replyChan := make(chan CommandReply, 1)
	w.Stop(replyChan)
	reply := <-replyChan
	w.Logger.Println("Terminating worker process manager:", reply.String())
}

// runCommand executes a command received by the task manager
func (w *Worker) runCommand(cmd Command) {
	w.Logger.Println("Received command ", cmd.String())
	switch cmd.Type {
	case "stop":
		w.Stop(cmd.ReplyChannel)
	//case "kill":
	//	w.Kill(cmd.ReplyChannel)
	case "info":
		w.Info(cmd.ReplyChannel)
	default:
		w.Logger.Println("received unknown command", cmd.String())
		cmd.ReplyChannel <- CommandReply{Error: fmt.Errorf("command not recognised")}
	}
}

// stoppedCommand creates a command to signal back that the worker process has stopped
func (w *Worker) stoppedCommand(err error, state *os.ProcessState, stalled bool) Command {
	return Command{
		Type:         "stoppedworker",
		TaskName:     w.Taskname,
		ReplyChannel: make(chan CommandReply, 1),
		Params: map[string]interface{}{
			"pid":     w.Pid,
			"error":   err,
			"state":   state,
			"stalled": stalled,
		},
	}
}

// HasStalled checks if the worker process is alive and has sent a keep-alive message recently
func (w *Worker) HasStalled() bool {
	earliest := time.Now().UnixNano() - (w.StallTimeout * 1000000)
	return w.LastAliveAt.UnixNano() < earliest
}

// Stop the worker process
func (w *Worker) Stop(replyChan chan<- CommandReply) {
	//err := syscall.Kill(worker.Pid, syscall.SIGTERM)
	proc, err := os.FindProcess(w.Pid)
	if nil != err {
		w.Logger.Printf("worker.Stop(): Cannot find worker process %d: %s\n", w.Pid, err)
		//w.exitChannel <- w.dtoppedCommand(err, state, stalled)
		replyChan <- CommandReply{Reply: fmt.Sprintf("[%s] worker process %d already stopped", w.Taskname, w.Pid)}
		return
	}

	// attempt stopping the worker with a SIGTERM first
	err = proc.Signal(syscall.SIGTERM)
	if err != nil {
		if err.Error() == "os: process already finished" {
			w.Logger.Println("worker.Stop() SIGTERM sent to already dead process")
		} else {
			w.Logger.Printf("worker.Stop(): Error sending SIGTERM to worker process %d: %s\n", w.Pid, err)
		}
	}

	var msg string
	var cmd Command
	var state *os.ProcessState
	gracePeriod := time.Duration(w.GracePeriod) * time.Millisecond

	// wait until the process returns gracefully from the SIGTERM, or times out + is killed after a grace period
	select {
	case <-time.After(gracePeriod):
		w.Logger.Printf("Grace Period (%s) expired, killing worker process %d", gracePeriod, w.Pid)
		err = proc.Kill()
		msg = fmt.Sprintf("Worker process %d was still around after %s, killed.", w.Pid, gracePeriod)
		if nil != err {
			msg = fmt.Sprintf("worker.Stop(): Failed to kill process %d - %s", w.Pid, err.Error())
		}
		cmd = <-w.exitChannel // coming from waitOnProcess()
		cmd.Params["killed"] = true
	case cmd = <-w.exitChannel: // wait for the original waitpid() syscall in waitOnProcess() to return
		cmd.Params["killed"] = false
		err = nil
		msg = fmt.Sprintf("Worker process %d terminated gracefully", w.Pid)
		if state2, ok := cmd.Params["state"]; ok {
			state, ok = state2.(*os.ProcessState)
		}
		if err2, ok := cmd.Params["error"]; ok {
			if err, ok = err2.(error); ok {
				msg = fmt.Sprintf("Worker process %d terminated with error: (%T) %#v (%s)", w.Pid, err, err, state.String())
			}
		}
	}
	cmd.Params["stalled"] = w.HasStalled()
	//w.Logger.Println(msg)
	replyChan <- CommandReply{Reply: msg, Error: err}
	w.TaskFeedbackChannel <- cmd
}

// get actual error for syscall, and ignore ECHILD errors caused by double waitpid() syscall
func (w *Worker) getSyscallError(err error) error {
	if syserr, ok := err.(*os.SyscallError); ok {
		errno := syserr.Err.(syscall.Errno)
		if errno != 0 && errno != syscall.ECHILD {
			return errno
		}
		return nil
	}
	return err
}

/*
// Kill unconditionally kills the worker process
func (w *Worker) Kill(replyChan chan<- CommandReply) {
	var msg string
	proc, err := os.FindProcess(w.Pid)
	if nil != err {
		w.Logger.Printf("worker.Kill(): Cannot find worker process %d: %s\n", w.Pid, err)
		//w.exitChannel <- w.stoppedCommand(err, state, stalled)
		msg = fmt.Sprintf("worker process %d already stopped", w.Pid)
	} else {
		err = proc.Kill()
		if nil != err {
			msg = fmt.Sprintf("worker.Stop(): Failed to kill process %d - %s", w.Pid, err.Error())
		} else {
			msg = fmt.Sprintf("Worker process still around after %s, killed pid %d\n", gracePeriod, w.Pid)
		}
	}

	replyChan <- CommandReply{Reply: msg, Error: err}
	w.TaskFeedbackChannel <- w.stoppedCommand(err, nil, false)
}
*/

// IsProcessAlive checks if the process is still around
// @see http://stackoverflow.com/questions/15204162/check-if-a-process-exists-in-go-way
func (w *Worker) IsProcessAlive() bool {
	proc, err := os.FindProcess(w.Pid)
	if err != nil {
		// on unix, FindProcess always returns true
		return false
	}
	return nil == proc.Signal(syscall.Signal(0))
	//if syscall.ESRCH == proc.Signal(syscall.Signal(0)) {
	//	processIsDead
	//}
}

// Info returns the status of the current worker process
func (w *Worker) Info(replyChan chan<- CommandReply) {
	replyChan <- CommandReply{
		Reply: w.String(),
		Params: map[string]interface{}{
			"pid":     w.Pid,
			"started": w.StartedAt,
			"alive":   w.LastAliveAt,
		},
	}
}

// String implements the Stringer interface
func (w *Worker) String() string {
	return fmt.Sprintf(
		"Task: %s,\tPid: %d,\tStarted: %s,\tLast alive at: %s (%d seconds ago)",
		w.Taskname,
		w.Pid,
		w.StartedAt.Format(time.RFC3339),
		w.LastAliveAt.Format(time.RFC3339),
		int(time.Since(w.LastAliveAt).Seconds()))
}
