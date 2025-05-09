package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
}

func FileChecker() (missingFiles []string) {
	requiredFilesPath, err := filepath.Abs("../../required_files.txt")
	if err != nil {
		return nil
	}

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
		if _, err := os.Stat("/autograder/submission/" + scanner.Text()); os.IsNotExist(err) {
			missingFiles = append(missingFiles, scanner.Text())
		}
	}
	
	return missingFiles
}

func GetJsonConfig() (autograderConfig AutograderConfig, err error) {
	// Open the autograderconfig JSON file
	testConfigPath, err := filepath.Abs("../../autograder.config.json")
	if err != nil {
		return
	}

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


func JsonTestRunner() (result AutograderOutput, err error) {
	// Open the autograderconfig JSON file
	autograderConfig, err := GetJsonConfig()
	if err != nil {
		return
	}

	// Run all the tests within the submission folder
	wd, err := os.Getwd()
	if err != nil {
		return
	}

	// Change working directory to the student submission
	err = os.Chdir(fmt.Sprintf("%v/../../submission", wd))
	if err != nil {
		return
	}

	// Run each test individually
	for _, testConfig := range autograderConfig.Tests {
		fmt.Printf("[%s] Running test: %s\n", time.Now().Format(time.RFC3339), testConfig.Name)
		// Change working directory to the test folder if specified
		if testConfig.Folder != "" {
			err = os.Chdir(fmt.Sprintf("%v/../../submission/%s", wd, testConfig.Folder))
			if err != nil {
				fmt.Printf("Error changing directory to %s: %v\n", testConfig.Folder, err)
				return
			}
		} else {
			err = os.Chdir(fmt.Sprintf("%v/../../submission", wd))
			if err != nil {
				fmt.Printf("Error changing directory to submission: %v\n", err)
				return
			}
		}

		// Run go test with the specific test name
		args := []string{"-u", "student", "--", "go", "test", "-v", "-count=1"}
		if testConfig.Timeout != "" {
			args = append(args, "-timeout", testConfig.Timeout)
		}
		args = append(args, "-run", "^"+testConfig.Name+"$", "./...")
		
		// Initialize test result
		res := TestResult{
			Score:      testConfig.Points,
			MaxScore:   testConfig.Points,
			Name:       testConfig.Name,
			Number:     testConfig.Number,
			Visibility: testConfig.Visibility,
		}

		// Prepend folder name to the test name if specified
		if testConfig.Folder != "" {
			res.Name = fmt.Sprintf("%s/%s", testConfig.Folder, testConfig.Name)
		}
		
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
			out, err := cmd.CombinedOutput()
			
			// Check if the command failed and extract the exit code
			exitCode := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
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
