package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/quipo/workerpoolmanager/taskmanager"
)

func readConfig(filename string) taskmanager.TaskManagerConf {
	//fmt.Println("TaskManagerConf factory()")
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Println("Cannot read configuration file ", filename)
		os.Exit(1)
	}
	var taskMgrConf taskmanager.TaskManagerConf
	err = json.Unmarshal(b, &taskMgrConf)
	if err != nil {
		fmt.Println("Cannot parse configuration file: ", err)
		os.Exit(1)
	}
	return taskMgrConf
}

func main() {
	configuration := flag.String("conf", "", "path to configuration file")
	//os.Args = []string{"progname", "-taskdir", "-conf", "-int", "100"}

	flag.Parse()

	if "" == *configuration {
		fmt.Println("Usage: ", os.Args[0], "-conf=<file.conf>")
		flag.PrintDefaults()
		os.Exit(2)
	}
	//fmt.Println("CONF: ", *configuration)

	taskMgrConf := readConfig(*configuration)
	taskRunner, err := taskmanager.NewRunner(taskMgrConf)
	//os.Exit(1)

	if err != nil {
		fmt.Println("Error starting task manager runner: ", err)
		os.Exit(1)
	}

	//fmt.Println("Starting task manager runner...")
	taskRunner.Run()
}
