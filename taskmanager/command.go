package taskmanager

import (
	"errors"
	"fmt"
	"time"
)

// CommandReply is the type of a reply on the ReplyChannel for a Command.
// It contains the successful response (string) or an error on command failure
type CommandReply struct {
	Reply string
	Error error
}

// Command sent on the command channel. Might be specific to a task or generic.
// The type can be one of 'status', 'set', 'stop', 'listworkers', 'stopworkers' or 'stoppedworkers'
type Command struct {
	Type         string      `json:"type"`
	Name         string      `json:"name,omitempty"`
	Value        interface{} `json:"value,omitempty"`
	TaskName     string      `json:"taskname,omitempty"`
	Timeout      int64       `json:"timeout"`
	ReplyChannel chan CommandReply
}

// Implement String() interface
func (cmd Command) String() string {
	return fmt.Sprintf("[Type: '%s', TaskName: '%s', Name: '%s', Value: '%+v']", cmd.Type, cmd.TaskName, cmd.Name, cmd.Value)
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

// Broadcast the command to other channels, wait for all the replies and close the channel
func (cmd *Command) Broadcast(outChannels map[string]chan Command) bool {
	//log.Println("Broadcast()", cmd.String())
	// create new channel to collect individual replies and swap it in the original channel
	collector := make(chan CommandReply)
	replies := cmd.ReplyChannel
	cmd.ReplyChannel = collector

	cnt := len(outChannels)
	//log.Println(cnt, "channels open")
	for _, out := range outChannels {
		out <- *cmd
	}
	if cnt == 0 {
		replies <- CommandReply{Reply: "", Error: errors.New("no active tasks")}
	}

	// get custom timeout from command
	timeout := time.Duration(10000) // 10s by default
	if cmd.Timeout > 0 {
		timeout = time.Duration(cmd.Timeout)
	}

	// wait for all channels to reply
	for cnt > 0 {
		select {
		case resp := <-cmd.ReplyChannel:
			replies <- resp
			cnt--
		case <-time.After(timeout * time.Millisecond):
			replies <- CommandReply{Reply: "", Error: errors.New("timeout")}
			cnt--
		}
	}
	close(replies)
	close(collector)
	return true
}

// Forward the command to another channel, wait for the reply and close the channel
func (cmd *Command) Forward(outChannel chan Command) bool {
	//log.Println("Forward()", cmd.String())
	outChannels := make(map[string]chan Command)
	outChannels[cmd.TaskName] = outChannel
	return cmd.Broadcast(outChannels)
}

// Send the Command and get the response(s) as string
func (cmd Command) Send(outChannel chan Command) string {
	outChannel <- cmd
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
