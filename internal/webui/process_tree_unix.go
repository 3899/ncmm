//go:build !windows

package webui

import (
	"errors"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type unixProcessTree struct {
	mu     sync.Mutex
	pid    int
	timer  *time.Timer
	closed bool
}

func prepareProcessTree(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

func attachProcessTree(cmd *exec.Cmd) (processTree, error) {
	return &unixProcessTree{pid: cmd.Process.Pid}, nil
}

func (p *unixProcessTree) stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return os.ErrProcessDone
	}
	err := syscall.Kill(-p.pid, syscall.SIGTERM)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	if err == nil {
		p.timer = time.AfterFunc(processStopGrace, func() {
			p.mu.Lock()
			defer p.mu.Unlock()
			if !p.closed {
				_ = syscall.Kill(-p.pid, syscall.SIGKILL)
			}
		})
	}
	return err
}

func (p *unixProcessTree) forceKill() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return os.ErrProcessDone
	}
	err := syscall.Kill(-p.pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}

func (p *unixProcessTree) close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	if p.timer != nil {
		p.timer.Stop()
		p.timer = nil
	}
	return nil
}
