package taskmaster

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"sync"
	"syscall"
	"time"
)

type ctrlStatus uint8

const (
	ctrlRunning ctrlStatus = iota
	ctrlStopped
)

type controller struct {
	cfg    Config
	procs  []*Proc
	cmdCh  chan procCmd
	pubChs map[chan<- []Proc]bool
	status ctrlStatus
	mutex  sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
}

func NewController(cfg Config) *controller {
	ctx, cancel := context.WithCancel(context.Background())
	procs := genProcs(cfg.Cluster)

	slog.Debug("new controller were created.")

	return &controller{
		cfg:    cfg,
		procs:  procs,
		cmdCh:  make(chan procCmd, 100),
		pubChs: map[chan<- []Proc]bool{},
		status: ctrlStopped,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (c *controller) Start() error {
	resp := make(chan error)
	go c.loop()
	if err := c.SendCmd(procCmd{cmd: procStartUp, resp: resp}); err != nil {
		return err
	}
	return <-resp
}

func (c *controller) Subscribe(subCh chan<- []Proc) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.pubChs[subCh] = true
}

func (c *controller) Unsubscribe(subCh chan<- []Proc) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.pubChs, subCh)
}

func (c *controller) SendCmd(cmd procCmd) error {
	select {
	case c.cmdCh <- cmd:
		return nil
	default:
		return fmt.Errorf("can't receive command.")
	}
}

// Possibly hanging if one of receivers is either not ready to receive,
// its channel is full, or exiting without calling Unsubscribe.
func (c *controller) publish() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	procs := copyProcs(c.procs)
	for pubCh, _ := range c.pubChs {
		pubCh <- procs
	}
}

// start proc
func (c *controller) startAutoProcs() error {
	for _, ps := range c.procs {
		if !isStartable(ps.Status) || !ps.Prog.Autostart {
			continue
		}
		err := c.startProc(ps)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) startAllProcs() error {
	for _, ps := range c.procs {
		if !isStartable(ps.Status) {
			continue
		}
		err := c.startProc(ps)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) startGroupProcs(gname string) error {
	procs := getGroupProcs(c.procs, gname)
	for _, ps := range procs {
		if !isStartable(ps.Status) {
			continue
		}
		err := c.startProc(ps)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) startProgProcs(gname, pname string) error {
	procs := getProgProcs(c.procs, gname, pname)
	for _, ps := range procs {
		if !isStartable(ps.Status) {
			continue
		}
		err := c.startProc(ps)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) startIDProc(gname, pname string, id uint8) error {
	ps := getIDProc(c.procs, gname, pname, id)
	if ps == nil {
		return fmt.Errorf("can't find the process %s:%s:%d", gname, pname, id)
	}
	if !isStartable(ps.Status) {
		return fmt.Errorf("the process is not startable")
	}
	return c.startProc(ps)
}

func (c *controller) startProc(ps *Proc) error {
	rStdout, wStdout, err := os.Pipe()
	if err != nil {
		slog.Error("startProc: pipe error")
		return err
	}
	defer wStdout.Close()

	rStderr, wStderr, err := os.Pipe()
	if err != nil {
		slog.Error("startProc: pipe error")
		return err
	}
	defer wStderr.Close()

	attr := &os.ProcAttr{
		Files: []*os.File{
			os.Stdin,
			wStdout,
			wStderr,
		},
		Dir: ps.Prog.Directory,
		Env: ps.Prog.Environment,
	}

	proc, err := os.StartProcess(ps.Prog.Cmd[0], ps.Prog.Cmd, attr)
	if err != nil {
		slog.Error("startProc: error occured in os.StartProcess()", "error", err.Error())
		return err
	}

	if ps.Prog.Stdout == "" {
		defer rStdout.Close()
	} else {
		go stdLog(ps.Stdout, rStdout)
	}

	if ps.Prog.Stderr == "" {
		defer rStderr.Close()
	} else {
		go stdLog(ps.Stderr, rStderr)
	}

	go reapProc(c.ctx, proc, c.cmdCh)

	ps.Pid = proc.Pid
	ps.Status = ProcStarting
	ps.Time = time.Now()

	slog.Info("process starting", "group", ps.Gname, "program", ps.Pname, "id", ps.Id, "pid", ps.Pid)

	go notifyCheck(c.ctx, c.cmdCh, procCmd{cmd: procStartCheck, pid: proc.Pid}, ps.Prog.Startsecs)

	return nil
}

func (c *controller) stopAllProcs() error {
	if isContainBackoff(c.procs) {
		return fmt.Errorf("the procs contains a backoff process")
	}
	for _, ps := range c.procs {
		if !isStoppable(ps.Status) {
			continue
		}
		err := c.stopProc(ps)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) stopGroupProcs(gname string) error {
	procs := getGroupProcs(c.procs, gname)
	if isContainBackoff(procs) {
		return fmt.Errorf("the procs contains a backoff process")
	}
	for _, ps := range procs {
		if !isStoppable(ps.Status) {
			continue
		}
		err := c.stopProc(ps)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) stopProgProcs(gname, pname string) error {
	procs := getProgProcs(c.procs, gname, pname)
	if isContainBackoff(procs) {
		return fmt.Errorf("the procs contains a backoff process")
	}
	for _, ps := range procs {
		if !isStoppable(ps.Status) {
			continue
		}
		err := c.stopProc(ps)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) stopIDProc(gname, pname string, id uint8) error {
	ps := getIDProc(c.procs, gname, pname, id)
	if !isStoppable(ps.Status) {
		return fmt.Errorf("the process is not stoppable")
	}
	return c.stopProc(ps)
}

func (c *controller) stopProc(ps *Proc) error {
	if ps.Pid == 0 {
		panic("stopProc: stopping process of pid 0")
	}

	ps.Status = ProcStopping

	err := syscall.Kill(ps.Pid, ps.Prog.Stopsignal)
	if err != nil {
		slog.Error("stopProc: kill error", "pid", ps.Pid)
		return err
	}

	slog.Info("process stopping", "group", ps.Gname, "program", ps.Pname, "id", ps.Id, "pid", ps.Pid)

	go notifyCheck(c.ctx, c.cmdCh, procCmd{cmd: procStopCheck, pid: ps.Pid}, ps.Prog.Stopwaitsecs)

	return nil
}

func (c *controller) loop() {
	defer c.cancel()

	c.status = ctrlRunning

	for {
		if c.status == ctrlStopped && isAllProcDead(copyProcs(c.procs)) {
			return
		}

		psch := <-c.cmdCh
		c.handleCmd(psch)
		c.publish()
	}
}

// Possibly hanging if psch.resp is either not ready to receive,
// its channel is full, or exiting without calling Unsubscribe.
func (c *controller) handleCmd(psch procCmd) {
	switch psch.cmd {
	case procStartUp:
		psch.resp <- c.startAutoProcs()
	case procGetStatus:
		psch.resp <- nil
	case procShutDown:
		c.status = ctrlStopped
		psch.resp <- c.stopAllProcs()
	case procExit:
		c.handleExit(psch.state)
	case procStartCheck:
		c.handleStartCheck(psch)
	case procStopCheck:
		c.handleStopCheck(psch)
	}
}

func (c *controller) handleExit(exitState os.ProcessState) {
	ps := searchProc(c.procs, exitState.Pid())

	if ps == nil {
		panic(fmt.Sprintf("can't find pid: %d", exitState.Pid()))
	}

	slog.Info("detect process exit", "group", ps.Gname, "program", ps.Pname, "id", ps.Id, "pid", ps.Pid, "exit_code", exitState.ExitCode())

	switch {
	case ps.Status == ProcStopping || exitState.ExitCode() == -1:
		// correct behavior
		slog.Info("process stopped", "group", ps.Gname, "program", ps.Pname, "id", ps.Id, "pid", ps.Pid)
		ps.Status = ProcStopped
		ps.Time = time.Now()
		ps.Pid = 0
	case ps.Status == ProcStarting:
		// correct behavior, but process stopped unexpectedly
		ps.Retry++
		if ps.Retry < ps.Prog.Startretries {
			slog.Info("process exited before startsecs; backing off", "group", ps.Gname, "program", ps.Pname, "id", ps.Id, "pid", ps.Pid)
			ps.Status = ProcBackoff
			return
		}
		slog.Warn("process reached the maximum of retries", "group", ps.Gname, "program", ps.Pname, "id", ps.Id, "pid", ps.Pid, "retry", ps.Retry, "max_retries", ps.Prog.Startretries)
		ps.Status = ProcFatal
		ps.Time = time.Now()
		ps.Pid = 0
		ps.Retry = 0
	case ps.Status == ProcRunning:
		// correct behavior, but process might stop unexpectedly
		autorestart := ps.Prog.Autorestart
		switch {
		case autorestart == RestartNever:
			slog.Info("process exited", "group", ps.Gname, "program", ps.Pname, "id", ps.Id, "pid", ps.Pid)
			ps.Status = ProcExited
			ps.Time = time.Now()
		case autorestart == RestartUnexpected && slices.Contains(ps.Prog.Exitcodes, uint8(exitState.ExitCode())):
			slog.Info("process exited", "group", ps.Gname, "program", ps.Pname, "id", ps.Id, "pid", ps.Pid)
			ps.Status = ProcExited
			ps.Time = time.Now()
		default:
			slog.Info("process will be restarted", "group", ps.Gname, "program", ps.Pname, "id", ps.Id, "pid", ps.Pid, "exit_code", exitState.ExitCode())
			c.startProc(ps)
		}
	case ps.Status == ProcStopped:
		// incorrect behavior
		panic("receive handleExit in procStopped status")
	case ps.Status == ProcBackoff:
		// incorrect behavior
		panic("receive handleExit in procBackoff status")
	case ps.Status == ProcExited:
		// incorrect behavior
		panic("receive handleExit in procExited status")
	case ps.Status == ProcFatal:
		// incorrect behavior
		panic("receive handleExit in procFatal status")
	}
}

func (c *controller) handleStartCheck(psch procCmd) {
	ps := searchProc(c.procs, psch.pid)

	// It's possible when stopping process right after starting.
	if ps == nil {
		slog.Debug("handleStartCheck: can't find process", "pid", psch.pid)
		return
	}

	// If they already stopped the process, ignore procCmd
	if time.Now().Sub(ps.Time) < ps.Prog.Startsecs {
		return
	}

	slog.Info("check start", "group", ps.Gname, "program", ps.Pname, "id", ps.Id, "pid", ps.Pid)

	switch ps.Status {
	case ProcStarting:
		// state should be running
		slog.Info("process was starting cleanly", "group", ps.Gname, "program", ps.Pname, "id", ps.Id, "pid", ps.Pid)
		ps.Status = ProcRunning
		return
	case ProcBackoff:
		// it have to start new process
		slog.Info("process was backed off", "group", ps.Gname, "program", ps.Pname, "id", ps.Id, "pid", ps.Pid)
		c.startProc(ps)
		return
	case ProcStopping:
		// it happens when stopping process right after starting
		slog.Debug("process might be stopped right after starting", "group", ps.Gname, "program", ps.Pname, "id", ps.Id, "pid", ps.Pid)
		return
	case ProcStopped:
		// incorrect behavior.
		panic("receive handleStartCheck in procStopped status")
	case ProcRunning:
		// incorrect behavior
		panic("receive handleStartCheck in procRunning status")
	case ProcExited:
		// incorrect behavior
		panic("receive handleStartCheck in procRunning status")
	case ProcFatal:
		// incorrect behavior.
		panic("receive handleStartCheck in procFatal status")
	}
}

func (c *controller) handleStopCheck(psch procCmd) {
	ps := searchProc(c.procs, psch.pid)
	// it's natural to be nil when process is stopped normally
	if ps == nil {
		return
	}

	// It's unlikely happened, but in case when os create new process with exact same pid.
	if time.Now().Sub(ps.Time) < ps.Prog.Stopwaitsecs {
		return
	}

	slog.Debug("check stop", "group", ps.Gname, "program", ps.Pname, "id", ps.Id, "pid", ps.Pid)

	switch ps.Status {
	case ProcStopping:
		slog.Warn("process wasn't stopped correctly; sending SIGKILL", "group", ps.Gname, "program", ps.Pname, "id", ps.Id, "pid", ps.Pid)
		err := syscall.Kill(ps.Pid, syscall.SIGKILL)
		if err != nil {
			slog.Error("handleStopCheck: got an error from syscall.Kill", "error", err.Error())
		}
	case ProcStarting:
		// incorrect behavior.
		panic("receive handleStopCheck in procStarting status")
	case ProcBackoff:
		// incorrect behavior.
		panic("receive handleStopCheck in procBackoff status")
	case ProcStopped:
		// incorrect behavior.
		panic("receive handleStopCheck in procStopped status")
	case ProcRunning:
		// incorrect behavior
		panic("receive handleStopCheck in procRunning status")
	case ProcExited:
		// incorrect behavior
		panic("receive handleStopCheck in procExited status")
	case ProcFatal:
		// incorrect behavior.
		panic("receive handleStopCheck in procFatal status")
	}

}

// Writing log for child process's stdout/stderr.
func stdLog(path string, in *os.File) {
	defer in.Close()

	out, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		slog.Error("stdLog: fail to open file", "path", path, "error", err.Error())
		return
	}
	defer out.Close()

	reader := bufio.NewReader(in)
	for {
		str, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				slog.Error("stdLog: got an error from Rearder.ReadString()", "error", err.Error())
			}
			return
		}
		_, err = out.WriteString(str)
		if err != nil {
			slog.Error("stdLog: got an error from *File.WriteString().", "error", err.Error())
			return
		}
	}
}

func notifyCheck(ctx context.Context, cmdCh chan<- procCmd, psch procCmd, wait time.Duration) {
	select {
	case <-time.After(wait):
		select {
		case cmdCh <- psch:
			slog.Debug("send notify", "pid", psch.pid)
		case <-ctx.Done():
		}
	case <-ctx.Done():
	}
}

// Wait process is finished.
func reapProc(ctx context.Context, proc *os.Process, cmdCh chan<- procCmd) {
	state, err := proc.Wait()
	if err != nil {
		slog.Error("reapProc: got an error from os.Wait()", "error", err.Error())
	}

	select {
	case cmdCh <- procCmd{cmd: procExit, state: *state}:
	case <-ctx.Done():
	}
}
