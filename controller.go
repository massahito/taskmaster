package taskmaster

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type ctrlStatus uint8

const (
	ctrlRunning ctrlStatus = iota
	ctrlStopped
)

// Controller handles the lifecycle of configurated processes.
//
// This should be created from [NewController].
//
// [Controller.Start] will start managing process and [Controller.Shutdown] will stop it.
//
// [Controller.Start] and [Controller.Shutdown] should be called only once for each Controller.
//
// [Controller.Shutdown] shouldn't be called before calling [Controller.Start].
type Controller struct {
	procs  procRefs
	cmdCh  chan procCmd
	pubChs map[chan<- Procs]bool
	status atomic.Value
	mutex  sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
}

// NewController creates new Controller from [Config].
func NewController(cfg Config) *Controller {
	ctx, cancel := context.WithCancel(context.Background())
	procs := buildProcRefFromGroup(cfg.Cluster)

	slog.Debug("new Controller were created.")

	c := &Controller{
		procs:  procs,
		cmdCh:  make(chan procCmd, 100),
		pubChs: map[chan<- Procs]bool{},
		ctx:    ctx,
		cancel: cancel,
	}
	c.status.Store(ctrlStopped)

	return c
}

// Start will start managing configurated processes.
//
// This function and [Controller.Shutdown] should be called only once for each Controller.
//
// [Controller.Shutdown] shouldn't be called before calling this function.
func (c *Controller) Start() error {
	if c.status.CompareAndSwap(ctrlRunning, ctrlRunning) {
		slog.Error("Controller.Start: this Controller has already started")
		return fmt.Errorf("Controller.Start: this Controller has already started")
	}

	go c.loop()

	resp := make(chan error)
	time.Sleep(500 * time.Millisecond)
	if err := c.sendCmd(procCmd{cmd: procAutoStart, arg: CmdArg{ID: -1}, resp: resp}); err != nil {
		slog.Error("Controller.Start", "error", err.Error())
		return err
	}

	return <-resp
}

// Subscribe enables subCh to receive current status of [Procs] managed by [Controller].
//
// This function should be used with [Controller.Unsubscribe] to delete subCh from [Controller].
// Most of case, [Controller.Unsubscribe] will be called as defer function.
//
// For efficiency, this function shouldn't be called more than twice for same subCh.
func (c *Controller) Subscribe(subCh chan<- Procs) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.pubChs[subCh] = true
}

// Unsubscribe delete subCh added by [Controller.Subscribe] from [Controller].
//
// This function should be called after [Controller.Subscribe] called and most of case, will be called as defer function.
// For efficiency, this function shouldn't be called more than twice for same subCh.
func (c *Controller) Unsubscribe(subCh chan<- Procs) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.pubChs, subCh)
}

// Stop will stop managing configurated processes.
//
// This function and [Controller.Start] should be called only once for each Controller.
//
// This function shouldn't be called before calling [Controller.Start].
func (c *Controller) Shutdown() error {
	if c.status.CompareAndSwap(ctrlStopped, ctrlStopped) {
		slog.Error("Controller.Shutdown: this Controller has already stopped")
		return fmt.Errorf("Controller.Shutdown: this Controller has already stopped")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	procsCh := make(chan Procs, 1)
	errCh := make(chan error)

	c.Subscribe(procsCh)
	defer c.Unsubscribe(procsCh)

	err := c.sendCmd(procCmd{cmd: procShutDown, resp: errCh, arg: CmdArg{}})
	if err != nil {
		slog.Error("Controller.Shutdown", "error", err.Error())
		return err
	}

	for {
		select {
		case err := <-errCh:
			if err != nil {
				slog.Error("Controller.Shutdown", "error", err.Error())
				return err
			}
		case resp := <-procsCh:
			if resp.IsAllProcDead() {
				return nil
			}
		case <-ctx.Done():
			slog.Warn("fail to stop gracefully")
			return fmt.Errorf("fail to stop gracefully")
		}
	}
}

func (c *Controller) sendCmd(cmd procCmd) error {
	select {
	case c.cmdCh <- cmd:
		return nil
	default:
		return fmt.Errorf("can't receive command.")
	}
}

// Possibly hanging if one of receivers is either not ready to receive,
// its channel is full, or exiting without calling Unsubscribe.
func (c *Controller) publish() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	procs := c.procs.Procs()
	for pubCh, _ := range c.pubChs {
		select {
		case pubCh <- procs:
			slog.Debug("publish: published", "channel", fmt.Sprintf("%p", pubCh))
		default:
			slog.Debug("publish: skipped", "channel", fmt.Sprintf("%p", pubCh))
		}
	}
}

// start proc
func (c *Controller) startByCmd(arg CmdArg) error {
	if isGeneralCmd(arg) {
		return c.startAllProcs()
	} else if isGroupCmd(arg) {
		return c.startGroupProcs(arg.Gname)
	} else if isProgCmd(arg) {
		return c.startProgProcs(arg.Gname, arg.Pname)
	} else if isProcCmd(arg) {
		return c.startIDProc(arg.Gname, arg.Pname, uint8(arg.ID))
	}

	return fmt.Errorf("can't find command args type")
}

func (c *Controller) startAutoProcs(arg CmdArg) error {
	for _, ps := range c.procs {
		if arg.Gname != "" && ps.Gname != arg.Gname {
			continue
		}
		if arg.Pname != "" && ps.Pname != arg.Pname {
			continue
		}
		if 0 <= arg.ID && ps.ID != uint8(arg.ID) {
			continue
		}
		if !ps.Prog.Autostart {
			continue
		}

		c.startProc(ps)
	}
	return nil
}

func (c *Controller) startAllProcs() error {
	for _, ps := range c.procs {
		c.startProc(ps)
	}
	return nil
}

func (c *Controller) startGroupProcs(gname string) error {
	procs := c.procs.filterByGroup(gname)
	if len(procs) == 0 {
		slog.Error("startGroupProcs", "error", "receive start request for non-existent group", "group", gname)
		return fmt.Errorf("start request for non-existent group: %s", gname)
	}
	for _, ps := range procs {
		c.startProc(ps)
	}
	return nil
}

func (c *Controller) startProgProcs(gname, pname string) error {
	procs := c.procs.filterByProg(gname, pname)
	if len(procs) == 0 {
		slog.Error("startProgProcs", "error", "receive start request for non-existent program", "group", gname, "program", pname)
		return fmt.Errorf("start request for non-existent group: %s:%s", gname, pname)
	}
	for _, ps := range procs {
		c.startProc(ps)
	}
	return nil
}

func (c *Controller) startIDProc(gname, pname string, id uint8) error {
	ps := c.procs.filterByID(gname, pname, id)
	if ps == nil {
		slog.Error("startIDProc", "error", "receive start request for non-existent process", "group", gname, "program", pname, "id", id)
		return fmt.Errorf("start request for non-existent process: %s:%s:%d", gname, pname, id)
	}

	return c.startProc(ps)
}

func (c *Controller) startProc(ps procRef) error {

	// trying to start a process of unstartable procRef is normal behavior
	// expecially calling command with entire/group/program scope.
	if !ps.Status.IsStartable() {
		slog.Debug("startProc: process not startable", "group", ps.Gname, "program", ps.Pname, "id", ps.ID)
		return fmt.Errorf("the process is not startable")
	}

	rStdout, wStdout, err := os.Pipe()
	if err != nil {
		slog.Error("startProc: pipe error")
		ps.Status = ProcFatal
		ps.Time = time.Now()
		ps.PID = 0
		ps.Retry = 0
		return err
	}
	defer wStdout.Close()

	rStderr, wStderr, err := os.Pipe()
	if err != nil {
		slog.Error("startProc: pipe error")
		ps.Status = ProcFatal
		ps.Time = time.Now()
		ps.PID = 0
		ps.Retry = 0
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

	ps.Time = time.Now()
	old := syscall.Umask(ps.Prog.Umask)
	proc, err := os.StartProcess(ps.Prog.Cmd[0], ps.Prog.Cmd, attr)
	syscall.Umask(old)

	if err != nil {
		defer rStdout.Close()
		defer rStderr.Close()

		ps.Retry++
		if ps.Retry < ps.Prog.Startretries {
			slog.Warn("process exited immediately; backing off", "group", ps.Gname, "program", ps.Pname, "id", ps.ID, "error", err.Error())
			ps.Status = ProcBackoff
			arg := CmdArg{Gname: ps.Gname, Pname: ps.Pname, ID: int(ps.ID)}
			go notify(c.ctx, c.cmdCh, procCmd{cmd: procFail, arg: arg}, ps.Prog.Startsecs)
			return err
		}

		slog.Warn("process failed to start and exited immediately. Retries exhausted — no further attempts will be made.", "group", ps.Gname, "program", ps.Pname, "id", ps.ID, "retry", ps.Retry, "max_retries", ps.Prog.Startretries)
		ps.Status = ProcFatal
		ps.Time = time.Now()
		ps.PID = 0
		ps.Retry = 0

		return err
	}

	ps.PID = proc.Pid
	ps.Status = ProcStarting
	slog.Info("process starting", "group", ps.Gname, "program", ps.Pname, "id", ps.ID, "pid", ps.PID)

	go notify(c.ctx, c.cmdCh, procCmd{cmd: procStartCheck, pid: proc.Pid}, ps.Prog.Startsecs)

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

	go reap(c.ctx, proc, c.cmdCh)

	return nil
}

// stop proc
func (c *Controller) stopByCmd(arg CmdArg) error {
	if isGeneralCmd(arg) {
		return c.stopAllProcs()
	} else if isGroupCmd(arg) {
		return c.stopGroupProcs(arg.Gname)
	} else if isProgCmd(arg) {
		return c.stopProgProcs(arg.Gname, arg.Pname)
	} else if isProcCmd(arg) {
		return c.stopIDProc(arg.Gname, arg.Pname, uint8(arg.ID))
	}

	return fmt.Errorf("can't find command args type")
}

func (c *Controller) stopAllProcs() error {
	for _, ps := range c.procs {
		c.stopProc(ps)
	}
	return nil
}

func (c *Controller) stopGroupProcs(gname string) error {
	procs := c.procs.filterByGroup(gname)
	for _, ps := range procs {
		c.stopProc(ps)
	}
	return nil
}

func (c *Controller) stopProgProcs(gname, pname string) error {
	procs := c.procs.filterByProg(gname, pname)
	for _, ps := range procs {
		c.stopProc(ps)
	}
	return nil
}

func (c *Controller) stopIDProc(gname, pname string, id uint8) error {
	ps := c.procs.filterByID(gname, pname, id)
	return c.stopProc(ps)
}

func (c *Controller) stopProc(ps *Proc) error {

	if !ps.Status.IsStoppable() {
		if ps.Status == ProcBackoff {
			ps.Status = ProcStopped
			ps.Time = time.Now()
			ps.PID = 0
			ps.Retry = 0
			slog.Warn("stopProc: stopped the backoff process", "group", ps.Gname, "program", ps.Pname, "id", ps.ID)
		}
		return nil
	}

	ps.Status = ProcStopping

	pid := ps.PID
	if ps.Prog.Stopasgroup {
		pid *= -1
	}

	err := syscall.Kill(pid, ps.Prog.Stopsignal)
	if err != nil {
		slog.Error("stopProc: kill error", "group", ps.Gname, "program", ps.Pname, "id", ps.ID, "pid", pid)
		return err
	}

	slog.Info("process stopping", "group", ps.Gname, "program", ps.Pname, "id", ps.ID, "pid", pid)

	go notify(c.ctx, c.cmdCh, procCmd{cmd: procStopCheck, pid: ps.PID}, ps.Prog.Stopwaitsecs)

	return nil
}

func (c *Controller) loop() {
	defer c.cancel()

	c.status.Store(ctrlRunning)

	for {
		if c.status.Load() == ctrlStopped && c.procs.Procs().IsAllProcDead() {
			time.Sleep(time.Second * 1)
			c.publish()
			slog.Info("shutdown Controller complete")
			return
		}

		psch := <-c.cmdCh
		c.handleCmd(psch)
		c.publish()
	}
}

// Possibly hanging if psch.resp is either not ready to receive,
// its channel is full, or exiting without calling Unsubscribe.
func (c *Controller) handleCmd(psch procCmd) {
	if psch.cmd.IsUserCmd() && c.status.Load() == ctrlStopped {
		psch.resp <- fmt.Errorf("the Controller is stopped.")
		return
	}

	switch psch.cmd {
	case procAutoStart:
		psch.resp <- c.startAutoProcs(psch.arg)
	case procStart:
		psch.resp <- c.startByCmd(psch.arg)
	case procStop:
		psch.resp <- c.stopByCmd(psch.arg)
	case procGetStatus:
		psch.resp <- nil
	case procShutDown:
		slog.Info("receive shutdown command")
		c.status.Store(ctrlStopped)
		psch.resp <- c.stopAllProcs()
	case procCreate:
		psch.resp <- c.createProc(psch.arg, psch.cfg)
	case procDelete:
		psch.resp <- c.deleteProc(psch.arg)
	case procExit:
		c.handleExit(psch.state)
	case procStartCheck:
		c.handleStartCheck(psch)
	case procStopCheck:
		c.handleStopCheck(psch)
	case procFail:
		c.handleProcFail(psch)
	default:
		panic("unknwon procCmd")
	}
}

func (c *Controller) createProc(arg CmdArg, cfg Config) error {

	for gname, group := range cfg.Cluster {
		if arg.Gname != "" && arg.Gname != gname {
			continue
		}
		for pname, prog := range group.Progs {
			if arg.Pname != "" && arg.Pname != pname {
				continue
			}
			c.procs = append(c.procs, buildProcRef(gname, pname, group.Priority, prog)...)
		}
	}
	sort.Sort(c.procs)

	return nil
}

func (c *Controller) deleteProc(arg CmdArg) error {
	procs := procRefs{}

	for _, ps := range c.procs {
		if arg.Gname == "" || arg.Gname == ps.Gname {
			if arg.Pname == "" || arg.Pname == ps.Pname {
				continue
			}
		}
		procs = append(procs, ps)
	}
	c.procs = procs

	return nil
}

func (c *Controller) handleExit(exitState os.ProcessState) {
	ps := c.procs.searchByPID(exitState.Pid())

	if ps == nil {
		panic(fmt.Sprintf("can't find pid: %d", exitState.Pid()))
	}

	slog.Info("detect process exit", "group", ps.Gname, "program", ps.Pname, "id", ps.ID, "pid", ps.PID, "exit_code", exitState.ExitCode())

	switch {
	case ps.Status == ProcStopping || exitState.ExitCode() == -1:
		// correct behavior
		slog.Info("process stopped", "group", ps.Gname, "program", ps.Pname, "id", ps.ID, "pid", ps.PID)
		ps.Status = ProcStopped
		ps.Time = time.Now()
		ps.PID = 0
	case ps.Status == ProcStarting:
		// correct behavior, but process stopped unexpectedly
		ps.Retry++
		if ps.Retry < ps.Prog.Startretries {
			slog.Info("process exited before startsecs; backing off", "group", ps.Gname, "program", ps.Pname, "id", ps.ID, "pid", ps.PID)
			ps.Status = ProcBackoff
			return
		}
		slog.Warn("process failed to start and exited immediately. Retries exhausted — no further attempts will be made.", "group", ps.Gname, "program", ps.Pname, "id", ps.ID, "pid", ps.PID, "retry", ps.Retry, "max_retries", ps.Prog.Startretries)
		ps.Status = ProcFatal
		ps.Time = time.Now()
		ps.PID = 0
		ps.Retry = 0
	case ps.Status == ProcRunning:
		// correct behavior, but process might stop unexpectedly
		autorestart := ps.Prog.Autorestart
		ps.Time = time.Now()
		ps.PID = 0
		ps.Retry = 0
		switch {
		case autorestart == RestartNever:
			slog.Info("process exited", "group", ps.Gname, "program", ps.Pname, "id", ps.ID, "pid", ps.PID)
			ps.Status = ProcExited
		case autorestart == RestartUnexpected && slices.Contains(ps.Prog.Exitcodes, uint8(exitState.ExitCode())):
			slog.Info("process exited", "group", ps.Gname, "program", ps.Pname, "id", ps.ID, "pid", ps.PID)
			ps.Status = ProcExited
		default:
			slog.Info("process will be restarted", "group", ps.Gname, "program", ps.Pname, "id", ps.ID, "pid", ps.PID, "exit_code", exitState.ExitCode())
			ps.Status = ProcStopped
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

func (c *Controller) handleStartCheck(psch procCmd) {
	ps := c.procs.searchByPID(psch.pid)

	// It's possible when stopping process right after starting.
	if ps == nil {
		slog.Debug("handleStartCheck: can't find process", "pid", psch.pid)
		return
	}

	// If they already stopped the process, ignore procCmd
	if time.Now().Sub(ps.Time) < ps.Prog.Startsecs {
		return
	}

	slog.Info("check start", "group", ps.Gname, "program", ps.Pname, "id", ps.ID, "pid", ps.PID)

	switch ps.Status {
	case ProcStarting:
		// state should be running
		slog.Info("process was starting cleanly", "group", ps.Gname, "program", ps.Pname, "id", ps.ID, "pid", ps.PID)
		ps.Status = ProcRunning
		return
	case ProcBackoff:
		// it have to start new process
		slog.Info("process was backed off", "group", ps.Gname, "program", ps.Pname, "id", ps.ID, "pid", ps.PID)
		ps.Status = ProcStopped // set stopped status temporary to call startProc normally
		c.startProc(ps)
		return
	case ProcStopping:
		// it happens when stopping process right after starting
		slog.Debug("process might be stopped right after starting", "group", ps.Gname, "program", ps.Pname, "id", ps.ID, "pid", ps.PID)
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

func (c *Controller) handleStopCheck(psch procCmd) {
	ps := c.procs.searchByPID(psch.pid)
	// it's natural to be nil when process is stopped normally
	if ps == nil {
		return
	}

	// It's unlikely happened, but in case when os create new process with exact same pid.
	if time.Now().Sub(ps.Time) < ps.Prog.Stopwaitsecs {
		return
	}

	slog.Debug("check stop", "group", ps.Gname, "program", ps.Pname, "id", ps.ID, "pid", ps.PID)

	switch ps.Status {
	case ProcStopping:
		slog.Warn("process wasn't stopped correctly; sending SIGKILL", "group", ps.Gname, "program", ps.Pname, "id", ps.ID, "pid", ps.PID)
		err := syscall.Kill(ps.PID, syscall.SIGKILL)
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

// handleProcFail restarts ps which failed right after starting.
func (c *Controller) handleProcFail(psch procCmd) {
	ps := c.procs.filterByID(psch.arg.Gname, psch.arg.Pname, uint8(psch.arg.ID))

	if ps == nil {
		// already stopped by user interaction.
		return
	}

	if time.Now().Sub(ps.Time) < ps.Prog.Startsecs {
		// already stopped by user interaction.
		return
	}

	switch ps.Status {
	case ProcBackoff:
		// start new process
		slog.Info("process was retried", "group", ps.Gname, "program", ps.Pname, "id", ps.ID)
		ps.Status = ProcStopped // set stopped status temporary to call startProc normally
		c.startProc(ps)
		return
	default:
		panic("receive handleProcFail in non-ProcBackoff status")
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

func notify(ctx context.Context, cmdCh chan<- procCmd, psch procCmd, wait time.Duration) {
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
func reap(ctx context.Context, proc *os.Process, cmdCh chan<- procCmd) {
	state, err := proc.Wait()
	if err != nil {
		slog.Error("reap: got an error from os.Wait()", "error", err.Error())
	}

	select {
	case cmdCh <- procCmd{cmd: procExit, state: *state}:
	case <-ctx.Done():
	}
}
