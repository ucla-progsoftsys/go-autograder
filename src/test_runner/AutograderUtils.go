package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

	// Initialize results map
	testResults := make(map[string]struct {
		Passed bool
		Output string
	})

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
		args := []string{"test", "-v"}
		if testConfig.Timeout != "" {
			args = append(args, "-timeout", testConfig.Timeout)
		}
		args = append(args, "-run", "^"+testConfig.Name+"$", "./...")
		// Print args
		cmd := exec.Command("go", args...)
		out, err := cmd.CombinedOutput()

		// Initialize exitCode to 0 (success)
		exitCode := 0

		// Check if the command failed and extract the exit code
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				// Command exited with a non-zero status
				exitCode = exitErr.ExitCode()
			} else {
				// Some other error occurred
				exitCode = 1
			}
		}
		// Parse the output for this specific test
		var singleTestResult struct {
			Passed bool
			Output string
		}
		if exitCode == 0 {
			singleTestResult.Passed = true
		} else {
			singleTestResult.Passed = false
		}
		singleTestResult.Output = string(out)
		
		testResults[testConfig.Name] = singleTestResult
		fmt.Printf("[%s] Finished running test: %s\n", time.Now().Format(time.RFC3339), testConfig.Name)
		fmt.Printf("[%s] Exit code: %d\n", time.Now().Format(time.RFC3339), exitCode)


	}

	// Generate autograder output from test results
	result.Visibility = autograderConfig.Visibility
	for _, testConfig := range autograderConfig.Tests {
		testRes, ok := testResults[testConfig.Name]
		if ok {
			res := TestResult{
				Score:      0,
				MaxScore:   testConfig.Points,
				Name:       testConfig.Name,
				Number:     testConfig.Number,
				Visibility: testConfig.Visibility,
			}

			if testRes.Passed {
				res.Score = testConfig.Points
			}
			res.Output = testRes.Output

			result.Tests = append(result.Tests, res)
		} else {
			res := TestResult{
				Score:      0,
				MaxScore:   testConfig.Points,
				Name:       testConfig.Name,
				Number:     testConfig.Number,
				Visibility: testConfig.Visibility,
				Output:     "This test failed to run on your submission. Make sure your submission is uploaded as-is, with files not inside of a folder, with the overall folder structure and location of code files unchanged, and no syntax/compiler errors on go version " + runtime.Version() + " (Ubuntu 22.04).",
			}

			result.Tests = append(result.Tests, res)
		}
	}

	return
}
