package taskmanager

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/codegangsta/martini"
)

// HTTPHandler holds the configuration for the HTTP handler
type HTTPHandler struct {
	Host           string
	Port           int
	CommandChannel chan Command
	Logger         *log.Logger
	ResponseFormat string
}

// Run the HTTP handler, which exposes an HTTP interface to control tasks
func (handler *HTTPHandler) Run() {
	if handler.Logger == nil {
		handler.Logger = log.New(os.Stdout, "[HTTPHandler] ", log.Ldate|log.Ltime)
	}
	m := martini.Classic()
	http.ListenAndServe(handler.Host+":"+string(handler.Port), m)

	m.Use(func(res http.ResponseWriter, req *http.Request) {
		handler.ResponseFormat = "text"
		if req.Header.Get("Accept") == "application/json" {
			handler.ResponseFormat = "json"
		}
	})

	// routing
	m.Get("/tasks", handler.taskStatus)
	m.Get("/tasks/:id", handler.taskStatus)

	m.Get("/list", handler.taskList)

	m.Get("/tasks/:id/start", handler.taskStart)
	m.Put("/tasks/:id/start", handler.taskStart)
	m.Post("/tasks/:id/start", handler.taskStart)

	m.Get("/tasks/:id/list", handler.taskListWorkers)

	m.Delete("/tasks", handler.taskStop)
	m.Delete("/tasks/:id", handler.taskStop)
	m.Put("/tasks/:id/stop", handler.taskStop)
	m.Post("/tasks/:id/stop", handler.taskStop)

	m.Get("/tasks/:id/set/:name/:value", handler.taskSetOption)
	m.Post("/tasks/:id/set/:name/:value", handler.taskSetOption)
	m.Put("/tasks/:id/set/:name/:value", handler.taskSetOption)

	//m.Run()

	address := fmt.Sprintf(":%d", handler.Port)
	handler.Logger.Println("HTTP handler listening on", address)

	handler.Logger.Fatalln(http.ListenAndServe(address, m))
}

//TODO: return appropriate HTTP status code in case of error

// List the available tasks
func (handler *HTTPHandler) taskList(params martini.Params) string {
	cmd := Command{TaskName: "", Type: "list", ReplyChannel: make(chan CommandReply, 1), ResponseFormat: handler.ResponseFormat}
	return cmd.Send(handler.CommandChannel)
}

// List the workers for a specific task
func (handler *HTTPHandler) taskListWorkers(params martini.Params) string {
	cmd := Command{TaskName: params["id"], Type: "listworkers", ReplyChannel: make(chan CommandReply, 1), ResponseFormat: handler.ResponseFormat}
	return cmd.Send(handler.CommandChannel)
}

// Start a specific task
func (handler *HTTPHandler) taskStart(params martini.Params) string {
	cmd := Command{TaskName: params["id"], Type: "start", ReplyChannel: make(chan CommandReply, 1), ResponseFormat: handler.ResponseFormat}
	return cmd.Send(handler.CommandChannel)
}

// Stop all tasks or just a specific one
func (handler *HTTPHandler) taskStop(params martini.Params) string {
	cmd := Command{TaskName: params["id"], Type: "stop", ReplyChannel: make(chan CommandReply, 1), ResponseFormat: handler.ResponseFormat}
	return cmd.Send(handler.CommandChannel)
}

// Get the status for one or more tasks
func (handler *HTTPHandler) taskStatus(params martini.Params) string {
	cmd := Command{TaskName: params["id"], Type: "status", ReplyChannel: make(chan CommandReply, 1), ResponseFormat: handler.ResponseFormat}
	return cmd.Send(handler.CommandChannel)
}

// Change the cardinality of workers for a specific task
func (handler *HTTPHandler) taskSetOption(params martini.Params) string {
	cmd := Command{TaskName: params["id"], Type: "set", Name: params["name"], Value: params["value"], ReplyChannel: make(chan CommandReply, 1), ResponseFormat: handler.ResponseFormat}
	return cmd.Send(handler.CommandChannel)
}
