package taskmanager

import (
	"fmt"
	"log"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// A TaskManager is a process manager for a specific task,
// keeping the cardinality of the number of worker processes to the desired value,
// and managing keep-alives
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
	workerChannels    map[int]chan Command   // metadata about worker processes
	startedAt         time.Time              // when the task manager was started
	feedbackChannel   chan Command           // channel used to communicate back to the task runner when the task stops
	commandChannel    chan Command           // channel of commands from the HTTP and Signal handlers
	keepaliveChannels map[int]chan KeepAlive // channels of keep-alive messages for each worker reference
	logger            *log.Logger            // custom logger

	keepAliveHandler *JobqueueKeepAliveHandler // handler for keep-alive messages
	nStoppedWorkers  int64                     // number of workers stalled/stopped since the task was started
	nStalledWorkers  int64                     //number of workers stalled since the task was restarted
}

func (task *TaskManager) init() {
	if nil == task.logger {
		prefix := fmt.Sprintf("[TaskManager][%s] ", task.Name)
		task.logger = log.New(os.Stdout, prefix, log.Ldate|log.Ltime)
	}
	task.workerChannels = make(map[int]chan Command)
	task.keepaliveChannels = make(map[int]chan KeepAlive)
}

// NewTaskManager creates a new Task Manager instance
func NewTaskManager(name string, keepAliveConf KeepAliveConf, feedback chan Command) TaskManager {
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
	return mgr
}

// Start the workers for this task
func (task *TaskManager) Start(commands chan Command, cmd Command) {
	task.init()
	task.logger.Println("Start()")

	task.Active = true
	task.startedAt = time.Now()
	task.commandChannel = commands

	// Create a channel for keep-alives for this task
	keepalives := make(chan KeepAlive, 10)
	// Start the keep-alive handler
	go task.keepAliveHandler.Run(keepalives)
	task.logger.Println("Started keepAliveHandler")

	for i := 0; i < task.Cardinality; i++ {
		err := task.StartWorker()
		if nil != err {
			// if a worker fails at startup, stop the task altogether
			task.logger.Println(err)
			task.logger.Println("Stopping the task manager, as one or more workers failed at startup")
			task.Stop()
			cmd.Fail(err.Error())
		}
	}

	task.run(keepalives)

	// cleanup
	task.Stop()
	close(keepalives)
	task.logger.Println("Terminated task" + task.Name)
	cmd.Success("Terminated task" + task.Name)
}

func (task *TaskManager) cleanup() {
	tick := time.NewTicker(time.Duration(task.StallTimeout) * time.Millisecond)

	for len(task.workerChannels) > 0 {
		select {
		case command := <-task.feedbackChannel:
			switch command.Type {
			case "stoppedworker":
				task.cleanWorkerReference(command)
			default:
				task.logger.Println("Task is shutting down, ignoring command", command.String())
			}
		case <-tick.C:
			task.logger.Printf("Task is shutting down, %d worker processes still alive\n", len(task.workerChannels))
		}
	}

	tick.Stop()
	task.feedbackChannel <- Command{Type: "stopped", TaskName: task.Name, ReplyChannel: make(chan CommandReply, 1)}
}

func (task *TaskManager) run(keepalives <-chan KeepAlive) {
	// loop until asked to terminate
	for task.Active {
		select {
		case command := <-task.commandChannel:
			task.RunCommand(command)
		case keepalive := <-keepalives: // keep-alives for workers of the current task
			//log.Println("Received keepalive for", task.Name, keepalive.TaskName)
			if keepalive.TaskName == task.Name {
				task.keepWorkerAlive(keepalive.Pid, keepalive)
			}
		}
	}
}

// StartWorker creates a new worker process
func (task *TaskManager) StartWorker() error {
	// make a copy of the logger, we want a custom prefix for each worker
	wLogger := log.New(os.Stdout, "", log.Ldate)
	*wLogger = *task.logger
	w := Worker{
		Logger:              wLogger,
		Taskname:            task.Name,
		Command:             task.Cmd,
		Args:                task.Args,
		CaptureOutput:       task.CaptureOutput,
		StallTimeout:        task.StallTimeout,
		GracePeriod:         task.GracePeriod,
		TaskFeedbackChannel: task.commandChannel,
	}

	info, err := w.Start()
	if nil != err {
		task.logger.Println(err)
		return err
	}
	task.workerChannels[info.Pid] = info.CommandChannel
	task.keepaliveChannels[info.Pid] = info.KeepAliveChannel
	return nil
}

// RunCommand runs a command on this task.
// Results are sent to the reply channel of the command itself
func (task *TaskManager) RunCommand(cmd Command) {
	task.logger.Println("RunCommand()", cmd.Type, cmd.String())

	// process
	switch cmd.Type {
	case "status":
		cmd.SafeReply(CommandReply{Reply: mapToString(task.Status())})
	case "set":
		task.Set(cmd)
	case "stop":
		task.Stop()
		cmd.SafeReply(CommandReply{Reply: "Stopped " + task.Name + " workers"})
	case "listworkers":
		cmd.SafeReply(CommandReply{Reply: strings.Join(task.ListWorkers(), "\n")})
	case "stopworkers":
		pids := cmd.Params["pids"].([]int)
		task.StopWorkersByPid(pids)
		cmd.SafeReply(CommandReply{Reply: fmt.Sprintf("Stopped individual workers (%v)", pids)})
	case "stoppedworker":
		task.cleanWorkerReference(cmd)
		//task.MaintainWorkerCardinality() //this conflicts with stopWorkers()
	}
}

func (task *TaskManager) cleanWorkerReference(cmd Command) {
	pid := cmd.Params["pid"].(int)
	died := false
	stalled := false
	defer func() {
		if err := recover(); nil != err {
			task.logger.Println("Recovered in cleanWorkerReference(", pid, ") after panic:", err)
			delete(task.keepaliveChannels, pid) // invalid channel, remove reference
			task.restoreCardinality(stalled, died)
		}
	}()
	if -1 != pid {
		stalled, _ = cmd.Params["stalled"].(bool)
		state, _ := cmd.Params["state"].(*os.ProcessState)
		died, _ = cmd.Params["died"].(bool)
		err, _ := cmd.Params["error"].(error)
		task.logger.Printf("Terminated worker [%d] State: %s, Error: %v\n", pid, state.String(), err)
		task.nStoppedWorkers++
		if stalled {
			task.nStalledWorkers++
		}

		close(task.workerChannels[pid])
		delete(task.workerChannels, pid)
		task.restoreCardinality(stalled, died)
	}
}

func (task *TaskManager) restoreCardinality(stalled bool, died bool) {
	if task.Active && (stalled || died) {
		// worker died unexpectedly, or stalled => restore worker cardinality
		for len(task.workerChannels) < task.Cardinality {
			task.StartWorker()
		}
	}
}

func (task *TaskManager) keepWorkerAlive(pid int, keepalive KeepAlive) {
	defer func() {
		if err := recover(); nil != err {
			task.logger.Println("Recovered in keepWorkerAlive() after panic:", err)
			delete(task.keepaliveChannels, pid) // invalid channel, remove reference
		}
	}()

	if ch, ok := task.keepaliveChannels[pid]; ok {
		ch <- keepalive
	} else {
		task.logger.Println("Received KeepAlive from un-monitored worker process", pid)
	}
}

// Stop asks all of this task's workers stop gracefully
// (or forcefully if they don't terminate in a timely fashion)
func (task *TaskManager) Stop() {
	//task.logger.Println("Stop()")
	task.Active = false
	task.startedAt = time.Unix(0, 0)
	task.StopWorkers()
}

// Status gets the status for this task (number of active workers, last alive TS, etc.)
func (task *TaskManager) Status() (ret map[string]string) {
	//task.logger.Println("Status()", len(task.workerChannels))
	runningWorkers := len(task.workerChannels)

	ret = make(map[string]string)
	ret["Task name"] = task.Name
	if task.Active {
		ret["Running"] = "true"
		ret["Started at"] = task.startedAt.Format(time.RFC3339)
		ret["Last alive at"] = task.lastAliveWorker().Format(time.RFC3339)
		ret["Active Workers"] = fmt.Sprintf("%d / %d", runningWorkers, task.Cardinality)
		ret["Dead Workers"] = fmt.Sprintf("%d", task.nStoppedWorkers)
		ret["Stalled Workers"] = fmt.Sprintf("%d", task.nStalledWorkers)
	} else {
		ret["Running"] = "false"
	}
	return ret
}

func (task *TaskManager) lastAliveWorker() time.Time {
	lastAliveAt := task.startedAt
	n := len(task.workerChannels)
	if n > 0 {
		replyCh := make(chan CommandReply, n)
		cmd := Command{Type: "info", ReplyChannel: replyCh}
		cmd.Broadcast(task.workerChannels)
		for reply := range replyCh {
			if la, ok := reply.Params["alive"]; ok {
				//task.logger.Printf("worker %d said to be alive at %s\n", reply.Params["pid"], reply.Params["alive"])
				if lastAliveAt.Before(la.(time.Time)) {
					lastAliveAt = la.(time.Time)
				}
			}
		}
	}
	return lastAliveAt
}

// ListWorkers returns status information from each worker process for this task
func (task *TaskManager) ListWorkers() []string {
	n := len(task.workerChannels)
	replies := make([]string, 0, n)
	if n > 0 {
		replyCh := make(chan CommandReply, n)
		cmd := Command{Type: "info", ReplyChannel: replyCh}
		cmd.Broadcast(task.workerChannels)
		for reply := range replyCh {
			replies = append(replies, reply.Reply)
		}
		sort.Strings(replies)
	}
	return replies
}

// Set task options (only "cardinality" is supported ATM)
func (task *TaskManager) Set(cmd Command) {
	var msg string
	var err error

	for opt, value := range cmd.Params {
		switch opt {
		case "cardinality":
			v, e := strconv.Atoi(value.(string))
			if e != nil {
				err = e
			} else {
				task.Cardinality = v
				task.MaintainWorkerCardinality()
				msg = "Changed workers cardinality"
			}
		default:
			err = fmt.Errorf("unknown task config update command: %s\n", cmd.String())
		}
	}

	cmd.SafeReply(CommandReply{Reply: msg, Error: err})
}

// utility method
func mapToString(m map[string]string) string {
	msg := ""
	for k, v := range m {
		msg = fmt.Sprintf("%s%s:\t%s\n", msg, k, v)
	}
	return msg
}

// StopWorkers asks all workers for this task to stop and waits for them to terminate
func (task *TaskManager) StopWorkers() {
	pids := make([]int, 0, len(task.workerChannels))
	for pid, _ := range task.workerChannels {
		pids = append(pids, pid)
	}
	task.StopWorkersByPid(pids)
}

// StopWorkersByPid asks all workers in the pid list to stop and waits for them to terminate
func (task *TaskManager) StopWorkersByPid(pids []int) {
	var wg sync.WaitGroup
	wg.Add(len(pids))
	//task.logger.Println("STOPPING", len(pids), "workers")
	for _, pid := range pids {
		if ch, ok := task.workerChannels[pid]; ok {
			go func(pid int, ch chan Command) {
				defer wg.Done()
				task.StopWorker(pid, ch)
				//task.logger.Println("STOPPED WORKER", pid)
			}(pid, ch)
		} else {
			//task.logger.Println("INVALID WORKER CHANNEL FOR PID", pid)
			wg.Done()
		}
	}
	wg.Wait()
	//task.logger.Println("StopWorkerByPid() ENDED. All WORKERS returned.")
}

// StopWorker sends a SIGTERM signal to the worker process identified by the given pid
func (task *TaskManager) StopWorker(pid int, ch chan Command) {
	var err error

	// the command channel for this worker might be closed
	/*
		defer func() {
			r := recover()
			if nil != r {
				task.logger.Println("Recovered in StopWorker() after panic:", r)
			}

			//remove reference
			delete(task.workerChannels, pid)
		}()
	*/
	//task.logger.Println("ATTEMPT STOPPING WORKER", pid)
	replyChan := make(chan CommandReply, 1)
	ch <- Command{
		Type:         "stop",
		ReplyChannel: replyChan,
	}
	// in case of panic writing to a closed channel, we don't have to wait for a reply
	if nil != err {
		return
	}
	task.logger.Println(<-replyChan)
}

// MaintainWorkerCardinality keeps the number of workers to the desired cardinality
func (task *TaskManager) MaintainWorkerCardinality() error {
	// start new workers if below the wanted cardinality
	for len(task.workerChannels) < task.Cardinality {
		task.logger.Println("Increasing number of workers from", len(task.workerChannels), "to", task.Cardinality)
		if err := task.StartWorker(); err != nil {
			return fmt.Errorf("Cannot start worker process. ERROR: %s", err.Error())
		}
		task.logger.Println("Increased number of workers to", len(task.workerChannels))
	}

	extraWorkers := len(task.workerChannels) - task.Cardinality
	if extraWorkers > 0 {
		// cardinality has been reduced, stop some workers
		pids := make([]int, 0)
		// get reference to a random worker
		for pid, _ := range task.workerChannels {
			pids = append(pids, pid)
			if len(pids) == extraWorkers {
				break
			}
		}
		task.logger.Printf("Decreasing number of workers from %d to %d\n", len(task.workerChannels), task.Cardinality)
		task.StopWorkersByPid(pids)
	}

	return nil
}

// CopyFrom Updates settings for a task manager from another task manager
func (task *TaskManager) CopyFrom(autotask TaskManager, tPath string) {
	if autotask.Cardinality > 0 {
		task.Cardinality = autotask.Cardinality
	}
	if len(autotask.Cmd) > 0 {
		task.Cmd = path.Join(tPath, autotask.Cmd)
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
