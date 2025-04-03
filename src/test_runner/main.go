package main

import (
	"encoding/json"
	"log"
	"os"
)

func main() {
	res, err := JsonTestRunner()
	if err != nil {
		log.Fatalf("Error: %v\n", err)
	}

	file, _ := json.MarshalIndent(res, "", " ")
	_ = os.WriteFile("/autograder/source/results.json", file, 0644)
}
