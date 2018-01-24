package utils

import(
	"fmt"
	"net"
	"reflect"
	"testing"

	"github.com/pebbe/zmq4"
)

// GetFreePort Ask the kernel for a free open port that is ready to use
func GetFreePort() int {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := l.Close(); err != nil {
			fmt.Println("Error when closing tcp connection:", err)
		}
	}()

	return l.Addr().(*net.TCPAddr).Port
}

func TestZmqReadPartN(t *testing.T) {
	port := GetFreePort()

	chRes := make(chan []byte, 0)

	expected := []byte(`test`)

	// start listener
	go func(port int, chRes chan<- []byte) {
		receiver, _ := zmq4.NewSocket(zmq4.PULL)
		defer receiver.Close()

		addr := fmt.Sprintf("tcp://localhost:%d", port)
		receiver.Connect(addr)

		var buf []byte
		ZmqReadPartN(receiver, 1, &buf)
		chRes<-buf
		close(chRes)
	}(port, chRes)

	// start producer
	go func(port int, expected []byte) {
		sender, _ := zmq4.NewSocket(zmq4.PUSH)
		defer sender.Close()
		addr := fmt.Sprintf("tcp://*:%d", port)
		err := sender.Bind(addr)
		if err != nil {
			t.Error(err)
		}

		// 3-part message, we only care about the 2nd
		sender.SendBytes([]byte(`dummy`), zmq4.SNDMORE)
		sender.SendBytes(expected, zmq4.SNDMORE)
		sender.SendBytes([]byte(`dummy`), 0)
	}(port, expected)

	actual := <- chRes
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("Was expecting the 2nd part of the message (%s), got '%s'", string(expected), string(actual))
	}
}