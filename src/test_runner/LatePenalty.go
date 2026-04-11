package main

import (
	"fmt"
	"log"
	"math"
	"time"
)

// ApplyLatePenalty applies an exponential deduction by late day:
// n days late (rounded up) => 2^(n-1)% deduction, capped at 100%.
func ApplyLatePenalty(res *AutograderOutput, history SubmissionMetadata) {
	lateDays, details, err := getLateDays(history)
	if err != nil {
		log.Printf("CAUTION: Late penalty calculator failed: %v", err)
		res.Output += "CAUTION: Late penalty calculator failed\n"
		return
	}

	if lateDays <= 0 {
		latePenaltyOutput := details + "No late penalty applied."
		res.Tests = append([]TestResult{TestResult{
			Score:      0,
			MaxScore:   0,
			Name:       "Late penalty",
			Number:     "0",
			Output:     latePenaltyOutput,
			Visibility: "visible",
		}}, res.Tests...)
		return
	}

	deductionPercent := math.Pow(2, float64(lateDays-1))
	if deductionPercent > 100 {
		deductionPercent = 100
	}

	totalScore := 0.0
	for i := range res.Tests {
		totalScore += res.Tests[i].Score
	}
	totalScore = roundTo(totalScore, 6)

	penaltyPoints := roundTo(totalScore*(deductionPercent/100), 6)
	if penaltyPoints < 0 {
		penaltyPoints = 0
	}
	penaltyLine := fmt.Sprintf("Applied %.2f%% late penalty to %.2f earned point(s): -%.2f point(s).", deductionPercent, totalScore, penaltyPoints)
	latePenaltyOutput := details + penaltyLine

	res.Tests = append([]TestResult{TestResult{
		Score:      -penaltyPoints,
		MaxScore:   0,
		Name:       "Late penalty",
		Number:     "0",
		Output:     latePenaltyOutput,
		Visibility: "visible",
	}}, res.Tests...)
}

func getLateDays(metadata SubmissionMetadata) (int, string, error) {
	if metadata.CreatedAt == "" {
		return 0, "", fmt.Errorf("missing submission created_at")
	}
	if metadata.Assignment.DueDate == "" {
		return 0, "", fmt.Errorf("missing assignment due_date")
	}

	dueDate, err := parseISODateTime(metadata.Assignment.DueDate)
	if err != nil {
		return 0, "", fmt.Errorf("invalid due_date: %w", err)
	}

	submissionDate, err := parseISODateTime(metadata.CreatedAt)
	if err != nil {
		return 0, "", fmt.Errorf("invalid created_at: %w", err)
	}

	details := ""
	if len(metadata.Users) > 0 && metadata.Users[0].Assignment != nil && metadata.Users[0].Assignment.DueDate != "" {
		userDueDate, err := parseISODateTime(metadata.Users[0].Assignment.DueDate)
		if err != nil {
			return 0, "", fmt.Errorf("invalid user-specific due_date: %w", err)
		}
		dueDate = userDueDate
		details += "Using user-specific due date\n"
	}

	details += fmt.Sprintf("Due Date: %s\n", dueDate.Format(time.RFC3339))
	details += fmt.Sprintf("Submission time: %s\n", submissionDate.Format(time.RFC3339))

	// Rounds timestamps down to nearest minute, e.g. 10:00:59pm becomes 10:00pm, on time if due at 10:00pm
	graceThreshold := dueDate.Add(time.Minute)

	lateDays := 0
	if submissionDate.Before(graceThreshold) {
		lateDays = 0
	} else {
		lateDays = int(submissionDate.Sub(graceThreshold).Hours()/24) + 1
	}

	if lateDays < 0 {
		lateDays = 0
	}
	details += fmt.Sprintf("Days late: %d\n", lateDays)

	return lateDays, details, nil
}

func parseISODateTime(raw string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999",
		"2006-01-02T15:04:05",
	}

	var parseErr error
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			return parsed, nil
		}
		parseErr = err
	}

	return time.Time{}, parseErr
}

func roundTo(value float64, places int) float64 {
	factor := math.Pow(10, float64(places))
	return math.Round(value*factor) / factor
}
