package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/bobappleyard/readline"
	wpmutils "github.com/quipo/workerpoolmanager/utils"
)

var autocomplete []string

// tab-completion from the autocomplete strings array
func taskNameCompleter(query, ctx string) []string {
	// find the prefix
	pos := strings.LastIndex(query, " ")
	prefix := query
	if pos >= 0 {
		prefix = query[pos:]
	}
	var candidates []string
	for _, name := range autocomplete {
		if strings.HasPrefix(name, prefix) {
			candidates = append(candidates, name)
		}
	}
	return candidates
}

func main() {
	host := flag.String("host", "http://localhost:8000", "hostname")
	flag.Parse()

	var client wpmutils.JobqueueHTTPClient

	msg, err := client.Open(*host)
	if err != nil {
		fmt.Println("Invalid address " + (*host) + ":" + err.Error())
		os.Exit(-1)
	}
	fmt.Println(msg)

	//autocomplete = wpmutils.ListTaskNames(*taskdir)
	//readline.Completer = readline.FilenameCompleter

	autocomplete = append(client.ListAsList(), getKeywords()...)
	readline.Completer = taskNameCompleter

	// loop until ReadLine returns nil (signalling EOF)
Loop:
	for {
		rl, err := readline.String("> ")
		if err == io.EOF {
			break Loop
		}
		if err != nil {
			fmt.Println("Error: ", err)
			break Loop
		}
		if "" == rl {
			// ignore blank lines
			continue Loop
		}

		// allow user to recall this line
		readline.AddHistory(rl)

		// split the main command from its (optional) parameters
		command, parameters := parseReadline(rl)

		var msg string

		switch command {
		case "quit", "exit":
			fmt.Println()
			break Loop //exit loop
		case "ls", "list":
			msg, err = client.List()
		case "lw", "listworkers":
			msg, err = client.ListWorkers(parameters)
		case "set":
			taskname, p, v := parseParams(parameters)
			msg, err = client.Set(taskname, p, v)
		case "start":
			msg, err = client.Start(parameters)
		case "stop":
			msg, err = client.Stop(parameters)
		case "st", "status":
			msg, err = client.Status(parameters)
		case "connect", "open":
			msg, err = client.Open(parameters)
			if err == nil {
				// refresh auto-complete list
				autocomplete = append(client.ListAsList(), getKeywords()...)
			}
		case "help", "h":
			msg = getHelp()
		default:
			msg = "ERROR: Command not recognised: " + command
		}

		if err != nil {
			msg = "ERROR: " + err.Error()
		}
		fmt.Println(msg)
	}
}

func getHelp() string {
	return `
	Available commands:
	- open <host:port>, connect <host:port>
	     Connect to a TaskManagerRunner listening to this address/port
	- ls, list
	     List available task managers
	- status [<taskname>]
	     Show the status of the task manager(s)
	- start <taskname>
	     Start a specific task manager
	- stop [<taskname>]
	     Stop all running task managers, or a specific one
	- set <taskname> cardinality <num>
	     Set the number of workers for a specific task manager 
	- lw <taskname>, listworkers <taskname>
	     List the status of each running worker process for a specific task manager
	- quit, exit
	     Quit the console
	- help
	     Print this message
	`
}

func getKeywords() []string {
	return []string{"ls", "lw", "list", "listworkers", "set", "start", "stop", "status", "open", "localhost", "cardinality", "quit", "exit"}
}

func parseReadline(rl string) (command string, params string) {
	parts := strings.SplitN(rl, " ", 2)
	if len(parts) == 1 {
		parts = append(parts, "")
	}
	return parts[0], parts[1]
}

func parseParams(rl string) (taskname string, param string, value string) {
	parts := strings.SplitN(rl, " ", 3)
	for len(parts) < 3 {
		parts = append(parts, "")
	}
	return parts[0], parts[1], parts[2]
}
