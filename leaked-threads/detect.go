package leaked_threads

// TestingInspector tools to examine running tests suites
type TestingInspector struct{}

// RunWithGoroutineLeakageDetection has the goal of detecting potential Goroutines Leakage while running tests suites.
// It runs the given test suite and compares the amount of Goroutines previous and after the tests.
// If an increase of Goroutines is detected after the test suite finished running, it prints a warning about the
// specific test with the potential Goroutine Leakage and a stacktrace of the Goroutines.
// Params:
// t: testing context
// testSuite: test suite to run
// SuiteName: Name of the test suite
func (i *TestingInspector) RunWithGoroutineLeakageDetection(t *testing.T, testSuite suite.TestingSuite, suiteName string) {
	startingGoroutineNumber := runtime.NumGoroutine()
	var b bytes.Buffer
	_ = pprof.Lookup("goroutine").WriteTo(&b, 1)
	startStackTrace := i.goroutineStackTraceToMap(b.String())

	// Run test suite
	suite.Run(t, testSuite)
	time.Sleep(1 * time.Second) // A brief moment for all pending goroutines to finish

	// Compare Goroutines from before and after the test suite was ran
	endingGoroutineNumber := runtime.NumGoroutine()
	if endingGoroutineNumber-startingGoroutineNumber > 0 {
		// Print only the new goroutines created during the test suite run that are still alive.
		b.Truncate(0)
		err := pprof.Lookup("goroutine").WriteTo(&b, 1)
		if err != nil {
			log.Logger.WithPrefix("goroutine-leakage-detection").Error(err)
		}
		endStackTrace := i.goroutineStackTraceToMap(b.String())
		var newGoroutinesStackTrace string
		// filter results
		for endKey := range endStackTrace {
			if i.isKeyInMap(endKey, startStackTrace) {
				continue
			}
			if i.ignoredGoroutines(endStackTrace[endKey]) {
				// Continue on goroutines that ar known to not terminate safely
				continue
			}
			newGoroutinesStackTrace += endStackTrace[endKey]
		}
		if newGoroutinesStackTrace != "" {
			// Print to stdout
			panic(fmt.Sprintf(
				"Potential Goroutines Leakage detected in %s. See the following stack trace: \n %v",
				suiteName,
				newGoroutinesStackTrace,
			))
		}
	}
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
		// FIXME: Investigate a better way to handle these.
		// 	      There are several other projects on GitHub using goleveldb with this issue.
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
