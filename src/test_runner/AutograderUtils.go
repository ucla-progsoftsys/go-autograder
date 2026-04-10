package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// AutograderConfig is a struct that represents the parsed contents of autograder.config.json
type AutograderConfig struct {
	Visibility string `json:"visibility"`
	Tests      []struct {
		Name       string  `json:"name"`
		Number     string  `json:"number"`
		Points     float64 `json:"points"`
		Visibility string  `json:"visibility,omitempty"`
		Folder     string  `json:"folder,omitempty"`
		Timeout    string	`json:"timeout,omitempty"`
		Count	   int     `json:"count,omitempty"`
	} `json:"tests"`
	Uploader string `json:"uploader,omitempty"`
	Ratelimit struct {
		Count int `json:"count"`
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
	Output    string       `json:"output,omitempty"`
}

// SubmissionHistory represents the submission_history.json file
type SubmissionHistory struct {
	ID                 int                  `json:"id"`
	CreatedAt          string               `json:"created_at"`
	Assignment         Assignment           `json:"assignment"`
	SubmissionMethod   string               `json:"submission_method"`
	Users              []User               `json:"users"`
	PreviousSubmissions []PreviousSubmission `json:"previous_submissions"`
}

// Assignment represents the assignment details in submission_history.json
type Assignment struct {
	DueDate         string      `json:"due_date"`
	GroupSize       *int        `json:"group_size"` // Using pointer to handle null
	GroupSubmission bool        `json:"group_submission"`
	ID              int         `json:"id"`
	CourseID        int         `json:"course_id"`
	LateDueDate     *string     `json:"late_due_date"` // Using pointer to handle null
	ReleaseDate     string      `json:"release_date"`
	Title           string      `json:"title"`
	TotalPoints     string      `json:"total_points"`
}

// User represents a user in the submission_history.json
type User struct {
	Email string `json:"email"`
	ID    int    `json:"id"`
	Name  string `json:"name"`
}

// PreviousSubmission represents a previous submission in submission_history.json
type PreviousSubmission struct {
	SubmissionTime  string          `json:"submission_time"`
	ScoreString           string         `json:"score"` // For some reason, score is a string
	Score float64            `json:"score_as_integer,omitempty"`
	AutograderError bool            `json:"autograder_error"`
	Results         json.RawMessage `json:"results"` // Using RawMessage for the nested results object
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

func GetSubmissionHistory() (submissionHistory SubmissionHistory, err error) {
	// Open the submission history JSON file
	submissionHistoryPath := SubmissionMetadataFile

	file, err := os.ReadFile(submissionHistoryPath)
	if err != nil {
		return
	}

	// Parse the JSON into an array of testConfig structs
	err = json.Unmarshal(file, &submissionHistory)
	if err != nil {
		return
	}
	// Convert string scores to float values
	for i := range submissionHistory.PreviousSubmissions {
		submissionHistory.PreviousSubmissions[i].Score, _ = strconv.ParseFloat(submissionHistory.PreviousSubmissions[i].ScoreString, 64)
	}

	return
}


func JsonTestRunner(autograderConfig AutograderConfig) (result AutograderOutput, err error) {
	// Run all the tests within the submission folder

	// Change working directory to the student submission
	err = os.Chdir(SubmissionDir)
	if err != nil {
		return
	}

	compiledBinaries := make(map[string]bool)
	compilationErrors := make(map[string]string)

	// Run each test individually
	for _, testConfig := range autograderConfig.Tests {
		fmt.Printf("[%s] Running test: %s\n", time.Now().Format(time.RFC3339), testConfig.Name)
		
		folder := testConfig.Folder
		absFolder := SubmissionDir
		if folder != "" {
			absFolder = filepath.Join(SubmissionDir, folder)
		}

		// Change working directory to the test folder
		err = os.Chdir(absFolder)
		if err != nil {
			fmt.Printf("Error changing directory to %s: %v\n", absFolder, err)
			
			res := TestResult{
				Score:      0,
				MaxScore:   testConfig.Points,
				Name:       testConfig.Name,
				Number:     testConfig.Number,
				Visibility: testConfig.Visibility,
				Output:     fmt.Sprintf("Error: could not find folder %s\n", folder),
			}
			if testConfig.Folder != "" {
				res.Name = fmt.Sprintf("%s/%s", testConfig.Folder, testConfig.Name)
			}
			result.Tests = append(result.Tests, res)
			continue
		}

		// Compile test binary if not already done for this folder
		if !compiledBinaries[folder] && compilationErrors[folder] == "" {
			fmt.Printf("[%s] Compiling tests in folder: %s\n", time.Now().Format(time.RFC3339), folder)
			
			// Remove old binary if it exists
			os.Remove("tests.test")

			cmd := exec.Command("runuser", "-u", "student", "--", "go", "test", "-c", "-o", "tests.test")
			out, compileErr := cmd.CombinedOutput()
			if compileErr != nil {
				compilationErrors[folder] = string(out)
				fmt.Printf("[%s] Compilation failed for folder %s\n", time.Now().Format(time.RFC3339), folder)
			} else {
				compiledBinaries[folder] = true
				fmt.Printf("[%s] Compilation successful for folder %s\n", time.Now().Format(time.RFC3339), folder)
			}
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

		if compilationErrors[folder] != "" {
			res.Score = 0
			res.Output += "\nCompilation failed:\n" + compilationErrors[folder]
			result.Tests = append(result.Tests, res)
			continue
		}

		// Prepare args for running the compiled binary
		args := []string{"-u", "student", "--", "./tests.test", "-test.v", "-test.count=1"}
		if testConfig.Timeout != "" {
			args = append(args, "-test.timeout", testConfig.Timeout)
		}
		args = append(args, "-test.run", "^"+testConfig.Name+"$")
		
		// Check if we need to run this test multiple times
		runCount := 1
		if testConfig.Count > 0 {
			runCount = testConfig.Count
		}
		
		// Run the test the specified number of times
		for i := 0; i < runCount; i++ {
			if runCount > 1 {
				fmt.Printf("[%s] Running %s (iteration %d/%d)\n", time.Now().Format(time.RFC3339), testConfig.Name, i+1, runCount)
			}
			
			cmd := exec.Command("runuser", args...)
			out, execErr := cmd.CombinedOutput()
			
			// Check if the command failed and extract the exit code
			exitCode := 0
			if execErr != nil {
				if exitErr, ok := execErr.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else {
					exitCode = 1
				}
			}
			
			if runCount > 1 {
				res.Output += fmt.Sprintf("\n\n--- Iteration %d/%d ---\n", i+1, runCount)
			}
			res.Output += string(out)
			
			// If any iteration fails, the entire test fails
			if exitCode != 0 {
				res.Score = 0
				if runCount > 1 {
					fmt.Printf("[%s] Test %s failed on iteration %d/%d\n", time.Now().Format(time.RFC3339), testConfig.Name, i+1, runCount)
				}
			}
		}
		
		// Add summary for multiple iterations
		if runCount > 1 {
			if res.Score != 0 {
				fmt.Printf("[%s] All %d iterations of test %s passed\n", time.Now().Format(time.RFC3339), runCount, testConfig.Name)
				res.Output += fmt.Sprintf("\n\n--- Summary ---\nAll %d iterations passed.\n", runCount)
			} else {
				fmt.Printf("[%s] Test %s failed (at least one of %d iterations failed)\n", time.Now().Format(time.RFC3339), testConfig.Name, runCount)
				res.Output += fmt.Sprintf("\n\n--- Summary ---\nAt least one of the %d iterations failed.\n", runCount)
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
