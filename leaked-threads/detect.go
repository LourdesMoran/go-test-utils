package leaked_threads

import (
	"bytes"
	"fmt"
	"log"
	"runtime"
	"runtime/pprof"
	"strings"
	"testing"
	"time"
)

// TestingInspector tools to examine running tests suites
type TestingInspector struct{}

// RunWithGoroutineLeakageDetection detects potential Goroutines Leakage while running tests suites.
// Runs the given test and compares the amount of Goroutines previous and after the tests are ran.
// If an increase of Goroutines is detected, prints a warning about the
// specific test with the potential Goroutine Leakage and a stacktrace of the Goroutines.
// Params:
// testName: Name of the test
// t: testing context
// testFunc: test function
func (i *TestingInspector) RunWithGoroutineLeakageDetection(testName string, t *testing.T, testFunc func(t *testing.T)) {
	initialStackTrace, initialGoroutineNumber := i.GetGoroutinesStackTrace()
	// Run test suite
	t.Run(testName, testFunc)
	time.Sleep(1 * time.Second) // A moment for all pending goroutines to finish
	finalStackTrace, finalGoroutineNumber := i.GetGoroutinesStackTrace()
	log.Printf("%s: %s %d %d",
		"goroutine-leakage-detection", "Initial and Final number of goroutines after running test: ",
		initialGoroutineNumber, finalGoroutineNumber)

	// Compare Goroutines from before and after the test suite was ran
	// Print only the new goroutines created during the test suite run that are still alive.
	var newGoroutinesStackTrace string
	// filter results
	for endKey := range finalStackTrace {
		if i.isKeyInMap(endKey, initialStackTrace) {
			continue
		}
		if i.ignoredGoroutines(finalStackTrace[endKey]) {
			// Continue on goroutines that ar known to not terminate safely
			continue
		}
		newGoroutinesStackTrace += finalStackTrace[endKey]
	}
	if newGoroutinesStackTrace != "" {
		// Print to stdout
		panic(fmt.Sprintf(
			"Potential Goroutines Leakage detected in %s. See the following stack trace: \n %v",
			testName,
			newGoroutinesStackTrace,
		))
	}
}

func (i *TestingInspector) GetGoroutinesStackTrace() (goroutines map[string]string, total int) {
	startingGoroutineNumber := runtime.NumGoroutine()
	var b bytes.Buffer
	err := pprof.Lookup("goroutine").WriteTo(&b, 1)
	if err != nil {
		log.Printf("%s: %s", "goroutine-leakage-detection", err.Error())
	}
	startStackTrace := i.goroutineStackTraceToMap(b.String())

	return startStackTrace, startingGoroutineNumber
}

func (i *TestingInspector) goroutineStackTraceToMap(stackTrace string) (goroutines map[string]string) {
	goroutines = make(map[string]string)
	lines := strings.Split(stackTrace, "\n")
	header := ""
	for _, line := range lines {
		if strings.HasPrefix(line, "1 @") {
			header = line
			goroutines[header] = line + "\n"
		} else {
			goroutines[header] += line + "\n"
		}
	}
	return
}

func (i *TestingInspector) isKeyInMap(key string, lookupMap map[string]string) bool {
	for mapKey := range lookupMap {
		if mapKey == key {
			return true
		}
	}
	return false
}

func (i *TestingInspector) ignoredGoroutines(stack string) bool {
	ignoredGoroutines := []string{
		// these leveldb goroutines are async terminated even after calling db.Close,
		// so they may still alive after test end, thus cause false positive leak failures.
		// sleeping after closing without knowing how much is not a good solution
		// FIXME: Investigate a better way to handle these. There are several other projects on GitHub using goleveldb with this issue.
		"leveldb/util.(*BufferPool).drain",
		"goleveldb/leveldb.(*DB).compactionError",
		"goleveldb/leveldb.(*DB).mCompaction",
		"goleveldb/leveldb.(*DB).tCompaction",
		"goleveldb/leveldb.(*DB).mpoolDrain",
		// ignore any goroutines caused by the profiler
		"runtime/pprof.writeRuntimeProfile",
	}
	for _, igr := range ignoredGoroutines {
		if strings.Contains(stack, igr) {
			return true
		}
	}
	return false
}
