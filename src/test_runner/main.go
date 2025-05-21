package main

import (
	"encoding/json"
	"log"
	"os"
	"runtime"
	"time"
	"fmt"
	"strings"
)

func main() {
	missingFiles := FileChecker()
	jsonConfig, err := GetJsonConfig()
	submissionHistory, submissionHistoryErr := GetSubmissionHistory()
	if submissionHistoryErr != nil {
		log.Printf("Error getting submission history: %v\n", submissionHistoryErr)
	}
	if err != nil {
		log.Fatalf("Error: %v\n", err)
		os.Exit(1)
	}
	var res AutograderOutput;
	if len(missingFiles) > 0 {
		// Get max points
		maxPoints := 0.0
		for _, test := range jsonConfig.Tests {
			maxPoints += test.Points
		}
		
		failedTest := TestResult{ Score: 0, MaxScore: maxPoints, Name: "Missing Files", Number: "0", Output: "Missing files in submission:\n" + strings.Join(missingFiles, "\n") + "\n\nAborted autograder run", Visibility: "visible" }
		res = AutograderOutput{ Tests: []TestResult{failedTest}, Visibility: "visible" }
		log.Printf("Missing files: %v\n", missingFiles)
	} else {
		var err error;
		res, err = JsonTestRunner()
		if err != nil {
			log.Fatalf("Error: %v\n", err)
		}
	
	}
	file, _ := json.MarshalIndent(res, "", " ")
	_ = os.WriteFile("/autograder/results/results.json", file, 0644)
	// Set file to nil and run GC to free up memory
	file = nil
	_ = ""
	runtime.GC()

	outputChanged := false
	ratelimitExceeded := false


	// Count number of submissions within last X hours as defined in the config
	if (submissionHistoryErr == nil && jsonConfig.Ratelimit.Count > 0) && (jsonConfig.Ratelimit.Minutes > 0) {
		count := 0
		thisSubmissionTime, err := time.Parse(time.RFC3339Nano, submissionHistory.CreatedAt)
		if err != nil {
			log.Printf("Error parsing submission time: %v\n", err)
			return
		}
		for _, submission := range submissionHistory.PreviousSubmissions {

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
		if count >= jsonConfig.Ratelimit.Count {
			ratelimitExceeded = true
			res.Output = fmt.Sprintf("Rate limit exceeded. You have submitted %d time(s) in the last %d minutes; not uploading log\n", count, jsonConfig.Ratelimit.Minutes)
			log.Printf("Rate limit exceeded: %d submission(s) in the last %d minutes.\n", count, jsonConfig.Ratelimit.Minutes)
		} else {
			res.Output = fmt.Sprintf("You have submitted %d time(s) in the last %d minutes.\n", count, jsonConfig.Ratelimit.Minutes)
			log.Printf("Rate limit count: %d submission(s) in the last %d minutes.\n", count, jsonConfig.Ratelimit.Minutes)
		}
	}
	if !ratelimitExceeded {
		if jsonConfig.Uploader != "" {
			switch jsonConfig.Uploader {
				case "bashupload.com":
					password, url, err := UploadLog("/autograder/results/results.json")
					if err == nil {
						log.Printf("Log uploaded successfully. URL: %s, Password: %s\n", url, password)
						res.Output = "Log uploaded successfully. URL (stored for 3 days, max one download): " + url + "\nPassword: " + password
					} else {
						res.Output = "Log upload failed: " + err.Error()
						log.Printf("Log upload failed: %v\n", err)
					}
				default:
					log.Printf("Unknown uploader: %s\n", jsonConfig.Uploader)
			}
		} else {
			log.Printf("No uploader specified or error getting config: %v\n", err)
		}
	}

	if outputChanged {
		file, _ = json.MarshalIndent(res, "", " ")
		_ = os.WriteFile("/autograder/results/results.json", file, 0644)
	}

}
