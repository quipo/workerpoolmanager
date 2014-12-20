package taskmanager

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// CommandReply is the type of a reply on the ReplyChannel for a Command.
// It contains the successful response (string) or an error on command failure
type CommandReply struct {
	Reply  string
	Params map[string]interface{}
	Error  error
}

// String implements the Stringer interface
func (r CommandReply) String() string {
	s := "Reply: " + r.Reply
	if len(r.Params) > 0 {
		s += fmt.Sprintf(", Params: %#v", r.Params)
	}
	if nil != r.Error {
		s += fmt.Sprintf(", Error: %v", r.Error)
	}
	return s
}

// Command sent on the command channel. Might be specific to a task or generic.
// The type can be one of 'status', 'set', 'stop', 'listworkers', 'stopworkers' or 'stoppedworkers'
type Command struct {
	Type         string                 `json:"type"`
	TaskName     string                 `json:"taskname,omitempty"`
	Params       map[string]interface{} `json:"params,omitempty"`
	Timeout      int64                  `json:"timeout"`
	ReplyChannel chan CommandReply
}

// Implement String() interface
func (cmd Command) String() string {
	s := fmt.Sprintf("[Type: '%s', TaskName: '%s'", cmd.Type, cmd.TaskName)
	if len(cmd.Params) > 0 {
		s += fmt.Sprintf(", Params: %#v", cmd.Params)
	}
	return s + "]"
}

// ToJSON converts an interaction object to a JSON byte array
func (cmd Command) ToJSON() []byte {
	b, err := json.Marshal(cmd)
	if err != nil {
		fmt.Printf("ERROR: %s", err)
		//os.Exit(1)
	}
	return b
}

// Fail sends a Reply with a failure message
func (cmd *Command) Fail(msg string) bool {
	cmd.ReplyChannel <- CommandReply{Reply: "", Error: errors.New(msg)}
	close(cmd.ReplyChannel)
	return false
}

// Success sends a Reply with a success message
func (cmd *Command) Success(msg string) bool {
	cmd.ReplyChannel <- CommandReply{Reply: msg, Error: nil}
	close(cmd.ReplyChannel)
	return true
}

func (cmd *Command) SafeReply(reply CommandReply) error {
	var err error
	defer func() {
		r := recover()
		if nil != r {
			fmt.Println("[TaskManager] Recovered in cmd.SafeReply() after panic:", r)
			err = fmt.Errorf("%s", r)
		}
	}()
	cmd.ReplyChannel <- reply
	return err
}

// Broadcast the command to other channels, wait for all the replies and close the channel
func (cmd *Command) Broadcast(outChannels map[int]chan Command) bool {
	//log.Println("Broadcast()", cmd.String())
	// create new channel to collect individual replies and swap it in the original channel
	collector := make(chan CommandReply)
	replies := cmd.ReplyChannel
	cmd.ReplyChannel = collector

	cnt := len(outChannels)
	//log.Println(cnt, "channels open")
	for pid, out := range outChannels {
		fmt.Println("[Command.Broadcast()] Sending command to process", pid, cmd.String())
		err := cmd.SafeSend(out)
		if nil != err {
			fmt.Println("[Command.Broadcast()] Error sending command to process", pid, cmd.String())
		}
	}
	if cnt == 0 {
		replies <- CommandReply{Reply: "", Error: errors.New("no active tasks")}
	}

	// get custom timeout from command
	timeout := 1 * time.Minute // 1m by default
	if cmd.Timeout > 0 {
		timeout = time.Duration(cmd.Timeout) * time.Millisecond
	}

	// wait for all channels to reply
	cnt = len(outChannels)
	for cnt > 0 {
		select {
		case resp := <-cmd.ReplyChannel:
			replies <- resp
			cnt--
		case <-time.After(timeout):
			replies <- CommandReply{Reply: "", Error: errors.New("timeout")}
			cnt = 0
		}
	}
	close(replies)
	close(collector)
	return true
}

// SafeSend attempts sending a command to an output channel,
// recovering from the panic if the channel was closed in the meanwhile
func (cmd *Command) SafeSend(out chan Command) error {
	var err error
	defer func() {
		r := recover()
		if nil != r {
			fmt.Println("[TaskManager] Recovered in cmd.SafeSend() after panic:", r)
			err = fmt.Errorf("%s", r)
		}
	}()

	out <- *cmd
	return err
}

// Forward the command to another channel, wait for the reply and close the channel
func (cmd *Command) Forward(outChannel chan Command) bool {
	//log.Println("Forward()", cmd.String())
	outChannels := make(map[int]chan Command)
	outChannels[-1] = outChannel
	return cmd.Broadcast(outChannels)
}

// Send the Command and get the response(s) as string
func (cmd Command) Send(outChannel chan Command) string {
	cmd.SafeSend(outChannel)
	var msg string
	for resp := range cmd.ReplyChannel {
		if resp.Error != nil {
			msg = fmt.Sprintf("%s%s\n", msg, resp.Error)
		} else {
			msg = fmt.Sprintf("%s%s\n", msg, resp.Reply)
		}
	}
	return msg
}
