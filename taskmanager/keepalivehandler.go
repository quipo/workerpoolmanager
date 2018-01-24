package taskmanager

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	zmq "github.com/pebbe/zmq4"
	wpmutils "github.com/quipo/workerpoolmanager/utils"
)

// KeepAlive contains the Pid of the process and the name of the Task
type KeepAlive struct {
	Pid      int
	TaskName string
}

// JobqueueKeepAliveHandler contains the configuration for the Keep-Alive handler
type JobqueueKeepAliveHandler struct {
	Host   string
	Port   int
	Topic  string
	Logger *log.Logger
}

// Implement String() interface
func (x KeepAlive) String() string {
	b, err := json.Marshal(x)
	if err != nil {
		return "Cannot encode KeepAlive struct to JSON"
	}
	return string(b)
}

// Run method: Listen to keep-alive messages sent via ZeroMQ and forward them to a channel
func (handler *JobqueueKeepAliveHandler) Run(keepalives chan<- KeepAlive) {
	if handler.Logger == nil {
		handler.Logger = log.New(os.Stdout, "[KeepAliveHandler] ", log.Ldate|log.Ltime)
	}
	receiver, _ := zmq.NewSocket(zmq.SUB)
	defer receiver.Close()

	addr := fmt.Sprintf("tcp://%s:%d", handler.Host, handler.Port)
	handler.Logger.Println("ZMQ SUB on", addr, handler.Topic)
	receiver.Connect(addr)
	receiver.SetSubscribe(handler.Topic)

	poller := zmq.NewPoller()
	poller.Add(receiver, zmq.POLLIN)

	// shut down cleanly when the keep-alive channel is closed
	defer func() {
		if r := recover(); r != nil {
			handler.Logger.Println("Recovering from panic (likely: trying to send msg on closed keep-alive channel)", r)
		}
	}()
	// at the end, close the channel to indicate that's all the work we have.
	defer close(keepalives)

	handler.Logger.Println("waiting for message for topic", handler.Topic)
	var keepaliveMsg []byte
	for {
		sockets, _ := poller.Poll(5 * time.Second)
		for _, socket := range sockets {
			switch s := socket.Socket; s {
			case receiver:
				var keepalive KeepAlive
				wpmutils.ZmqReadPartN(s, 1, &keepaliveMsg)
				err := json.Unmarshal(keepaliveMsg, &keepalive)
				if err != nil {
					handler.Logger.Println("Error decoding json string:", err)
					continue
				}
				//log.Println("[KeepAliveHandler] sending to channel ", keepalive)
				keepalives <- keepalive
			}
		}
	}
}
