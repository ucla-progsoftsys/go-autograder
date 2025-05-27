package main

import (
	"log"
	"runtime"
	"time"
)

// StartRamChecker prints out the current RAM usage and max of the system every 15 seconds
func StartRamChecker() {
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			printMemStats()
			<-ticker.C
		}
	}()
}

func printMemStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	// Convert bytes to MB for readability
	allocatedMB := float64(m.Alloc) / 1024 / 1024
	systemMB := float64(m.Sys) / 1024 / 1024
	
	log.Printf("RAM Usage: %.2f MB (allocated) / %.2f MB (system)", allocatedMB, systemMB)
}