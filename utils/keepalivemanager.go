package utils

import (
	"encoding/json"
	"log"
	"os"
	"runtime"

	zmq "github.com/pebbe/zmq4"
)

// private struct, use constructor
type keepAliveManager struct {
	Taskname string
	Msg      []string
	Socket   *zmq.Socket
}

// NewKeepAliveManager initialise a ZeroMQ socket and the keep-alive message to send
func NewKeepAliveManager(taskname string, zmqAddress string) *keepAliveManager {
	mgr := keepAliveManager{Taskname: taskname, Msg: getKeepAliveMsg(taskname)}

	socket, err := zmq.NewSocket(zmq.PUB)
	if err != nil {
		log.Fatal(err)
	}

	//defer socket.Close()
	socket.Connect(zmqAddress)
	mgr.Socket = socket
	runtime.SetFinalizer(&mgr, mgr.Close)

	return &mgr
}

// SetAlive sends a keep-alive message to the manager
func (mgr *keepAliveManager) SetAlive() {
	log.Println("Sending keep-alive:", mgr.Msg)
	ZmqSendMulti(mgr.Socket, mgr.Msg)
}

// Close closes the ZeroMQ socket on exit
func (mgr *keepAliveManager) Close(m *keepAliveManager) {
	log.Println("Closing ZMQ socket")
	m.Socket.Close()
}

// init the 2-part ZeroMQ message
func getKeepAliveMsg(taskname string) []string {
	keepalive := struct {
		Taskname string
		Pid      int
	}{
		taskname,
		os.Getpid(),
	}
	jsonKeepalive, err := json.Marshal(keepalive)
	if err != nil {
		log.Fatal("Error preparing keep-alive message:", err)
	}
	return []string{taskname, string(jsonKeepalive)}
}
