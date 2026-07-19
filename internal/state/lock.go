package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func lockPath(path string) (func(), error) {
	lockName := path + ".lock"
	deadline := time.Now().Add(lockTimeout)
	for {
		f, err := os.OpenFile(lockName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
			_ = f.Close()
			return func() { _ = os.Remove(lockName) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("creating state lock: %w", err)
		}
		if removed, err := removeStaleLock(lockName); err != nil {
			return nil, err
		} else if removed {
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for %s — if no orc process is running, remove the lock and retry", lockName)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func InspectLock(featureDir string) (LockInfo, error) {
	return inspectLockPath(filepath.Join(featureDir, Filename+".lock"))
}

// ClearStaleLock removes the feature's state lock only when it is provably
// stale — dead PID, or old without a valid PID. It reports whether a lock was
// removed; live or ambiguous locks are left untouched.
func ClearStaleLock(featureDir string) (bool, error) {
	lockName := filepath.Join(featureDir, Filename+".lock")
	lock, err := inspectLockPath(lockName)
	if err != nil {
		return false, err
	}
	if lock.Status != LockStale {
		return false, nil
	}
	if err := os.Remove(lockName); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("removing stale state lock %s: %w", lockName, err)
	}
	return true, nil
}

func removeStaleLock(lockName string) (bool, error) {
	lock, err := inspectLockPath(lockName)
	if err != nil {
		return false, err
	}
	if lock.Status == LockMissing {
		return true, nil
	}
	if lock.Status != LockStale {
		return false, nil
	}
	if err := os.Remove(lockName); err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, fmt.Errorf("removing stale state lock %s: %w", lockName, err)
	}
	return true, nil
}

func inspectLockPath(lockName string) (LockInfo, error) {
	lock := LockInfo{Path: lockName, Status: LockActive}
	info, err := os.Stat(lockName)
	if err != nil {
		if os.IsNotExist(err) {
			lock.Status = LockMissing
			lock.Detail = "not present"
			return lock, nil
		}
		return lock, fmt.Errorf("checking state lock: %w", err)
	}

	lock.Age = time.Since(info.ModTime())
	data, err := os.ReadFile(lockName)
	if err != nil {
		lock.Detail = "cannot read PID"
		if lock.Age > staleLockAge {
			lock.Status = LockStale
			lock.Detail = "old lock with unreadable PID"
		}
		return lock, nil
	}

	pidText := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidText)
	if err != nil || pid <= 0 {
		lock.Detail = "lock exists without a valid PID"
		if lock.Age > staleLockAge {
			lock.Status = LockStale
			lock.Detail = "old lock without a valid PID"
		}
		return lock, nil
	}
	lock.PID = pid
	if processExists(pid) {
		lock.Detail = fmt.Sprintf("held by pid %d", pid)
		return lock, nil
	}
	lock.Status = LockStale
	lock.Detail = fmt.Sprintf("pid %d is not running", pid)
	return lock, nil
}

func processExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// Start marks the feature as active and records a history entry.
