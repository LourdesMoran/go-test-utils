package leaks

import (
	"bytes"
	"fmt"
	"github.com/LourdesMoran/go-test-utils/utils"
	"log"
	"runtime"
	"runtime/pprof"
	"strings"
	"testing"
	"time"
)

// Inspector tools to examine running tests suites.
type Inspector struct{}

// RunGoroutineLeakDetection detects potential Goroutines Leakage while running tests suites.
// Runs the given test and compares the Goroutines previous and after the tests are ran.
// If an increase of Goroutines is detected raises panic about the
// specific test with the potential Goroutine Leakage and a stacktrace of the Goroutines.
// Params:
// testName: Name of the test
// t: testing context
// testFunc: test function
func (i *Inspector) RunGoroutineLeakDetection(testName string, t *testing.T, testFunc func(t *testing.T)) {
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
		if utils.IsKeyInMap(endKey, initialStackTrace) {
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

// GetGoroutinesStackTrace returns a map of the goroutines stack trace and the total number of goroutines.
func (i *Inspector) GetGoroutinesStackTrace() (goroutines map[string]string, total int) {
	startingGoroutineNumber := runtime.NumGoroutine()
	var b bytes.Buffer
	err := pprof.Lookup("goroutine").WriteTo(&b, 1)
	if err != nil {
		panic(fmt.Sprintf("%s: %s", "goroutine-leakage-detection", err.Error()))
	}
	startStackTrace := i.goroutineStackTraceToMap(b.String())

	return startStackTrace, startingGoroutineNumber
}

func (i *Inspector) goroutineStackTraceToMap(stackTrace string) (goroutines map[string]string) {
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

func (i *Inspector) ignoredGoroutines(stack string) bool {
	ignoredGoroutines := []string{
		// These leveldb goroutines are async terminated even after calling db.Close,
		// so they may still live after the test ends and cause false positive leak failures.
		// Sleeping after closing without knowing how much is not a good solution
		// FIXME: Investigate a better way to handle these.
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
