package log

import (
	"testing"
)

func TestLog(t *testing.T) {
	Debugf("hello %s", "world")
}
