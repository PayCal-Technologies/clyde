//go:build windows

package clyde

import (
	"os/exec"
	"sync"
	"syscall"
	"unsafe"
)

const (
	processTerminate             = 0x0001
	processSetQuota              = 0x0100
	createBreakawayFromJob       = 0x01000000
	jobObjectExtendedLimitClass  = 9
	jobObjectLimitKillOnJobClose = 0x00002000
)

var (
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	procAssignProcessToJobObject = kernel32.NewProc("AssignProcessToJobObject")
	procCloseHandle              = kernel32.NewProc("CloseHandle")
	procCreateJobObjectW         = kernel32.NewProc("CreateJobObjectW")
	procOpenProcess              = kernel32.NewProc("OpenProcess")
	procSetInformationJobObject  = kernel32.NewProc("SetInformationJobObject")
	procTerminateJobObject       = kernel32.NewProc("TerminateJobObject")
	mcpJobHandles                sync.Map
)

func configureMCPCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createBreakawayFromJob}
}

func trackMCPProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	job, _, _ := procCreateJobObjectW.Call(0, 0)
	if job == 0 {
		return nil
	}
	limits := jobObjectExtendedLimitInformation{}
	limits.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose
	if ok, _, _ := procSetInformationJobObject.Call(
		job,
		uintptr(jobObjectExtendedLimitClass),
		uintptr(unsafe.Pointer(&limits)),
		unsafe.Sizeof(limits),
	); ok == 0 {
		_, _, _ = procCloseHandle.Call(job)
		return nil
	}
	process, _, _ := procOpenProcess.Call(processTerminate|processSetQuota, 0, uintptr(cmd.Process.Pid))
	if process == 0 {
		_, _, _ = procCloseHandle.Call(job)
		return nil
	}
	defer procCloseHandle.Call(process)
	if ok, _, _ := procAssignProcessToJobObject.Call(job, process); ok == 0 {
		_, _, _ = procCloseHandle.Call(job)
		return nil
	}
	mcpJobHandles.Store(cmd.Process.Pid, job)
	return nil
}

func killMCPProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	if job, ok := mcpJobHandles.LoadAndDelete(cmd.Process.Pid); ok {
		handle := job.(uintptr)
		_, _, _ = procTerminateJobObject.Call(handle, 1)
		_, _, _ = procCloseHandle.Call(handle)
		return nil
	}
	return cmd.Process.Kill()
}

type jobObjectBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobObjectExtendedLimitInformation struct {
	BasicLimitInformation jobObjectBasicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}
