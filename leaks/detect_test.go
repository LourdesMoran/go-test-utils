package leaks

import (
	"strings"
	"testing"
)

func TestThreadDetectionLeak_Found(t *testing.T) {
	stop := make(chan bool)
	defer func() {
		stop <- true
		if x := recover(); x != nil {
			if strings.Contains(x.(string), "Potential Goroutines Leakage detected") {
				// Leaked goroutine found as expected.
				return
			}
		}
		panic("Leaking goroutine not detected")
	}()

	inspector := Inspector{}
	inspector.RunGoroutineLeakDetection("ThreadDetectionLeakFound", t, func(t *testing.T) {
		go func() {
			select {
			case <-stop:
				break
			}
		}()
	})
}

func TestThreadDetectionLeak_NotFound(t *testing.T) {
	defer func() {
		if x := recover(); x != nil {
			panic("Detection of leaked goroutines failed, there is no leak")
		}
	}()

	inspector := Inspector{}
	inspector.RunGoroutineLeakDetection("ThreadDetectionLeakFound", t, func(t *testing.T) {})
}
