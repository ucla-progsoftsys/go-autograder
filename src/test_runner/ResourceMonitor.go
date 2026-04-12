package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// cpuTimes holds the raw jiffies read from /proc/stat.
type cpuTimes struct {
	user, nice, system, idle, iowait, irq, softirq, steal uint64
}

func (c cpuTimes) total() uint64 {
	return c.user + c.nice + c.system + c.idle + c.iowait + c.irq + c.softirq + c.steal
}

func (c cpuTimes) busy() uint64 {
	return c.total() - c.idle - c.iowait
}

// StartResourceMonitor prints the total available RAM and CPU count, then
// periodically logs current RAM and CPU usage every 15 seconds.
func StartResourceMonitor() {
	totalRAM, err := getTotalRAM()
	if err != nil {
		log.Printf("Resource Monitor: could not read total RAM: %v", err)
	} else {
		log.Printf("Resource Monitor: Total RAM: %d MB", totalRAM/1024)
	}

	numCPU, err := getNumCPUs()
	if err != nil {
		log.Printf("Resource Monitor: could not determine CPU count: %v", err)
	} else {
		log.Printf("Resource Monitor: Available CPU Threads: %d", numCPU)
	}

	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		prev, err := readCPUTimes()
		if err != nil {
			log.Printf("Resource Monitor: could not read initial CPU times: %v", err)
		}

		for {
			<-ticker.C
			printResourceUsage(&prev)
		}
	}()
}

// printResourceUsage logs the current RAM and CPU usage.
func printResourceUsage(prevCPU *cpuTimes) {
	// --- RAM ---
	usedKB, totalKB, err := getRAMUsage()
	if err != nil {
		log.Printf("Resource Monitor: could not read RAM usage: %v", err)
	} else {
		pct := 0.0
		if totalKB > 0 {
			pct = float64(usedKB) / float64(totalKB) * 100
		}
		log.Printf("Resource Monitor: RAM: %d / %d MB (%.1f%%)", usedKB/1024, totalKB/1024, pct)
	}

	// --- CPU ---
	cur, err := readCPUTimes()
	if err != nil {
		log.Printf("Resource Monitor: could not read CPU times: %v", err)
		return
	}
	if prevCPU != nil {
		dTotal := cur.total() - prevCPU.total()
		dBusy := cur.busy() - prevCPU.busy()
		cpuPct := 0.0
		if dTotal > 0 {
			cpuPct = float64(dBusy) / float64(dTotal) * 100
		}
		log.Printf("Resource Monitor: CPU: %.1f%%", cpuPct)
	}
	*prevCPU = cur
}

// ──────────────────────────────────────────────
// Helpers — all read from /proc so they work in
// Docker containers on Linux.
// ──────────────────────────────────────────────

// getTotalRAM returns total RAM in kB from /proc/meminfo.
func getTotalRAM() (uint64, error) {
	return readMeminfoField("MemTotal:")
}

// getRAMUsage returns (used kB, total kB).
func getRAMUsage() (uint64, uint64, error) {
	total, err := readMeminfoField("MemTotal:")
	if err != nil {
		return 0, 0, err
	}
	available, err := readMeminfoField("MemAvailable:")
	if err != nil {
		return 0, 0, err
	}
	return total - available, total, nil
}

// readMeminfoField reads a single kB field from /proc/meminfo.
func readMeminfoField(field string) (uint64, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, field) {
			parts := strings.Fields(line)
			if len(parts) < 2 {
				return 0, fmt.Errorf("unexpected format for %s", field)
			}
			val, err := strconv.ParseUint(parts[1], 10, 64)
			if err != nil {
				return 0, err
			}
			return val, nil
		}
	}
	return 0, fmt.Errorf("field %s not found in /proc/meminfo", field)
}

// readCPUTimes reads the aggregate "cpu" line from /proc/stat.
func readCPUTimes() (cpuTimes, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return cpuTimes{}, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			return parseCPULine(line)
		}
	}
	return cpuTimes{}, fmt.Errorf("cpu line not found in /proc/stat")
}

// parseCPULine parses a "cpu ..." line from /proc/stat.
func parseCPULine(line string) (cpuTimes, error) {
	fields := strings.Fields(line)
	if len(fields) < 9 {
		return cpuTimes{}, fmt.Errorf("unexpected /proc/stat format: %q", line)
	}

	vals := make([]uint64, 8)
	for i := 0; i < 8; i++ {
		v, err := strconv.ParseUint(fields[i+1], 10, 64)
		if err != nil {
			return cpuTimes{}, fmt.Errorf("parsing field %d: %w", i, err)
		}
		vals[i] = v
	}

	return cpuTimes{
		user:    vals[0],
		nice:    vals[1],
		system:  vals[2],
		idle:    vals[3],
		iowait:  vals[4],
		irq:     vals[5],
		softirq: vals[6],
		steal:   vals[7],
	}, nil
}

// getNumCPUs counts the "processor" lines in /proc/cpuinfo.
func getNumCPUs() (int, error) {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "processor") {
			count++
		}
	}
	if count == 0 {
		return 0, fmt.Errorf("no processors found in /proc/cpuinfo")
	}
	return count, nil
}
