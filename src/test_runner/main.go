package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"
)

func main() {
	// Write output if the autograder crashes (probably due to OOM)
	var tempRes AutograderOutput
	tempRes.Tests = []TestResult{TestResult{Score: 0, MaxScore: 0, Name: "Autograder Crash", Number: "0", Output: "The autograder has crashed while running, likely due to running out of memory. Note that printed output is stored in-memory, so avoid printing large amounts of data such as values in the key-value database.", Visibility: "visible"}}
	file2, _ := json.MarshalIndent(tempRes, "", " ")
	_ = os.WriteFile(ResultsFile, file2, 0644)
	StartRamChecker()

	jsonConfig, err := GetJsonConfig()
	if err != nil {
		log.Fatalf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	missingFiles := FileChecker()
	submissionMetadata, submissionMetadataErr := GetSubmissionMetadata()
	if submissionMetadataErr != nil {
		log.Printf("Error getting submission history: %v\n", submissionMetadataErr)
	}

	var res AutograderOutput
	if len(missingFiles) > 0 {
		// Get max points
		maxPoints := 0.0
		for _, test := range jsonConfig.Tests {
			maxPoints += test.Points
		}

		failedTest := TestResult{Score: 0, MaxScore: maxPoints, Name: "Missing Files", Number: "0", Output: "Missing files in submission:\n" + strings.Join(missingFiles, "\n") + "\n\nAborted autograder run", Visibility: "visible"}
		res = AutograderOutput{Tests: []TestResult{failedTest}, Visibility: "visible"}
		log.Printf("Missing files: %v\n", missingFiles)
	} else {
		var err error
		res, err = JsonTestRunner(jsonConfig)
		if err != nil {
			log.Fatalf("Error: %v\n", err)
		}
		ApplyLatePenalty(&res, submissionMetadata)
		res.Output += "Please note: the automatically generated autograder score when you submit is not your final score. We will rerun the autograder once after submission closes on your active submission to determine your actual project score."
	}
	file, _ := json.MarshalIndent(res, "", " ")
	_ = os.WriteFile(ResultsFile, file, 0644)
	// Set file to nil and run GC to free up memory
	file = nil
	_ = ""
	runtime.GC()

	outputChanged := false
	ratelimitExceeded := false

	// Count number of submissions within last X hours as defined in the config
	if (submissionMetadataErr == nil && jsonConfig.Ratelimit.Count > 0) && (jsonConfig.Ratelimit.Minutes > 0) {
		count := 1
		thisSubmissionTime, err := time.Parse(time.RFC3339Nano, submissionMetadata.CreatedAt)
		if err != nil {
			log.Printf("Error parsing submission time: %v\n", err)
			return
		}
		for _, submission := range submissionMetadata.PreviousSubmissions {

			// Parse the submission time
			submissionTime, err := time.Parse(time.RFC3339Nano, submission.SubmissionTime)
			if err != nil {
				log.Printf("Error parsing submission time: %v\n", err)
				continue
			}

			// Check if submission is within the ratelimit window
			if thisSubmissionTime.Sub(submissionTime).Minutes() < float64(jsonConfig.Ratelimit.Minutes) {
				count++
			}
		}
		outputChanged = true
		if count > jsonConfig.Ratelimit.Count {
			ratelimitExceeded = true
			res.Output += fmt.Sprintf("Rate limit exceeded. You have submitted %d time(s) in the last %d minutes; not uploading log\n", count, jsonConfig.Ratelimit.Minutes)
			log.Printf("Rate limit exceeded: %d submission(s) in the last %d minutes.\n", count, jsonConfig.Ratelimit.Minutes)
		} else {
			res.Output += fmt.Sprintf("You have submitted %d time(s) in the last %d minutes.\n", count, jsonConfig.Ratelimit.Minutes)
			log.Printf("Rate limit count: %d submission(s) in the last %d minutes.\n", count, jsonConfig.Ratelimit.Minutes)
		}
	}
	if !ratelimitExceeded {
		if jsonConfig.Uploader != "" {
			switch jsonConfig.Uploader {
			case "bashupload.com":
				/*
					bashupload.com is no longer online.
					
					password, url, err := UploadLog("/autograder/results/results.json")
					if err == nil {
						log.Printf("Log uploaded successfully. URL: %s, Password: %s\n", url, password)
						res.Output += fmt.Sprintf("Log uploaded successfully. URL (stored for 3 days, max one download): %s\nPassword: %s\n", url, password)
					} else {
						res.Output += fmt.Sprintf("Log upload failed: %s\n", err.Error())
						log.Printf("Log upload failed: %v\n", err)
					}
				*/
				res.Output += "Log upload is deprecated and has been disabled.\n"
				log.Printf("Log upload requested with deprecated uploader: %s\n", jsonConfig.Uploader)
			default:
				log.Printf("Unknown uploader: %s\n", jsonConfig.Uploader)
			}
		} else {
			log.Printf("No uploader specified or error getting config: %v\n", err)
		}
	}

	if outputChanged {
		file, _ = json.MarshalIndent(res, "", " ")
		_ = os.WriteFile(ResultsFile, file, 0644)
	}

}
