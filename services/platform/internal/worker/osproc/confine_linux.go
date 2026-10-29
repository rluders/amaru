package osproc

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

// Landlock through the stdlib syscall package, no library (ADR-0057 item 8 allows it). Numbers and
// values are those of linux/landlock.h and asm-generic/unistd.h, verified against the kernel
// headers on the development host and the kernel user guide (2026-10-28).
const (
	sysLandlockCreateRuleset = 444
	sysLandlockAddRule       = 445
	sysLandlockRestrictSelf  = 446

	createRulesetVersion = 1 << 0
	rulePathBeneath      = 1
	prSetNoNewPrivs      = 38
	oPath                = 0x200000 // O_PATH (x86-64 and arm64 value, also the generic one)

	accessWriteFile  = 1 << 1
	accessRemoveDir  = 1 << 4
	accessRemoveFile = 1 << 5
	accessMakeChar   = 1 << 6
	accessMakeDir    = 1 << 7
	accessMakeReg    = 1 << 8
	accessMakeSock   = 1 << 9
	accessMakeFifo   = 1 << 10
	accessMakeBlock  = 1 << 11
	accessMakeSym    = 1 << 12
	accessRefer      = 1 << 13 // ABI 2
	accessTruncate   = 1 << 14 // ABI 3
	accessIoctlDev   = 1 << 15 // ABI 5
)

// deviceNodes are the few device files every program writes; without them a confined process could
// not even discard output.
var deviceNodes = []string{"/dev/null", "/dev/tty"}

// landlockABI returns the Landlock ABI version of the running kernel, or the error that says why
// there is none (ENOSYS: not built in; EOPNOTSUPP: disabled at boot).
func landlockABI() (int, error) {
	v, _, e := syscall.Syscall(sysLandlockCreateRuleset, 0, 0, createRulesetVersion)
	if e != 0 {
		return 0, fmt.Errorf("Landlock is not available: %w", e)
	}
	return int(v), nil
}

func writeRights(abi int) uint64 {
	r := uint64(accessWriteFile | accessRemoveDir | accessRemoveFile | accessMakeChar | accessMakeDir |
		accessMakeReg | accessMakeSock | accessMakeFifo | accessMakeBlock | accessMakeSym)
	if abi >= 2 {
		r |= accessRefer
	}
	if abi >= 3 {
		r |= accessTruncate
	}
	if abi >= 5 {
		r |= accessIoctlDev
	}
	return r
}

// enforce confines the calling thread and everything it executes to writes under roots. It is
// called by the launch helper on the thread that then calls execve, so the restriction is the one
// the agent program starts with.
func enforce(roots []string) error {
	runtime.LockOSThread() // never unlocked: the thread is replaced by the program
	abi, err := landlockABI()
	if err != nil {
		return err
	}
	if abi < MinLandlockABI {
		return fmt.Errorf("Landlock ABI %d is below the required %d", abi, MinLandlockABI)
	}
	handled := writeRights(abi)
	// struct landlock_ruleset_attr, first field only (the ABI 1 size, 8 bytes): filesystem rights.
	attr := struct{ handledFS uint64 }{handled}
	fd, _, e := syscall.Syscall(sysLandlockCreateRuleset, uintptr(unsafe.Pointer(&attr)), unsafe.Sizeof(attr), 0)
	if e != 0 {
		return fmt.Errorf("landlock_create_ruleset: %w", e)
	}
	defer syscall.Close(int(fd))
	for _, r := range roots {
		if err := addRule(int(fd), r, handled); err != nil {
			return err
		}
	}
	for _, d := range deviceNodes {
		err := addRule(int(fd), d, handled&(accessWriteFile|accessTruncate|accessIoctlDev))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if _, _, e := syscall.RawSyscall6(syscall.SYS_PRCTL, prSetNoNewPrivs, 1, 0, 0, 0, 0); e != 0 {
		return fmt.Errorf("no_new_privs: %w", e)
	}
	if _, _, e := syscall.RawSyscall(sysLandlockRestrictSelf, fd, 0, 0); e != 0 {
		return fmt.Errorf("landlock_restrict_self: %w", e)
	}
	return nil
}

// addRule allows access beneath path. struct landlock_path_beneath_attr is packed: a u64 of rights
// then an s32 file descriptor, 12 bytes.
func addRule(ruleset int, path string, access uint64) error {
	p, err := syscall.Open(path, oPath|syscall.O_CLOEXEC, 0)
	if err != nil {
		if errors.Is(err, syscall.ENOENT) {
			return fmt.Errorf("%s: %w", path, os.ErrNotExist)
		}
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer syscall.Close(p)
	var b [12]byte
	*(*uint64)(unsafe.Pointer(&b[0])) = access
	*(*int32)(unsafe.Pointer(&b[8])) = int32(p)
	if _, _, e := syscall.Syscall6(sysLandlockAddRule, uintptr(ruleset), rulePathBeneath, uintptr(unsafe.Pointer(&b[0])), 0, 0, 0); e != 0 {
		return fmt.Errorf("landlock_add_rule %s: %w", path, e)
	}
	return nil
}

func execve(path string, argv, env []string) error { return syscall.Exec(path, argv, env) }
