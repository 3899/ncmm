package webui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultProcessOutputLimit = 1 << 20
	processStopGrace          = 3 * time.Second
)

var errProcessSupervisorClosed = errors.New("process supervisor is closed")

type processCommandFactory func(context.Context, string, ...string) *exec.Cmd

type processSpec struct {
	Kind           string
	Command        string
	Args           []string
	Dir            string
	Env            []string
	Stdout         io.Writer
	Stderr         io.Writer
	Timeout        time.Duration
	CommandFactory processCommandFactory
}

type managedProcess struct {
	cancel        context.CancelFunc
	control       *processControl
	done          chan struct{}
	stopRequested atomic.Bool
	mu            sync.Mutex
	err           error
	ctxErr        error
}

func (p *managedProcess) Wait() error {
	<-p.done
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

func (p *managedProcess) Stop() {
	if p == nil {
		return
	}
	p.stopRequested.Store(true)
	p.cancel()
}

func (p *managedProcess) ForceKill() {
	if p == nil {
		return
	}
	p.stopRequested.Store(true)
	_ = p.control.forceKill()
}

func (p *managedProcess) StopRequested() bool {
	return p != nil && p.stopRequested.Load()
}

func (p *managedProcess) ContextError() error {
	<-p.done
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ctxErr
}

type processSupervisor struct {
	mu        sync.Mutex
	closed    bool
	processes map[*managedProcess]struct{}
	command   processCommandFactory
}

func newProcessSupervisor() *processSupervisor {
	return &processSupervisor{
		processes: make(map[*managedProcess]struct{}),
		command:   exec.CommandContext,
	}
}

func (s *processSupervisor) Start(parent context.Context, spec processSpec) (*managedProcess, error) {
	if parent == nil {
		parent = context.Background()
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, errProcessSupervisorClosed
	}
	s.mu.Unlock()

	ctx := parent
	cancel := func() {}
	if spec.Timeout > 0 {
		ctx, cancel = context.WithTimeout(parent, spec.Timeout)
	} else {
		ctx, cancel = context.WithCancel(parent)
	}
	factory := spec.CommandFactory
	if factory == nil {
		factory = s.command
	}
	cmd := factory(ctx, spec.Command, spec.Args...)
	if cmd == nil {
		cancel()
		return nil, fmt.Errorf("create %s process: command factory returned nil", spec.Kind)
	}
	cmd.Dir = spec.Dir
	cmd.Env = spec.Env
	cmd.Stdout = spec.Stdout
	cmd.Stderr = spec.Stderr
	prepareProcessTree(cmd)
	control := newProcessControl()
	cmd.Cancel = control.stop
	cmd.WaitDelay = processStopGrace

	if err := cmd.Start(); err != nil {
		control.set(nil, err)
		cancel()
		return nil, fmt.Errorf("start %s process: %w", spec.Kind, err)
	}
	tree, err := attachProcessTree(cmd)
	control.set(tree, err)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		cancel()
		return nil, fmt.Errorf("supervise %s process: %w", spec.Kind, err)
	}

	process := &managedProcess{
		cancel: cancel, control: control, done: make(chan struct{}),
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		go s.wait(process, cmd, ctx, cancel)
		process.Stop()
		_ = process.Wait()
		return nil, errProcessSupervisorClosed
	}
	s.processes[process] = struct{}{}
	s.mu.Unlock()

	go s.wait(process, cmd, ctx, cancel)
	return process, nil
}

func (s *processSupervisor) wait(process *managedProcess, cmd *exec.Cmd, ctx context.Context, cancel context.CancelFunc) {
	err := cmd.Wait()
	ctxErr := ctx.Err()
	process.control.close()
	cancel()
	process.mu.Lock()
	process.err = err
	process.ctxErr = ctxErr
	process.mu.Unlock()
	close(process.done)
	s.mu.Lock()
	delete(s.processes, process)
	s.mu.Unlock()
}

func (s *processSupervisor) RunOutput(parent context.Context, spec processSpec, limit int) ([]byte, error) {
	if limit <= 0 {
		limit = defaultProcessOutputLimit
	}
	output := newTailBuffer(limit)
	spec.Stdout = output
	spec.Stderr = output
	process, err := s.Start(parent, spec)
	if err != nil {
		return nil, err
	}
	err = process.Wait()
	if contextErr := process.ContextError(); contextErr != nil {
		err = contextErr
	}
	return output.Bytes(), err
}

func (s *processSupervisor) Close(grace time.Duration) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.closed = true
	processes := make([]*managedProcess, 0, len(s.processes))
	for process := range s.processes {
		processes = append(processes, process)
	}
	s.mu.Unlock()
	for _, process := range processes {
		process.Stop()
	}
	if waitProcesses(processes, grace) {
		return
	}
	for _, process := range processes {
		process.ForceKill()
	}
	_ = waitProcesses(processes, processStopGrace)
}

func waitProcesses(processes []*managedProcess, timeout time.Duration) bool {
	if len(processes) == 0 {
		return true
	}
	done := make(chan struct{})
	go func() {
		for _, process := range processes {
			<-process.done
		}
		close(done)
	}()
	if timeout <= 0 {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

type tailBuffer struct {
	mu    sync.Mutex
	limit int
	data  []byte
}

func newTailBuffer(limit int) *tailBuffer {
	return &tailBuffer{limit: limit}
}

func (b *tailBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	written := len(data)
	if len(data) >= b.limit {
		b.data = append(b.data[:0], data[len(data)-b.limit:]...)
		return written, nil
	}
	b.data = append(b.data, data...)
	if overflow := len(b.data) - b.limit; overflow > 0 {
		copy(b.data, b.data[overflow:])
		b.data = b.data[:b.limit]
	}
	return written, nil
}

func (b *tailBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.data...)
}

type processTree interface {
	stop() error
	forceKill() error
	close() error
}

type processControl struct {
	ready chan struct{}
	once  sync.Once
	mu    sync.Mutex
	tree  processTree
	err   error
}

func newProcessControl() *processControl {
	return &processControl{ready: make(chan struct{})}
}

func (c *processControl) set(tree processTree, err error) {
	c.mu.Lock()
	c.tree = tree
	c.err = err
	c.mu.Unlock()
	c.once.Do(func() { close(c.ready) })
}

func (c *processControl) stop() error {
	<-c.ready
	c.mu.Lock()
	tree, err := c.tree, c.err
	c.mu.Unlock()
	if err != nil {
		return err
	}
	if tree == nil {
		return nil
	}
	return tree.stop()
}

func (c *processControl) forceKill() error {
	<-c.ready
	c.mu.Lock()
	tree, err := c.tree, c.err
	c.mu.Unlock()
	if err != nil {
		return err
	}
	if tree == nil {
		return nil
	}
	return tree.forceKill()
}

func (c *processControl) close() {
	<-c.ready
	c.mu.Lock()
	tree := c.tree
	c.mu.Unlock()
	if tree != nil {
		_ = tree.close()
	}
}
