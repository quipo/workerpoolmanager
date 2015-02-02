package utils

import (
	"fmt"
	"log"
	"time"

	zmq "github.com/pebbe/zmq4"
)

// ZmqPubSubProxy Implements a many-to-many device on a zmq PUB-SUB connection
func ZmqPubSubProxy(host string, portIn int, portOut int, logger *log.Logger) {
	xsub, _ := zmq.NewSocket(zmq.SUB)
	xpub, _ := zmq.NewSocket(zmq.PUB)

	defer xsub.Close()
	defer xpub.Close()

	addrIn := fmt.Sprintf("tcp://*:%d", portIn)
	addrOut := fmt.Sprintf("tcp://*:%d", portOut)

	logger.Println("ZMQ XSUB on", addrIn)
	xsub.Bind(addrIn)
	xsub.SetSubscribe("")

	logger.Println("ZMQ XPUB on", addrOut)
	xpub.Bind(addrOut)

	poller := zmq.NewPoller()
	poller.Add(xsub, zmq.POLLIN)

	for {
		// keep looping
		sockets, _ := poller.Poll(5 * time.Second)
		for _, socket := range sockets {
			switch s := socket.Socket; s {
			case xsub:
				ZmqSendMulti(xpub, ZmqRecvMulti(s))
			}
		}
	}
}

// ZmqRecvMulti Receives a multi-part message and return it as a slice of strings
func ZmqRecvMulti(s *zmq.Socket) []string {
	var msg []string
	more := true
	for more {
		part, _ := s.Recv(0)
		msg = append(msg, part)
		more, _ = s.GetRcvmore()
	}
	return msg
}

// ZmqSendMulti Sends a slice of strings as a multi-part message
func ZmqSendMulti(s *zmq.Socket, msg []string) {
	lastIdx := len(msg) - 1
	for idx, part := range msg {
		if idx == lastIdx {
			s.Send(part, 0)
		} else {
			s.Send(part, zmq.SNDMORE)
		}
	}
}
