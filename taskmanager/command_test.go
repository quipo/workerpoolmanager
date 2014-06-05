package taskmanager

import (
	//"fmt"
	//"runtime"
	"testing"
	//"time"
)

func TestCommandFail(t *testing.T) {
	msg := "test error"
	reply := make(chan CommandReply, 1)
	cmd := Command{Type: "test", ReplyChannel: reply}
	cmd.Fail(msg)

	response := <-reply
	if response.Error == nil {
		t.Error("Was expecting an error response")
	}
	if response.Error.Error() != msg {
		t.Error("Was expecting same message in Error response")
	}
}

func TestCommandSuccess(t *testing.T) {
	msg := "test success"
	reply := make(chan CommandReply, 1)
	cmd := Command{Type: "test", ReplyChannel: reply}
	cmd.Success(msg)

	response := <-reply
	if response.Error != nil {
		t.Error("Was expecting a successful response")
	}
	if response.Reply != msg {
		t.Error("Was expecting same message in Reply")
	}
}

/*
func TestSend(t *testing.T) {
	runtime.GOMAXPROCS(2)
	testchannel := make(chan Command, 1)
	cmd := Command{Type: "test", ReplyChannel: make(chan CommandReply, 2)}

	go func(t *testing.T, x chan Command) {
		c := <-x
		c.ReplyChannel <- CommandReply{Reply: "testreply"}
		if c.Type != "test" {
			t.Error("Received unknown command")
		}
	}(t, testchannel)

	msg := cmd.Send(testchannel)
	if msg != "testreply" {
		t.Errorf("Received unexpected reply: expected 'testreply', got '%s'", msg)
	}
}

func TestBroadcast(t *testing.T) {
	testchannel1 := make(chan Command, 1)
	testchannel2 := make(chan Command, 1)
	reply := make(chan CommandReply, 2)
	cmd := Command{Type: "test", ReplyChannel: reply}
	cmd.Broadcast(map[string]chan Command{"a": testchannel1, "b": testchannel2})

	cnt := 0
	//for i := 0; i < 2; i++ {
	select {
	case x := <-testchannel1:
		cnt++
		x.ReplyChannel <- CommandReply{Reply: "x"}
	case y := <-testchannel2:
		cnt++
		y.ReplyChannel <- CommandReply{Reply: "y"}
	case <-time.After(1 * time.Second):
		//timeout
	}
	if 2 != cnt {
		t.Error(fmt.Sprintf("Broadcast command did not reach all channels (%d/2)", cnt))
	}
	//}

	response := <-reply
	if response.Error != nil {
		t.Error("Was expecting a successful response")
	}
}
*/
