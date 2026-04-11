package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

type TestConfig struct {
	Name          string  `json:"name"`
	Number        string  `json:"number"`
	Points        float64 `json:"points"`
	Visibility    string  `json:"visibility,omitempty"`
	Folder        string  `json:"folder,omitempty"`
	Timeout       string  `json:"timeout,omitempty"`
	Count         int     `json:"count,omitempty"`
	ParallelCount int     `json:"parallelCount,omitempty"`
	Race          bool    `json:"race,omitempty"`
}

// AutograderConfig is a struct that represents the parsed contents of autograder.config.json
type AutograderConfig struct {
	Visibility string       `json:"visibility"`
	Tests      []TestConfig `json:"tests"`
	Uploader   string       `json:"uploader,omitempty"`
	Ratelimit  struct {
		Count   int `json:"count"`
		Minutes int `json:"minutes"`
	} `json:"ratelimit,omitempty"`
}

// TestResult is a struct that represents the result of a test case in Gradescope's specifications
// https://gradescope-autograders.readthedocs.io/en/latest/specs/
type TestResult struct {
	Score      float64 `json:"score"`
	MaxScore   float64 `json:"max_score"`
	Name       string  `json:"name"`
	Number     string  `json:"number"`
	Output     string  `json:"output"`
	Visibility string  `json:"visibility,omitempty"`
}

// AutograderOutput represents the output that conforms to Gradescope's specifications
// https://gradescope-autograders.readthedocs.io/en/latest/specs/
type AutograderOutput struct {
	Visibility string       `json:"visibility,omitempty"`
	Tests      []TestResult `json:"tests"`
	Output     string       `json:"output,omitempty"`
}

// SubmissionMetadata represents the submission_metadata.json file
type SubmissionMetadata struct {
	ID                  int                  `json:"id"`
	CreatedAt           string               `json:"created_at"`
	Assignment          Assignment           `json:"assignment"`
	SubmissionMethod    string               `json:"submission_method"`
	Users               []User               `json:"users"`
	PreviousSubmissions []PreviousSubmission `json:"previous_submissions"`
}

// Assignment represents the assignment details in submission_history.json
type Assignment struct {
	DueDate         string  `json:"due_date"`
	GroupSize       *int    `json:"group_size"` // Using pointer to handle null
	GroupSubmission bool    `json:"group_submission"`
	ID              int     `json:"id"`
	CourseID        int     `json:"course_id"`
	LateDueDate     *string `json:"late_due_date"` // Using pointer to handle null
	ReleaseDate     string  `json:"release_date"`
	Title           string  `json:"title"`
	TotalPoints     string  `json:"total_points"`
}

// User represents a user in the submission_history.json
type User struct {
	Email      string      `json:"email"`
	ID         int         `json:"id"`
	Name       string      `json:"name"`
	Assignment *Assignment `json:"assignment,omitempty"`
}

// PreviousSubmission represents a previous submission in submission_history.json
type PreviousSubmission struct {
	SubmissionTime  string          `json:"submission_time"`
	ScoreString     string          `json:"score"` // For some reason, score is a string
	Score           float64         `json:"score_as_integer,omitempty"`
	AutograderError bool            `json:"autograder_error"`
	Results         json.RawMessage `json:"results"` // Using RawMessage for the nested results object
}

type testRunResult struct {
	output   string
	exitCode int
}

func truncateMiddleBytesIfTooLong(output []byte) string {
	const maxBytes = 200000 // Should be long enough that this truncation message is never seen in gradescope
	const keepBytes = 100000

	if len(output) <= maxBytes {
		return string(output)
	}

	removedBytes := len(output) - maxBytes
	marker := []byte(fmt.Sprintf("\n<truncated %d bytes>\n", removedBytes))

	truncated := make([]byte, 0, maxBytes+len(marker))
	truncated = append(truncated, output[:keepBytes]...)
	truncated = append(truncated, marker...)
	truncated = append(truncated, output[len(output)-keepBytes:]...)
	return string(truncated)
}

func FileChecker() (missingFiles []string) {
	requiredFilesPath := RequiredFilesFile

	file, err := os.Open(requiredFilesPath)
	if err != nil {
		return nil
	}
	defer file.Close()

	// Parse the file into an array of strings
	// One line per string
	missingFiles = make([]string, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		// Check if the file exists
		if _, err := os.Stat(filepath.Join(SubmissionDir, scanner.Text())); os.IsNotExist(err) {
			missingFiles = append(missingFiles, scanner.Text())
		}
	}

	return missingFiles
}

func GetJsonConfig() (autograderConfig AutograderConfig, err error) {
	// Open the autograder config JSON file
	testConfigPath := ConfigFile

	file, err := os.ReadFile(testConfigPath)
	if err != nil {
		return
	}

	// Parse the JSON into an array of testConfig structs
	err = json.Unmarshal(file, &autograderConfig)
	if err != nil {
		return
	}

	return
}

func GetSubmissionMetadata() (submissionMetadata SubmissionMetadata, err error) {
	// Open the submission metadata JSON file
	submissionMetadataPath := SubmissionMetadataFile

	file, err := os.ReadFile(submissionMetadataPath)
	if err != nil {
		return
	}

	// Parse the JSON into an array of testConfig structs
	err = json.Unmarshal(file, &submissionMetadata)
	if err != nil {
		return
	}
	// Convert string scores to float values
	for i := range submissionMetadata.PreviousSubmissions {
		submissionMetadata.PreviousSubmissions[i].Score, _ = strconv.ParseFloat(submissionMetadata.PreviousSubmissions[i].ScoreString, 64)
	}

	return
}

func buildGoTestArgs(testConfig TestConfig) []string {
	args := []string{"-u", "student", "--", "go", "test", "-v", "-count=1"}
	if testConfig.Race {
		args = append(args, "-race")
	}
	if testConfig.Timeout != "" {
		args = append(args, "-timeout", testConfig.Timeout)
	}
	return append(args, "-run", "^"+testConfig.Name+"$", ".")
}

func runGoTest(testDir string, testConfig TestConfig) testRunResult {
	cmd := exec.Command("runuser", buildGoTestArgs(testConfig)...)
	cmd.Dir = testDir
	out, err := cmd.CombinedOutput()
	result := testRunResult{output: truncateMiddleBytesIfTooLong(out)}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.exitCode = exitErr.ExitCode()
		} else {
			result.exitCode = 1
		}
	}
	return result
}

func JsonTestRunner(autograderConfig AutograderConfig) (result AutograderOutput, err error) {
	// Run all the tests within the submission folder

	// Run each test individually
	for _, testConfig := range autograderConfig.Tests {
		testConfig := testConfig
		fmt.Printf("[%s] Running test: %s\n", time.Now().Format(time.RFC3339), testConfig.Name)
		testDir := SubmissionDir
		if testConfig.Folder != "" {
			testDir = filepath.Join(SubmissionDir, testConfig.Folder)
		}
		dirInfo, statErr := os.Stat(testDir)
		if statErr != nil {
			err = fmt.Errorf("invalid test folder for test %q: %s: %w", testConfig.Name, testDir, statErr)
			return
		}
		if !dirInfo.IsDir() {
			err = fmt.Errorf("invalid test folder for test %q: %s is not a directory", testConfig.Name, testDir)
			return
		}

		// Initialize test result
		res := TestResult{
			Score:      testConfig.Points,
			MaxScore:   testConfig.Points,
			Name:       testConfig.Name,
			Number:     testConfig.Number,
			Visibility: testConfig.Visibility,
		}

		if testConfig.Timeout != "" {
			res.Output = fmt.Sprintf("Timeout: %s\n", testConfig.Timeout)
		}

		// Prepend folder name to the test name if specified
		if testConfig.Folder != "" {
			res.Name = fmt.Sprintf("%s/%s", testConfig.Folder, testConfig.Name)
		}

		// min/max required go 1.21 or later, so doing this manually to remain backward compatible
		runCount := 1
		if testConfig.Count > 0 {
			runCount = testConfig.Count
		}
		parallelCount := 1
		if testConfig.ParallelCount > 1 {
			parallelCount = testConfig.ParallelCount
		}
		if parallelCount > runCount {
			parallelCount = runCount
		}
		if parallelCount > 1 {
			res.Output += fmt.Sprintf("Running tests in parallel with %d workers.\n", parallelCount)
		}
		runResults := make([]testRunResult, runCount)
		failureCount := 0

		var wg sync.WaitGroup
		var failureMu sync.Mutex
		sem := make(chan struct{}, parallelCount)
		for i := 0; i < runCount; i++ {
			sem <- struct{}{}
			if runCount > 1 {
				fmt.Printf("[%s] Running %s (iteration %d/%d)\n", time.Now().Format(time.RFC3339), testConfig.Name, i+1, runCount)
			}
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				defer func() { <-sem }()
				runResults[i] = runGoTest(testDir, testConfig)
				if runResults[i].exitCode != 0 {
					failureMu.Lock()
					failureCount++
					failureMu.Unlock()
					if runCount > 1 {
						fmt.Printf("[%s] Test %s failed on iteration %d/%d\n", time.Now().Format(time.RFC3339), testConfig.Name, i+1, runCount)
					}
				} else if runCount > 1 {
					fmt.Printf("[%s] Test %s passed iteration %d/%d\n", time.Now().Format(time.RFC3339), testConfig.Name, i+1, runCount)
				}
			}(i)
		}
		wg.Wait()

		for i, runResult := range runResults {
			if runCount > 1 {
				res.Output += fmt.Sprintf("\n\n--- Iteration %d/%d ---\n", i+1, runCount)
			}
			res.Output += runResult.output
		}

		if failureCount > 0 {
			res.Score = 0
		}

		if runCount > 1 {
			if failureCount == 0 {
				fmt.Printf("[%s] All %d iterations of test %s passed\n", time.Now().Format(time.RFC3339), runCount, testConfig.Name)
				res.Output += fmt.Sprintf("\n\n--- Summary ---\nAll %d iterations passed.\n", runCount)
			} else {
				fmt.Printf("[%s] Test %s failed (%d/%d iterations failed)\n", time.Now().Format(time.RFC3339), testConfig.Name, failureCount, runCount)
				res.Output += fmt.Sprintf("\n\n--- Summary ---\n%d/%d iterations failed.\n", failureCount, runCount)
			}
		}
		if res.Score == 0 {
			fmt.Printf("[%s] Test failed: %s\n", time.Now().Format(time.RFC3339), testConfig.Name)
		} else {
			fmt.Printf("[%s] Test passed: %s\n", time.Now().Format(time.RFC3339), testConfig.Name)
		}

		result.Tests = append(result.Tests, res)
	}

	// Generate autograder output from test results
	result.Visibility = autograderConfig.Visibility

	return
}
