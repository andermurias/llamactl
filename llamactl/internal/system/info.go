// Package system provides hardware resource information for Apple Silicon Macs.
package system

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Info holds system resource metrics.
type Info struct {
	MemTotal     uint64  `json:"mem_total"`      // bytes — from sysctl hw.memsize
	MemAvailable uint64  `json:"mem_available"`  // bytes — free+inactive+speculative pages * 16384
	MemUsed      uint64  `json:"mem_used"`       // bytes
	Mem75Pct     uint64  `json:"mem_75pct"`      // 75% of total
	CPUCores     int     `json:"cpu_cores"`      // sysctl hw.physicalcpu
	CPULogical   int     `json:"cpu_logical"`    // sysctl hw.logicalcpu
	CPULoadAvg1  float64 `json:"cpu_load_avg_1"` // 1-minute load average
	ChipModel    string  `json:"chip_model"`     // e.g. "Apple M4"
	MacOSVersion string  `json:"macos_version"`  // e.g. "15.3.1"
	HWModel      string  `json:"hw_model"`       // e.g. "Mac16,10"
	DiskTotal    uint64  `json:"disk_total"`     // bytes — home filesystem
	DiskAvail    uint64  `json:"disk_avail"`     // bytes
	DiskUsed     uint64  `json:"disk_used"`      // bytes
	ModelsDirGB  float64 `json:"models_dir_gb"`  // ~/AI/models approximate size
	HFCacheGB    float64 `json:"hf_cache_gb"`    // ~/.cache/huggingface approximate size
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

	info := &Info{
		MemTotal:     memTotal,
		MemAvailable: memAvail,
		MemUsed:      memTotal - memAvail,
		Mem75Pct:     memTotal * 3 / 4,
		CPUCores:     cpuCores,
	}

	// Logical CPU count (best-effort)
	if n, err := sysctlInt("hw.logicalcpu"); err == nil {
		info.CPULogical = n
	}

	// Chip model, HW model, macOS version (best-effort)
	if chip, err := sysctlStr("machdep.cpu.brand_string"); err == nil {
		info.ChipModel = chip
	}
	if hwm, err := sysctlStr("hw.model"); err == nil {
		info.HWModel = hwm
	}
	if out, err := exec.Command("sw_vers", "-productVersion").Output(); err == nil {
		info.MacOSVersion = strings.TrimSpace(string(out))
	}

	// Load average (best-effort)
	if la, err := loadAvg1(); err == nil {
		info.CPULoadAvg1 = la
	}

	// Disk usage for home filesystem (best-effort)
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/"
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(home, &st); err == nil {
		info.DiskTotal = st.Blocks * uint64(st.Bsize)
		info.DiskAvail = st.Bavail * uint64(st.Bsize)
		info.DiskUsed = info.DiskTotal - st.Bfree*uint64(st.Bsize)
	}

	// Approximate sizes for models directories (best-effort, fast du)
	info.ModelsDirGB = dirGBFast(filepath.Join(home, "AI", "models"))
	info.HFCacheGB = dirGBFast(filepath.Join(home, ".cache", "huggingface"))

	return info, nil
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

func sysctlStr(key string) (string, error) {
	out, err := exec.Command("/usr/sbin/sysctl", "-n", key).Output()
	if err != nil {
		return "", fmt.Errorf("sysctl %s: %w", key, err)
	}
	return strings.TrimSpace(string(out)), nil
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

// loadAvg1 reads the 1-minute load average from sysctl vm.loadavg.
func loadAvg1() (float64, error) {
	// vm.loadavg output: "{ 1.23 2.34 3.45 }"
	out, err := exec.Command("/usr/sbin/sysctl", "-n", "vm.loadavg").Output()
	if err != nil {
		return 0, err
	}
	s := strings.Trim(strings.TrimSpace(string(out)), "{} ")
	parts := strings.Fields(s)
	if len(parts) < 1 {
		return 0, fmt.Errorf("unexpected vm.loadavg output")
	}
	return strconv.ParseFloat(parts[0], 64)
}

// dirGBFast returns the approximate size in GB of a directory using `du -sk`.
// Returns 0 on error or if the directory does not exist.
func dirGBFast(dir string) float64 {
	if _, err := os.Stat(dir); err != nil {
		return 0
	}
	out, err := exec.Command("/usr/bin/du", "-sk", dir).Output()
	if err != nil {
		return 0
	}
	parts := strings.Fields(string(out))
	if len(parts) < 1 {
		return 0
	}
	kb, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0
	}
	return kb / (1024 * 1024) // KB → GB
}