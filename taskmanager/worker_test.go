package taskmanager

import (
	//"fmt"
	//"runtime"
	"os"
	"testing"
	//"time"
)

/*
func TestWorkerString(t *testing.T) {
	ts := time.Now().Add(-1 * time.Second)
	w := &Worker{Pid: 123, Taskname: "test", StartedAt: ts, LastAliveAt: ts}

	t.Error(w)
}
*/

func TestIsProcessAlive(t *testing.T) {
	w := &Worker{Pid: -1}

	if w.IsProcessAlive() {
		t.Error("Was expecting process with pid -1 not to exist", w.IsProcessAlive())
	}

	w.Pid = os.Getpid()
	if !w.IsProcessAlive() {
		t.Error("Cannot find self process", w.IsProcessAlive())
	}
}
