package taskmanager

import (
	"testing"
)

func TestMapToString(t *testing.T) {
	m := map[string]string{
		"b": "y",
		"a": "x",
		"c": "z",
	}
	expected := "a:\tx\nb:\ty\nc:\tz\n"
	actual := mapToString(m)
	if actual != expected {
		t.Errorf("Sorting map failed: expected\n%s\ngot\n%s", expected, actual)
	}
}
