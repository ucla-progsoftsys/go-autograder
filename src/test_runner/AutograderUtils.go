package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
	AllOrNothing  bool    `json:"allOrNothing,omitempty"`
	DisplayName   string  `json:"displayName,omitempty"`
}

// AutograderConfig is a struct that represents the parsed contents of autograder.config.json
type AutograderConfig struct {
	Visibility   string       `json:"visibility"`
	Tests        []TestConfig `json:"tests"`
	Uploader     string       `json:"uploader,omitempty"`
	ScoreMessage string       `json:"score_message,omitempty"`
	Ratelimit    struct {
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

// Assignment represents the assignment details in submission_metadata.json
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

// User represents a user in submission_metadata.json
type User struct {
	Email      string      `json:"email"`
	ID         int         `json:"id"`
	Name       string      `json:"name"`
	Assignment *Assignment `json:"assignment,omitempty"`
}

// PreviousSubmission represents a previous submission in submission_metadata.json
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
		return flattenToASCII(output)
	}

	removedBytes := len(output) - maxBytes
	marker := []byte(fmt.Sprintf("\n<truncated %d bytes>\n", removedBytes))

	truncated := make([]byte, 0, maxBytes+len(marker))
	truncated = append(truncated, output[:keepBytes]...)
	truncated = append(truncated, marker...)
	truncated = append(truncated, output[len(output)-keepBytes:]...)
	return flattenToASCII(truncated)
}

// flattenToASCII strips all non-ASCII bytes and returns the result as a string.
func flattenToASCII(b []byte) string {
	out := make([]byte, 0, len(b))
	for _, c := range b {
		if c <= 127 {
			out = append(out, c)
		}
	}
	return string(out)
}

// calculateExponentialScore computes partial credit using smooth exponential decay.
// The formula is: points × 0.5^((1 - passRate) / 0.1)
// Anchor points: 100% passed = 100%, 90% = 50%, 80% = 25%, 70% = 12.5%, <70% = 0%.
// Values between anchor points are smoothly interpolated (e.g. 95% ≈ 70.7%).
func calculateExponentialScore(maxPoints float64, passCount, totalCount int) float64 {
	if totalCount <= 0 {
		return 0
	}
	passRate := float64(passCount) / float64(totalCount)
	if passRate < 0.6999 { // Limit floating point math errors to allow 70% exactly to still get points
		return 0
	}
	if passRate >= 1.0 {
		return maxPoints
	}
	// Smooth exponential: halves for every 10% drop from 100%
	return maxPoints * math.Pow(0.5, (1.0-passRate)/0.1)
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
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Check if the file exists
		if _, err := os.Stat(filepath.Join(SubmissionDir, line)); os.IsNotExist(err) {
			missingFiles = append(missingFiles, line)
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

	// Parse the JSON into a SubmissionMetadata struct
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

		// If race is enabled, add `-race`
		if testConfig.Race {
			res.Name += " -race"
		}

		// Override display name if specified
		if testConfig.DisplayName != "" {
			res.Name = testConfig.DisplayName
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

		passCount := runCount - failureCount

		// Scoring: exponential decay by default when count > 1, unless allOrNothing is set
		if failureCount > 0 {
			if runCount > 1 && !testConfig.AllOrNothing {
				res.Score = calculateExponentialScore(testConfig.Points, passCount, runCount)
			} else {
				res.Score = 0
			}
		}

		if runCount > 1 {
			passRate := float64(passCount) / float64(runCount) * 100
			if failureCount == 0 {
				fmt.Printf("[%s] All %d iterations of test %s passed\n", time.Now().Format(time.RFC3339), runCount, testConfig.Name)
				res.Output += fmt.Sprintf("\n\n--- Summary ---\nAll %d iterations passed.\n", runCount)
			} else {
				fmt.Printf("[%s] Test %s failed (%d/%d iterations failed)\n", time.Now().Format(time.RFC3339), testConfig.Name, failureCount, runCount)
				res.Output += fmt.Sprintf("\n\n--- Summary ---\n%d/%d iterations passed (%.0f%% pass rate).\n", passCount, runCount, passRate)
				if !testConfig.AllOrNothing {
					res.Output += fmt.Sprintf("Exponential scoring: %.2f/%.2f points awarded.\n", res.Score, testConfig.Points)
				} else {
					res.Output += "All-or-nothing scoring: 0 points awarded due to failure(s).\n"
				}
			}
		}
		if res.Score == 0 {
			fmt.Printf("[%s] Test failed: %s\n", time.Now().Format(time.RFC3339), testConfig.Name)
		} else if res.Score < testConfig.Points {
			fmt.Printf("[%s] Test partial: %s (%.2f/%.2f)\n", time.Now().Format(time.RFC3339), testConfig.Name, res.Score, testConfig.Points)
		} else {
			fmt.Printf("[%s] Test passed: %s\n", time.Now().Format(time.RFC3339), testConfig.Name)
		}

		result.Tests = append(result.Tests, res)
	}

	// Generate autograder output from test results
	result.Visibility = autograderConfig.Visibility

	return
}
