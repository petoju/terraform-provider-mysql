package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/olekukonko/tablewriter"
)

var (
	// MySQL versions to test
	mysqlVersions = []string{
		"mysql:5.6",
		"mysql:5.7",
		"mysql:8.0",
	}

	// Percona versions to test
	perconaVersions = []string{
		"percona:5.7",
		"percona:8.0",
	}

	// MariaDB versions to test
	mariadbVersions = []string{
		"mariadb:10.3",
		"mariadb:10.8",
		"mariadb:10.10",
	}

	// TiDB versions to test (version numbers only, not full image names)
	tidbVersions = []string{
		"6.1.7",
		"6.5.12",
		"7.1.6",
		"7.5.7",
		"8.1.2",
		"8.5.3",
	}
)

type testResult struct {
	image    string
	dbType   string
	passed   bool
	logFile  string
	duration time.Duration
}

type testJob struct {
	image       string
	dbType      string
	testPattern string
	testNum     int
}

var (
	outputMutex sync.Mutex
)

func main() {
	// Get test pattern from command line args, default to "WithTestcontainers"
	testPattern := "WithTestcontainers"
	if len(os.Args) > 1 {
		testPattern = os.Args[1]
	}

	// Get parallelism from environment variable
	parallel := getParallelism()

	fmt.Println("==========================================")
	fmt.Println("Testcontainers Matrix Test Suite")
	fmt.Println("==========================================")
	fmt.Printf("Test pattern: %s\n", testPattern)
	fmt.Printf("Parallelism: %d\n", parallel)
	fmt.Println()

	// Build all test jobs
	var jobs []testJob
	testNum := 0

	// MySQL tests
	for _, version := range mysqlVersions {
		testNum++
		jobs = append(jobs, testJob{
			image:       version,
			dbType:      "MySQL",
			testPattern: testPattern,
			testNum:     testNum,
		})
	}

	// Percona tests
	for _, version := range perconaVersions {
		testNum++
		jobs = append(jobs, testJob{
			image:       version,
			dbType:      "Percona",
			testPattern: testPattern,
			testNum:     testNum,
		})
	}

	// MariaDB tests
	for _, version := range mariadbVersions {
		testNum++
		jobs = append(jobs, testJob{
			image:       version,
			dbType:      "MariaDB",
			testPattern: testPattern,
			testNum:     testNum,
		})
	}

	// TiDB tests
	for _, version := range tidbVersions {
		testNum++
		jobs = append(jobs, testJob{
			image:       version,
			dbType:      "TiDB",
			testPattern: testPattern,
			testNum:     testNum,
		})
	}

	// Run tests (sequentially or in parallel)
	var results []testResult
	if parallel > 1 {
		results = runTestsParallel(jobs, parallel)
	} else {
		results = runTestsSequential(jobs)
	}

	// Print summary
	printSummary(results)

	// Exit with error code if any tests failed
	for _, result := range results {
		if !result.passed {
			os.Exit(1)
		}
	}
}

func getParallelism() int {
	parallelStr := os.Getenv("PARALLEL")
	if parallelStr == "" {
		return 1 // Default to sequential
	}

	parallel, err := strconv.Atoi(parallelStr)
	if err != nil || parallel < 1 {
		fmt.Fprintf(os.Stderr, "Warning: Invalid PARALLEL value '%s', using 1 (sequential)\n", parallelStr)
		return 1
	}

	// Cap parallelism at number of CPUs + 2 to avoid overwhelming the system
	maxParallel := runtime.NumCPU() + 2
	if parallel > maxParallel {
		fmt.Fprintf(os.Stderr, "Warning: PARALLEL=%d exceeds recommended max (%d), capping at %d\n", parallel, maxParallel, maxParallel)
		return maxParallel
	}

	return parallel
}

func runTestsSequential(jobs []testJob) []testResult {
	var results []testResult

	// Print section headers
	fmt.Println("==========================================")
	fmt.Println("MySQL Tests")
	fmt.Println("==========================================")
	for _, job := range jobs {
		if job.dbType == "MySQL" {
			result := runTest(job)
			results = append(results, result)
		}
	}

	fmt.Println()
	fmt.Println("==========================================")
	fmt.Println("Percona Tests")
	fmt.Println("==========================================")
	for _, job := range jobs {
		if job.dbType == "Percona" {
			result := runTest(job)
			results = append(results, result)
		}
	}

	fmt.Println()
	fmt.Println("==========================================")
	fmt.Println("MariaDB Tests")
	fmt.Println("==========================================")
	for _, job := range jobs {
		if job.dbType == "MariaDB" {
			result := runTest(job)
			results = append(results, result)
		}
	}

	fmt.Println()
	fmt.Println("==========================================")
	fmt.Println("TiDB Tests")
	fmt.Println("==========================================")
	for _, job := range jobs {
		if job.dbType == "TiDB" {
			result := runTest(job)
			results = append(results, result)
		}
	}

	return results
}

func runTestsParallel(jobs []testJob, parallel int) []testResult {
	// Create job channel
	jobChan := make(chan testJob, len(jobs))
	resultChan := make(chan testResult, len(jobs))

	// Send all jobs to channel
	for _, job := range jobs {
		jobChan <- job
	}
	close(jobChan)

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < parallel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobChan {
				result := runTest(job)
				resultChan <- result
			}
		}()
	}

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var results []testResult
	for result := range resultChan {
		results = append(results, result)
	}

	return results
}

func runTest(job testJob) testResult {
	// Synchronize output to prevent interleaving
	outputMutex.Lock()
	fmt.Println("----------------------------------------")
	fmt.Printf("[%d] Testing %s: %s\n", job.testNum, job.dbType, job.image)
	fmt.Println("----------------------------------------")
	outputMutex.Unlock()

	// Sanitize image name for log file
	logFile := fmt.Sprintf("/tmp/testcontainers-%s-%s.log", job.dbType, sanitizeImageName(job.image))

	start := time.Now()

	// Build the go test command
	cmd := exec.Command("go", "test",
		"-tags=testcontainers",
		"-v",
		"./mysql/...",
		"-run", job.testPattern,
		"-timeout", "15m",
	)

	// Set environment variables
	envVars := os.Environ()
	if job.dbType == "TiDB" {
		// TiDB uses version number, not full image name
		envVars = append(envVars, "TIDB_VERSION="+job.image)
	} else {
		envVars = append(envVars, "DOCKER_IMAGE="+job.image)
	}
	envVars = append(envVars, "TF_ACC=1", "GOTOOLCHAIN=auto")
	cmd.Env = envVars

	// Create log file
	logFileHandle, err := os.Create(logFile)
	if err != nil {
		outputMutex.Lock()
		fmt.Fprintf(os.Stderr, "Error creating log file %s: %v\n", logFile, err)
		outputMutex.Unlock()
		return testResult{
			image:   job.image,
			dbType:  job.dbType,
			passed:  false,
			logFile: logFile,
		}
	}
	defer logFileHandle.Close()

	// Capture both stdout and stderr
	cmd.Stdout = logFileHandle
	cmd.Stderr = logFileHandle

	// Run the command
	err = cmd.Run()
	duration := time.Since(start)

	// Read and display the log file (synchronized)
	logContent, readErr := os.ReadFile(logFile)
	outputMutex.Lock()
	if readErr == nil {
		fmt.Print(string(logContent))
	}

	passed := err == nil

	if passed {
		fmt.Printf("\033[0;32m✓ PASSED: %s %s\033[0m\n", job.dbType, job.image)
	} else {
		fmt.Printf("\033[0;31m✗ FAILED: %s %s\033[0m\n", job.dbType, job.image)
	}
	outputMutex.Unlock()

	return testResult{
		image:    job.image,
		dbType:   job.dbType,
		passed:   passed,
		logFile:  logFile,
		duration: duration,
	}
}

func sanitizeImageName(image string) string {
	// Replace colons and slashes with underscores
	result := strings.ReplaceAll(image, ":", "_")
	result = strings.ReplaceAll(result, "/", "_")
	return result
}

func printSummary(results []testResult) {
	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║              Test Matrix Summary                           ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	total := len(results)
	passed := 0
	failed := 0

	// Sort results by database type and version for better readability
	sortedResults := make([]testResult, len(results))
	copy(sortedResults, results)
	sort.Slice(sortedResults, func(i, j int) bool {
		if sortedResults[i].dbType != sortedResults[j].dbType {
			return sortedResults[i].dbType < sortedResults[j].dbType
		}
		return sortedResults[i].image < sortedResults[j].image
	})

	// Create table
	table := tablewriter.NewWriter(os.Stdout)
	table.Options(
		tablewriter.WithHeader([]string{"Database", "Version", "Status", "Duration"}),
	)

	// Add rows
	for _, result := range sortedResults {
		status := "✓ PASS"
		if !result.passed {
			status = "✗ FAIL"
			failed++
		} else {
			passed++
		}

		// Extract version from image (e.g., "mysql:8.0" -> "8.0")
		version := extractVersion(result.image)
		duration := formatDuration(result.duration)

		row := []string{
			result.dbType,
			version,
			status,
			duration,
		}
		table.Append(row)
	}

	// Add summary row
	table.Footer("", "", fmt.Sprintf("%d/%d", passed, total), "")
	table.Render()

	fmt.Println()

	// Print summary statistics
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("Total:  %d\n", total)
	if passed > 0 {
		fmt.Printf("\033[0;32mPassed: %d\033[0m\n", passed)
	}
	if failed > 0 {
		fmt.Printf("\033[0;31mFailed:  %d\033[0m\n", failed)
	}
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()

	if failed == 0 {
		fmt.Printf("\033[0;32m✓ All tests passed!\033[0m\n")
	} else {
		fmt.Printf("\033[0;31m✗ Some tests failed. Check logs in /tmp/testcontainers-*.log\033[0m\n")
	}
}

func extractVersion(image string) string {
	// For TiDB, the image is already just the version number
	// For MySQL/Percona/MariaDB, extract version from image string (e.g., "mysql:8.0" -> "8.0")
	parts := strings.Split(image, ":")
	if len(parts) > 1 {
		return parts[1]
	}
	return image
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "-"
	}
	if d < time.Second {
		return fmt.Sprintf("%.0fms", float64(d.Nanoseconds())/1e6)
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%ds", minutes, seconds)
}
