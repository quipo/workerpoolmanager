# Task / worker pool manager in Go

[![GoDoc](https://godoc.org/github.com/quipo/workerpoolmanager/taskmanager?status.svg)](https://godoc.org/github.com/quipo/workerpoolmanager/taskmanager)

- Start cli tasks automatically 
- Maintain the desidered number of worker processes for each task
- Handle automatic restarts when a worker dies or stalls

The task manager will be able to start any cli (shell) script from the chosen directory.
For tasks that are long-running and meant to be monitored continuously, each worker process 
should regular keep-alive messages via a ZeroMQ PUB-SUB channel to communicate its health, 
and should handle SIGTERM messages when asked to terminate.
If the worker doesn't respond to a SIGTERM signal, it will be killed with SIGKILL after 
a (configurable) grace period.
The number of workers stalled/stopped since the task manager was started is reported in the task status.

The main package is the Task Manager (wpmanager), which can load the configuration, 
start some tasks automatically, and handle signals (CTRL+C) and HTTP requests to control 
the state of the tasks.

There's also a sample console application (wpconsole) to control the status and the 
configuration of each task from the command line. It supports controlling task managers 
running on different hosts/ports, and has a tab-completion interface (for both commands and task names).

## Usage

### Installation and configuration

```shell
go install github.com/quipo/workerpoolmanager/wpmanager
go install github.com/quipo/workerpoolmanager/wpconsole
```

Prepare a configuration file along the lines of [the provided example](resources/etc/wpmanager/example.json)

Start the manager by pointing it to the configuration file:

```shell
./wpmanager -conf=tasks.json
```

Terminate the manager with CTRL+C: the manager will ask the tasks 
(and all their workers) to stop gracefully before shutting down.

Control the status of the tasks using the console:

```shell
./wpconsole -host=localhost:8010

> ls
Task1
Task2
Task3

> set Task1 cardinality 10
Changed workers cardinality

> stop Task3
Stopped Task3 workers

> start Task3
Started task manager 

> listworkers Task3
Task: Task3,	Pid: 5991,	Started: 2014-01-23T18:05:55Z,	Last alive at: 2014-01-23T18:07:37Z
Task: Task3,	Pid: 6179,	Started: 2014-01-23T18:06:03Z,	Last alive at: 2014-01-23T18:07:41Z

> stop
Stopped Task3 workers
Stopped Task1 workers
Stopped Task2 workers

> status
No active tasks

> quit

```

or using the HTTP interface:

```shell
$ curl http://localhost:8010/tasks

No active tasks

$ curl -X POST http://localhost:8010/tasks/Task1/start

Started task manager

$ curl http://localhost:8010/tasks/Task1

Task name:      Task1
Running:        true
Started at:     2014-01-21T09:56:45Z
Last alive at:  2014-01-21T09:57:53Z
Active Workers: 1 / 1
Dead Workers:   0

$ curl -X POST http://localhost:8010/tasks/Task1/set/cardinality/10

Changed workers cardinality

$ curl http://localhost:8010/tasks/Task1

Task name:	    Task1
Running:	    true
Started at:	    2014-01-21T09:56:45Z
Last alive at:  2014-01-21T09:57:53Z
Active Workers: 10 / 10
Dead Workers:   1

$ curl -X DELETE http://localhost:8010/tasks/Task1

Stopped Task1 workers


$ curl -X DELETE http://localhost:8010/tasks

Stopped Task3 workers
Stopped Task2 workers

```



### Programmatic usage

Make sure you have the zeromq-devel and readline-devel packages installed.

Load the packages:

```shell
# install dependencies
go get github.com/codegangsta/martini
go get github.com/bobappleyard/readline
go get github.com/pebbe/zmq4

# install packages
go get github.com/quipo/workerpoolmanager
go get github.com/quipo/workerpoolmanager/taskmanager
go get github.com/quipo/workerpoolmanager/utils

```

Init a task runner:

```go
package main

import (
	"github.com/quipo/workerpoolmanager/taskmanager"
	"github.com/quipo/workerpoolmanager/utils"
)

func main() {
	//...
}
```



### API Documentation 

View the GoDoc generated documentation [here](http://godoc.org/github.com/quipo/workerpoolmanager).


### Worker examples

The [/resources/examples/](src/resources/examples/) folder contains some worker examples in a few languages.
Note the signal handler (to catch SIGTERM signals and terminate gracefully) and the keep-alive messages (sent by the workers to the manager).

## TODO

* Capture stdout from processes
* Better logging (implement INFO/WARNING/ERROR levels)
* Use [concurrent map](https://github.com/PokemonUniverse/nonamelib/blob/master/container/concurrentmap/concurrentmap.go) for workers?
* Test suite
* Command.Type => constants instead of string literals
* Send "stopping" message on keep-alive channel on worker exit

## Design

1. Task Runner 
   * holds a reference of all the available tasks
   * keeps an open channel to communicate with each Task Manager
   * keeps a command channel to communicate with the Signal and HTTP handlers

2. Task Manager
   * holds a reference of all the running workers
   * keeps an open channel to communicate with the Task Runner
   * has a ticker to regularly check for stalled workers

3. Signal Handler
   * Detects CTRL+C signals and asks the Task Runner to terminate

4. Keep-alive Handler
   * Listens for worker keep-alive messages on a ZMQ PubSub channel
   * Asks the Task Manager to update the workers' last-seen-alive datetime

5. HTTP Handler
   * Listens for requests to list, start and stop Task Managers, or change the cardinality of their workers

6. Console App / HTTP client
   * Tools/Libraries to communicate with the HTTP Handler, to get the status and control the Task Managers. 



```
                                Cmd channel
                               ╔══════════════════════════════════════════════════════════
                               ║                            ^                      ^
                               ║                            |                      |
                               ║                  +--------------------+           |
+--------+                     ║              ____| Keep-alive Handler |           |
|        |            +----------------+     /    +--------------------+           |
|        |      +---- | Task Manager 1 |-----          ^   zeromq   ^              |
|        |      |     |----------------|     \         |   PubSub   |              |
|        |------+---- |      ...       |      \    +----------+----------+-----+----------+
|        |      |     |----------------|       +---| Worker 1 | Worker 2 | ... | Worker N |
|        |      +---- | Task Manager N |        \  +----------+----------+-----+----------+
|        |            +----------------+         \         ^        ^            ^
|        |                    |                   \        |        |            |
|        |                    |                    \      +-------------------------+
|        |                    |                     +-----| Stalled Worker Detector |
|        |                    |                           +-------------------------+
|  Task  |  Cmd Channel       v
| Runner |════════════════════════════════════════════
|        |                    ^             ^
|        |                    |             |
|        |_____               |             |
|        |     \              |             |
|        |      \     +----------------+    |
|        |       ¯¯¯¯¯| Signal Handler |    |
|        |            +----------------+    |
|        |_____                             |
|        |     \                            |
|        |      \                   +----------------+
|        |       ¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯|  HTTP Handler  |
|        |                          +----------------+
+--------+                                 ^
                                           ║
                            ______________/ \_________________
                           /                                  \
                  +----------------+                  +----------------+
                  |   Console App  |                  |   HTTP Client  |
                  +----------------+                  +----------------+

```

## Contribute

Contributions are welcome. Please open pull requests or issue reports!



## Author

Lorenzo Alberton

* Web: [http://alberton.info](http://alberton.info)
* Twitter: [@lorenzoalberton](https://twitter.com/lorenzoalberton)
* Linkedin: [/in/lorenzoalberton](https://www.linkedin.com/in/lorenzoalberton)


## License

This repository is Copyright (c) 2014 Lorenzo Alberton, All rights reserved.
It is licensed under the MIT license. Please see the [LICENSE](LICENSE) file for applicable license terms.
