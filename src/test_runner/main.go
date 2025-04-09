package main

import (
	"encoding/json"
	"log"
	"os"
	"strings"
)

func main() {
	missingFiles := FileChecker()
	var res AutograderOutput;
	if len(missingFiles) > 0 {
		// Get max points
		maxPoints := 0.0
		jsonConfig, err := GetJsonConfig()
		if err == nil {
			for _, test := range jsonConfig.Tests {
				maxPoints += test.Points
			}
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
}
