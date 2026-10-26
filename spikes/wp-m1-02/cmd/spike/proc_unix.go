//go:build !windows

package main

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func killPID(pid int) error { return syscall.Kill(pid, syscall.SIGKILL) }

// elevated reports whether the process runs with administrator rights.
func elevated() (bool, string) {
	return syscall.Geteuid() == 0, "euid=" + strconv.Itoa(syscall.Geteuid())
}

// rssKB sums resident memory of pids with ps (Linux and macOS).
func rssKB(pids []int) (int64, bool) {
	var total int64
	for _, p := range pids {
		out, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(p)).Output()
		if err != nil {
			continue
		}
		n, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
		if err == nil {
			total += n
		}
	}
	return total, total > 0
}

// children lists descendant pids of pid using ps.
func descendants(pid int) []int {
	out, err := exec.Command("ps", "-A", "-o", "pid=,ppid=").Output()
	if err != nil {
		return nil
	}
	kids := map[int][]int{}
	for _, l := range strings.Split(string(out), "\n") {
		f := strings.Fields(l)
		if len(f) != 2 {
			continue
		}
		a, _ := strconv.Atoi(f[0])
		b, _ := strconv.Atoi(f[1])
		kids[b] = append(kids[b], a)
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
