package processinfo

import (
	"os"
	"strconv"
	"strings"
)

func ResidentMemory(pid int) *uint64 {
	if pid <= 0 {
		return nil
	}
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/statm")
	if err != nil {
		return nil
	}
	fields := strings.Fields(string(data))
	if len(fields) < 2 {
		return nil
	}
	pages, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return nil
	}
	bytes := pages * uint64(os.Getpagesize())
	return &bytes
}
