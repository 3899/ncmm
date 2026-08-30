//go:build windows

package webui

import (
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const windowsCreateNewProcessGroup = 0x00000200

type windowsProcessTree struct {
	mu  sync.Mutex
	job windows.Handle
}

func prepareProcessTree(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= windowsCreateNewProcessGroup
}

func attachProcessTree(cmd *exec.Cmd) (processTree, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		_ = windows.CloseHandle(job)
		return nil, err
	}
	process, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false,
		uint32(cmd.Process.Pid),
	)
	if err != nil {
		_ = windows.CloseHandle(job)
		return nil, err
	}
	defer windows.CloseHandle(process)
	if err := windows.AssignProcessToJobObject(job, process); err != nil {
		_ = windows.CloseHandle(job)
		return nil, err
	}
	return &windowsProcessTree{job: job}, nil
}

func (p *windowsProcessTree) stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.job == 0 {
		return os.ErrProcessDone
	}
	return windows.TerminateJobObject(p.job, 1)
}

func (p *windowsProcessTree) forceKill() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.job == 0 {
		return os.ErrProcessDone
	}
	return windows.TerminateJobObject(p.job, 1)
}

func (p *windowsProcessTree) close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.job == 0 {
		return nil
	}
	err := windows.CloseHandle(p.job)
	p.job = 0
	return err
}
