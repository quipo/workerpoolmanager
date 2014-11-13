package taskmanager

import (
	//"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const minStallDetectionFrequency int64 = 5000 // check at least every 5 seconds

type CommandResponse interface {
	MarshalJSON() ([]byte, error)
	MarshalText() string
}

type KeyValueResponse struct {
	Value map[string]string
}

func (k KeyValueResponse) MarshalJSON() ([]byte, error) {
	return []byte("{\"foo\":\"bar\"}"), nil
}

func (k KeyValueResponse) MarshalText() string {
	return "Foo"
}

type StringResponse struct {
	Value string
}

func (r StringResponse) MarshalJSON() ([]byte, error) {
	return []byte("Foo"), nil
}

func (r StringResponse) MarshalText() string {
	return "String"
}

// TaskManager contains an instance of all its running worker processes,
// and controls they keep running until asked to terminate.
// Stalled workers are asked to terminate and restarted.
type TaskManager struct {
	Name          string   `json:"name,omitempty"`           // task name
	Cmd           string   `json:"cmd,omitempty"`            // cli command
	Args          []string `json:"args,omitempty"`           // cli args
	Cardinality   int      `json:"cardinality,omitempty"`    // number of workers
	StallTimeout  int64    `json:"stall_timeout,omitempty"`  // consider the worker dead if no keep-alives are received for this period (ms)
	AutoStart     bool     `json:"autostart,omitempty"`      // whether to start the task automatically
	GracePeriod   int64    `json:"grace_period,omitempty"`   // grace period (ms) before killing a worker after being asked to stop
	CaptureOutput bool     `json:"capture_output,omitempty"` //whether to capture the output and send it to stdout
	Active        bool
	// private struct members
	workers              map[int]Worker // metadata about worker processes
	startedAt            time.Time      // when the task manager was started
	feedbackChannel      chan<- Command // channel used to communicate back to the task runner when the task stops
	commandChannel       chan Command   // channel of commands from the HTTP and Signal handlers
	logger               *log.Logger    // custom logger
	stallDetectionTicker *time.Ticker   // ticker to check for stalled workers regularly
	//zombieDetectionTicker *time.Ticker              // ticker to check for zombie workers regularly (more expensive, but reliable)
	keepAliveHandler *JobqueueKeepAliveHandler // handler for keep-alive messages
	nStoppedWorkers  int64                     // number of workers stalled/stopped since the task was started
}

// NewTaskManager creates a new Task Manager instance
func NewTaskManager(name string, keepAliveConf KeepAliveConf, feedback chan<- Command) TaskManager {
	keepAlives := JobqueueKeepAliveHandler{
		Topic:  name,
		Host:   keepAliveConf.Host,
		Port:   keepAliveConf.InternalPort,
		Logger: log.New(os.Stdout, "[KeepAliveHandler] ", log.Ldate|log.Ltime),
	}
	mgr := TaskManager{
		Name:             name,
		keepAliveHandler: &keepAlives,
		StallTimeout:     keepAliveConf.StallTimeout,
		GracePeriod:      keepAliveConf.GracePeriod,
	}
	if mgr.StallTimeout == 0 {
		mgr.StallTimeout = 60000 // 1min by default
	}
	if mgr.GracePeriod == 0 {
		mgr.GracePeriod = 100 // 100ms by default
	}
	mgr.feedbackChannel = feedback
	mgr.logger = log.New(os.Stdout, "[TaskManager] ["+name+"] ", log.Ldate|log.Ltime)
	return mgr
}

// Run the workers for this task
func (task *TaskManager) Run(commands chan Command, cmd Command) {
	task.logger.Println("Run()")
	task.Active = true
	task.startedAt = time.Now()
	task.commandChannel = commands

	// Create a channel for keep-alives for this task
	keepalives := make(chan KeepAlive, 10)
	// Start the keep-alive handler
	go task.keepAliveHandler.Run(keepalives)
	task.logger.Println("Started keepAliveHandler")

	stallDetectionFrequency := task.StallTimeout / 2
	if stallDetectionFrequency > minStallDetectionFrequency {
		stallDetectionFrequency = minStallDetectionFrequency
	}

	// Periodically detect stalled workers (at least at twice the frequency of the stall timeout)
	task.stallDetectionTicker = time.NewTicker(time.Duration(stallDetectionFrequency) * time.Millisecond)
	// And once in 10 times, actually check for dead/zombie processes (not just their references)
	//task.zombieDetectionTicker = time.NewTicker(time.Duration(stallDetectionFrequency*10) * time.Millisecond)

	// Start task workers
	task.workers = make(map[int]Worker)

	// Start the workers for the first time.
	// Any failure here warrants stopping the task
	err := task.MaintainWorkerCardinality()
	if err != nil {
		fmt.Println(err)
		task.Stop()
		cmd.Fail(err.Error())
	} else {
		cmd.Success("Started task manager")
	}

	// loop until asked to terminate
	for task.Active {
		select {

		// handle commands for the current task
		case command := <-commands:
			task.RunCommand(command)

		// handle keep-alives for workers of the current task
		case keepalive := <-keepalives:
			//log.Println("Received keepalive for", task.Name, keepalive.TaskName)
			if keepalive.TaskName == task.Name {
				task.KeepWorkerAlive(keepalive.Pid)
			}

		// periodically detect stalled workers
		case <-task.stallDetectionTicker.C:
			//task.logger.Println("TICKER")
			go task.DetectStalledWorkers()
			task.MaintainWorkerCardinality()

			//go cleanZombies()

			// periodically detect zombie processes
			//case <-task.zombieDetectionTicker.C:
			//	//task.logger.Println("TICKER")
			//	go task.DetectZombies()
		}
	}

	// cleanup
	task.Stop()
	close(keepalives)
	task.logger.Println("Terminated task")
}

// https://groups.google.com/forum/?hl=en#!topic/golang-nuts/mR2kHbLhapE
func cleanZombies() {
	r := syscall.Rusage{}
	for {
		syscall.Wait4(-1, nil, 0, &r)
	}
}

// RunCommand runs a command on this task.
// Results are sent to the reply channel of the command itself
func (task *TaskManager) RunCommand(cmd Command) {
	//task.logger.Println("RunCommand()", cmd.Type)

	// process
	switch cmd.Type {
	case "status":
		cmd.ReplyChannel <- CommandReply{Reply: &KeyValueResponse{Value: task.Status()}, Error: nil}
	case "set":
		task.Set(cmd)
	case "stop":
		task.Stop()
		cmd.ReplyChannel <- CommandReply{Reply: &StringResponse{Value: "Stopped " + task.Name + " workers"}, Error: nil}
	case "listworkers":
		cmd.ReplyChannel <- CommandReply{Reply: &StringResponse{Value: strings.Join(task.ListWorkers(), "\n")}, Error: nil}
	case "stopworkers":
		pids := cmd.Value.([]int)
		task.StopWorkers(pids)
		cmd.ReplyChannel <- CommandReply{Reply: &StringResponse{Value: fmt.Sprintf("Stopped individual workers (%v)", pids)}, Error: nil}
	case "stoppedworker":
		task.logger.Println("TERMINATED WORKER", cmd.Value)
		task.nStoppedWorkers++
		delete(task.workers, cmd.Value.(int))
	}
}

// Stop asks all of this task's workers stop gracefully
// (or forcefully if they don't terminate in a timely fashion)
func (task *TaskManager) Stop() {
	//task.logger.Println("Stop()")
	task.Active = false
	task.stallDetectionTicker.Stop()
	//task.zombieDetectionTicker.Stop()
	task.startedAt = time.Unix(0, 0)

	pids := make([]int, 0)
	for pid, _ := range task.workers {
		pids = append(pids, pid)
	}
	task.StopWorkers(pids)
	task.feedbackChannel <- Command{Type: "stopped", TaskName: task.Name, ReplyChannel: make(chan CommandReply, 1)}
}

// StopWorkers asks all workers in the pid list to stop (don't wait for them to terminate)
func (task *TaskManager) StopWorkers(pids []int) {
	if len(pids) > 0 {
		var wg sync.WaitGroup
		gracePeriod := time.Duration(task.GracePeriod) * time.Millisecond
		for _, pid := range pids {
			if worker, ok := task.workers[pid]; ok {
				wg.Add(1)
				go func(w Worker) {
					defer wg.Done()
					w.Stop(gracePeriod, task.commandChannel)
				}(worker)
			}
		}
		wg.Wait()
	}
}

// Status gets the status for this task (number of active workers, last alive TS, etc.)
func (task *TaskManager) Status() (ret map[string]string) {
	//task.logger.Println("Status()")
	var lastAliveAt time.Time
	runningWorkers := 0
	for _, worker := range task.workers {
		runningWorkers++
		if worker.LastAliveAt.Unix() > lastAliveAt.Unix() {
			lastAliveAt = worker.LastAliveAt
		}
	}
	ret = make(map[string]string)
	ret["Task name"] = task.Name
	if task.Active {
		ret["Running"] = "true"
		ret["Started at"] = task.startedAt.Format(time.RFC3339)
		ret["Last alive at"] = lastAliveAt.Format(time.RFC3339)
		ret["Active Workers"] = fmt.Sprintf("%d / %d", runningWorkers, task.Cardinality)
		ret["Dead Workers"] = fmt.Sprintf("%d", task.nStoppedWorkers)
	} else {
		ret["Running"] = "false"
	}
	return ret
}

// ListWorkers gets the status for all the workers of this task (pid, started TS, last alive TS, etc.)
func (task *TaskManager) ListWorkers() (ret []string) {
	ret = make([]string, 0)
	for _, worker := range task.workers {
		ret = append(ret, worker.String())
	}
	return ret
}

// Set task options (only "cardinality" is supported ATM)
func (task *TaskManager) Set(cmd Command) {
	var msg string
	var err error

	switch cmd.Name {
	case "cardinality":
		v, e := strconv.Atoi(cmd.Value.(string))
		if e != nil {
			err = e
		} else {
			task.Cardinality = v
			task.MaintainWorkerCardinality()
			msg = "Changed workers cardinality"
		}
	default:
		err = errors.New("Unknown task config update command: " + cmd.String())
	}

	cmd.ReplyChannel <- CommandReply{Reply: &StringResponse{Value: msg}, Error: err}
}

// utility method
func mapToString(m map[string]string) string {
	msg := ""
	for k, v := range m {
		msg = fmt.Sprintf("%s%s:\t%s\n", msg, k, v)
	}
	return msg
}

// StartWorker Starts a new worker process for this task
func (task *TaskManager) StartWorker() error {
	//task.logger.Println("StartWorker()", task.Cmd, task.Args)
	cmd := exec.Command(task.Cmd, task.Args...)
	if task.CaptureOutput {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stdout
	}
	err := cmd.Start()
	if err != nil {
		return err
	}

	task.logger.Printf("Started new worker (pid %d) %s %+v\n", cmd.Process.Pid, task.Cmd, task.Args)
	worker := Worker{
		Pid:         cmd.Process.Pid,
		Taskname:    task.Name,
		StartedAt:   time.Now(),
		LastAliveAt: time.Now(),
		Logger:      task.logger,
	}
	if len(task.workers) < 1 {
		task.workers = make(map[int]Worker)
	}
	task.workers[cmd.Process.Pid] = worker
	cmd.Process.Release()
	return nil
}

// KeepWorkerAlive Updates the last-seen-alive timestamp for this worker process
func (task *TaskManager) KeepWorkerAlive(pid int) bool {
	//task.logger.Println("KeepWorkerAlive", pid)
	worker, ok := task.workers[pid]
	if !ok {
		task.logger.Println("Received keep-alive from unknown worker", pid)
		//task.logger.Printf("%+v", task.workers)
		return false
	}
	//task.logger.Println("Worker alive", pid)
	worker.LastAliveAt = time.Now()
	task.workers[pid] = worker
	return true
}

// DetectStalledWorkers Checks the workers for this task, and stop those which have stalled,
// i.e. those which haven't sent keep-alives in a while
func (task *TaskManager) DetectStalledWorkers() {
	//task.logger.Println("DetectStalledWorkers()")

	// all workers should have been alive in the past <StallTimeout> ms
	earliest := time.Now().UnixNano() - (task.StallTimeout * 1000000)

	// collect pids of stalled workers
	pids := make([]int, 0)
	for pid, worker := range task.workers {
		//task.logger.Printf("pid %d: %d < %d", pid, worker.LastAliveAt.UnixNano(), earliest)
		if worker.LastAliveAt.UnixNano() < earliest {
			task.logger.Println("DETECTED STALLED WORKER", pid)
			pids = append(pids, pid)
		}
	}
	if len(pids) > 0 {
		task.commandChannel <- Command{
			Type:         "stopworkers",
			TaskName:     task.Name,
			Name:         "pids",
			Value:        pids,
			ReplyChannel: make(chan CommandReply, 1)}
	}
}

// DetectZombies Actually checks the worker processes for this task,
// and cleans up zombies / dead processes and their references
func (task *TaskManager) DetectZombies() {
	task.logger.Println("DetectZombies()")
	for _, worker := range task.workers {
		go func(w Worker, confirmChannel chan<- Command) {
			w.CleanupProcessIfDead(confirmChannel)
		}(worker, task.commandChannel)
	}
}

// MaintainWorkerCardinality keeps the number of workers to the desired cardinality
func (task *TaskManager) MaintainWorkerCardinality() error {
	// start new workers if below the wanted cardinality
	for len(task.workers) < task.Cardinality {
		task.logger.Println("Increasing number of workers from", len(task.workers), "to", task.Cardinality)
		if err := task.StartWorker(); err != nil {
			return errors.New("Cannot start worker process. ERROR: " + err.Error())
		}
		task.logger.Println("Increased number of workers to", len(task.workers))
	}

	extraWorkers := len(task.workers) - task.Cardinality
	if extraWorkers > 0 {
		// cardinality has been reduced, stop some workers
		pids := make([]int, 0)
		// get reference to a random worker
		for pid, _ := range task.workers {
			pids = append(pids, pid)
			if len(pids) == extraWorkers {
				break
			}
		}
		task.logger.Println("Decreasing number of workers from", len(task.workers), "to", task.Cardinality)
		task.StopWorkers(pids)
	}

	return nil
}

// CopyFrom Updates settings for a task manager from another task manager
func (task *TaskManager) CopyFrom(autotask TaskManager) {
	if autotask.Cardinality > 0 {
		task.Cardinality = autotask.Cardinality
	}
	if len(autotask.Cmd) > 0 {
		task.Cmd = autotask.Cmd
	}
	if len(autotask.Args) > 0 {
		task.Args = autotask.Args
	}
	if autotask.Active {
		task.Active = true
	}
	if autotask.CaptureOutput {
		task.CaptureOutput = true
	}
}
