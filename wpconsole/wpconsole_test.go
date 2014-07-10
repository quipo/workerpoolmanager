package main

import (
	//"fmt"
	//"runtime"
	"testing"
	//"time"
)

func TestParseReadline(t *testing.T) {
	rl := "  start    TaskName param1 param2"

	cmd, args := parseReadline(rl)
	expectedCmd := "start"
	expectedArgs := "TaskName param1 param2"

	if cmd != expectedCmd {
		t.Errorf("Invalid line parsing: \nEXPECTED: %s \nACTUAL: %s", expectedCmd, cmd)
	}
	if args != expectedArgs {
		t.Errorf("Invalid line parsing: \nEXPECTED: %s \nACTUAL: %s", expectedArgs, args)
	}
}

func TestParseParams(t *testing.T) {
	rl := "  TaskName  cardinality   4 "

	task, param, value := parseParams(rl)
	expectedTask := "TaskName"
	expectedParam := "cardinality"
	expectedValue := "4"

	if task != expectedTask {
		t.Errorf("Invalid line parsing: \nEXPECTED: %s \nACTUAL: %s", expectedTask, task)
	}
	if param != expectedParam {
		t.Errorf("Invalid line parsing: \nEXPECTED: %s \nACTUAL: %s", expectedParam, param)
	}
	if value != expectedValue {
		t.Errorf("Invalid line parsing: \nEXPECTED: %s \nACTUAL: %s", expectedValue, value)
	}
}
