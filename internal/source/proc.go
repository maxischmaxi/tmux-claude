package source

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// procRoot is overridable in tests.
var procRoot = "/proc"

func hasProc() bool {
	_, err := os.Stat(procRoot + "/self")
	return err == nil
}

// procStat returns comm, ppid and starttime (field 22) of a process.
// Without /proc (macOS) it falls back to ps and returns no start time.
func procStat(pid int) (comm string, ppid int, start string, ok bool) {
	if !hasProc() {
		return psStat(pid)
	}
	b, err := os.ReadFile(procRoot + "/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return "", 0, "", false
	}
	s := string(b)
	// comm is wrapped in parens and may contain spaces
	l, r := strings.IndexByte(s, '('), strings.LastIndexByte(s, ')')
	if l < 0 || r < l {
		return "", 0, "", false
	}
	comm = s[l+1 : r]
	f := strings.Fields(s[r+1:])
	// f[0] is field 3 (state), so field n is f[n-3]
	if len(f) < 20 {
		return "", 0, "", false
	}
	ppid, _ = strconv.Atoi(f[1])
	return comm, ppid, f[19], true
}

func psStat(pid int) (string, int, string, bool) {
	out, err := exec.Command("ps", "-o", "ppid=,comm=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return "", 0, "", false
	}
	f := strings.Fields(string(out))
	if len(f) < 2 {
		return "", 0, "", false
	}
	ppid, _ := strconv.Atoi(f[0])
	return filepath.Base(strings.Join(f[1:], " ")), ppid, "", true
}

// Alive reports whether pid runs and, if procStart is known, is the same process.
func Alive(pid int, procStart string) bool {
	if pid <= 0 {
		return false
	}
	if !hasProc() {
		err := syscall.Kill(pid, 0)
		return err == nil || err == syscall.EPERM
	}
	_, _, start, ok := procStat(pid)
	if !ok {
		return false
	}
	return procStart == "" || procStart == start
}

// FindClaudeAncestor walks up from pid to the nearest process named "claude".
func FindClaudeAncestor(pid int) (int, string) {
	for i := 0; i < 16 && pid > 1; i++ {
		comm, ppid, start, ok := procStat(pid)
		if !ok {
			return 0, ""
		}
		if comm == "claude" {
			return pid, start
		}
		pid = ppid
	}
	return 0, ""
}
