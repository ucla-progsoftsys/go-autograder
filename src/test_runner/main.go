package main

import (
	"encoding/json"
	"log"
	"os"
	"strings"
	"runtime"
)

func main() {
	missingFiles := FileChecker()
	jsonConfig, err := GetJsonConfig()
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
		file, _ = json.MarshalIndent(res, "", " ")
		_ = os.WriteFile("/autograder/results/results.json", file, 0644)
	} else {
		log.Printf("No uploader specified or error getting config: %v\n", err)
	}

}
