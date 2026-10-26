//go:build windows

package main

import (
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/sys/windows"
)

func alive(pid int) bool {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	ev, err := windows.WaitForSingleObject(h, 0)
	return err == nil && ev == uint32(windows.WAIT_TIMEOUT)
}

func killPID(pid int) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return windows.TerminateProcess(h, 1)
}

func elevated() (bool, string) {
	e := windows.GetCurrentProcessToken().IsElevated()
	return e, "token.IsElevated=" + strconv.FormatBool(e)
}

// rssKB sums working sets via PowerShell (wmic is removed from recent Windows 11).
func rssKB(pids []int) (int64, bool) {
	var total int64
	for _, p := range pids {
		out, err := exec.Command("powershell", "-NoProfile", "-Command", "(Get-Process -Id "+strconv.Itoa(p)+").WorkingSet64").Output()
		if err != nil {
			continue
		}
		if n, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64); err == nil {
			total += n / 1024
		}
	}
	return total, total > 0
}

func descendants(pid int) []int {
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		`Get-CimInstance Win32_Process | ForEach-Object { "$($_.ProcessId),$($_.ParentProcessId)" }`).Output()
	if err != nil {
		return nil
	}
	kids := map[int][]int{}
	for _, l := range strings.Split(string(out), "\n") {
		f := strings.Split(strings.TrimSpace(l), ",")
		if len(f) != 2 {
			continue
		}
		p, e1 := strconv.Atoi(f[0])
		pp, e2 := strconv.Atoi(f[1])
		if e1 == nil && e2 == nil {
			kids[pp] = append(kids[pp], p)
		}
	}
	var res []int
	var walk func(int)
	walk = func(p int) {
		for _, c := range kids[p] {
			res = append(res, c)
			walk(c)
		}
	}
	walk(pid)
	return res
}
