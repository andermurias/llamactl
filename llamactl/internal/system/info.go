// Package system provides hardware resource information for Apple Silicon Macs.
package system

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Info holds system resource metrics.
type Info struct {
	MemTotal     uint64 `json:"mem_total"`     // bytes — from sysctl hw.memsize
	MemAvailable uint64 `json:"mem_available"` // bytes — free+inactive+speculative pages * 16384
	MemUsed      uint64 `json:"mem_used"`      // bytes
	Mem75Pct     uint64 `json:"mem_75pct"`     // 75% of total
	CPUCores     int    `json:"cpu_cores"`     // sysctl hw.physicalcpu
}

const pageSize = 16384 // macOS default page size (16 KB on Apple Silicon)

// Get returns current system memory and CPU info.
func Get() (*Info, error) {
	memTotal, err := sysctlUint64("hw.memsize")
	if err != nil {
		return nil, err
	}

	cpuCores, err := sysctlInt("hw.physicalcpu")
	if err != nil {
		return nil, err
	}

	freePages, inactivePages, speculativePages, err := vmStatPages()
	if err != nil {
		return nil, err
	}

	memAvail := (freePages + inactivePages + speculativePages) * pageSize
	if memAvail > memTotal {
		memAvail = memTotal
	}

	return &Info{
		MemTotal:     memTotal,
		MemAvailable: memAvail,
		MemUsed:      memTotal - memAvail,
		Mem75Pct:     memTotal * 3 / 4,
		CPUCores:     cpuCores,
	}, nil
}

func sysctlUint64(key string) (uint64, error) {
	out, err := exec.Command("/usr/sbin/sysctl", "-n", key).Output()
	if err != nil {
		return 0, fmt.Errorf("sysctl %s: %w", key, err)
	}
	v, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse sysctl %s: %w", key, err)
	}
	return v, nil
}

func sysctlInt(key string) (int, error) {
	out, err := exec.Command("/usr/sbin/sysctl", "-n", key).Output()
	if err != nil {
		return 0, fmt.Errorf("sysctl %s: %w", key, err)
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("parse sysctl %s: %w", key, err)
	}
	return v, nil
}

// vmStatPages parses `vm_stat` output and returns free, inactive, speculative
// page counts. Lines look like: "Pages free:                  462420."
func vmStatPages() (free, inactive, speculative uint64, err error) {
	out, err := exec.Command("/usr/bin/vm_stat").Output()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("vm_stat: %w", err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		valStr := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(parts[1]), "."))
		n, parseErr := strconv.ParseUint(valStr, 10, 64)
		if parseErr != nil {
			continue
		}
		switch key {
		case "Pages free":
			free = n
		case "Pages inactive":
			inactive = n
		case "Pages speculative":
			speculative = n
		}
	}
	return free, inactive, speculative, scanner.Err()
}
