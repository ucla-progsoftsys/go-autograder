package main

import (
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)


// UploadLog compresses, encrypts and uploads log data to bashupload.com
// Takes a file path to the log file rather than the actual data
// Returns the encryption password and the URL where the file was uploaded
func UploadLog(filePath string) (string, string, error) {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", "", fmt.Errorf("file does not exist: %v", err)
	}

	// Check if file is less than 32GB
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return "", "", fmt.Errorf("failed to get file info: %v", err)
	}
	if fileInfo.Size() > 1<<30 * 32 { // 32GB
		return "", "", fmt.Errorf("file is larger than 32 GB")
	}
	
	// Generate random password (32 alphanumeric characters)
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	password := ""
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", fmt.Errorf("failed to generate password: %v", err)
	}
	for i := 0; i < 32; i++ {
		password += string(chars[randomBytes[i]%byte(len(chars))])
	}
	
	// Create temporary directory for the zip file
	tempDir, err := os.MkdirTemp("", "logupload")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create zip file with password
	zipPath := filepath.Join(tempDir, "log_data.zip")
	zipCmd := exec.Command("zip", "-j", "-P", password, zipPath, filePath)
	if err := zipCmd.Run(); err != nil {
		return "", "", fmt.Errorf("failed to zip data: %v", err)
	}
	
	// Check if zip file is less than 0.1GB
	fileInfo, err = os.Stat(zipPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to get file info: %v", err)
	}
	if fileInfo.Size() > 1<<30 / 100 { // 0.1GB
		return "", "", fmt.Errorf("compressed file is larger than 0.1 GB")
	}

	// Upload to bashupload.com
	curlCmd := exec.Command("curl", "-s", "--data-binary", "@"+zipPath, "https://bashupload.com/results.zip")
	output, err := curlCmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to upload data: %v", err)
	}
	
	// Extract URL from curl output
	outputStr := string(output)
	urlLine := ""
	
	lines := strings.Split(outputStr, "\n")
	for _, line := range lines {
		if strings.Contains(line, "wget") || strings.Contains(line, "curl") {
			urlLine = line
			break
		}
	}
	
	if urlLine == "" {
		return "", "", fmt.Errorf("failed to extract URL from response")
	}
	
	// Extract actual URL from response line
	parts := strings.Fields(urlLine)
	var url string
	for _, part := range parts {
		if strings.HasPrefix(part, "https://bashupload.com/") {
			url = part
			break
		}
	}
	
	if url == "" {
		return "", "", fmt.Errorf("URL not found in response")
	}
	
	return password, url, nil
}
