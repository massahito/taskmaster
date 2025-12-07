package taskmaster

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"
)

type TaskCmd struct {
	c   *controller
	pid int
}

func NewTaskCmd(c *controller) *TaskCmd {
	return &TaskCmd{
		c:   c,
		pid: os.Getpid(),
	}
}

type CmdArg struct {
	Gname string
	Pname string
	Id    int
}

func isGeneralCmd(arg CmdArg) bool {
	if arg.Gname == "" && arg.Pname == "" && arg.Id < 0 {
		return true
	}
	return false
}

func isGroupCmd(arg CmdArg) bool {
	if arg.Gname != "" && arg.Pname == "" && arg.Id < 0 {
		return true
	}
	return false
}

func isProgCmd(arg CmdArg) bool {
	if arg.Gname != "" && arg.Pname != "" && arg.Id < 0 {
		return true
	}
	return false
}

func isProcCmd(arg CmdArg) bool {
	if arg.Gname != "" && arg.Pname != "" && 0 <= arg.Id {
		return true
	}
	return false
}

type cmd uint32

const (
	procStartUp cmd = iota
	procGetStatus
	procStart
	procStop
	procShutDown
	procExit
	procStartCheck
	procStopCheck
	procFail
)

type procCmd struct {
	pid   int
	cmd   cmd
	arg   CmdArg
	state os.ProcessState
	resp  chan<- error
}

func (t *TaskCmd) Pid(_ *CmdArg, pid *int) error {
	*pid = t.pid
	return nil
}

func (t *TaskCmd) Start(req *CmdArg, resp *[]Proc) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	procsCh := make(chan []Proc, 1)
	errCh := make(chan error)

	t.c.Subscribe(procsCh)
	defer t.c.Unsubscribe(procsCh)

	err := t.c.SendCmd(procCmd{cmd: procStart, resp: errCh, arg: *req})
	if err != nil {
		return err
	}

	for {
		select {
		case err := <-errCh:
			if err != nil {
				return err
			}
		case *resp = <-procsCh:
			if checkStatus(*resp, ProcStarting|ProcRunning, *req) {
				return nil
			}
		case <-ctx.Done():
			slog.Warn("TaskCmd.Start: Timeout: fail to start processes")
			return fmt.Errorf("TaskCmd.Start: Timeout: fail to start processes")
		}
	}

	return nil

}

func (t *TaskCmd) Stop(req *CmdArg, resp *[]Proc) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	procsCh := make(chan []Proc, 1)
	errCh := make(chan error)

	t.c.Subscribe(procsCh)
	defer t.c.Unsubscribe(procsCh)

	err := t.c.SendCmd(procCmd{cmd: procStop, resp: errCh, arg: *req})
	if err != nil {
		return err
	}

	for {
		select {
		case err := <-errCh:
			if err != nil {
				return err
			}
		case *resp = <-procsCh:
			if checkStatus(*resp, ProcStopping|ProcStopped|ProcExited|ProcFatal, *req) {
				return nil
			}
		case <-ctx.Done():
			slog.Warn("TaskCmd.Stop: Timeout: fail to stopping processes")
			return fmt.Errorf("TaskCmd.Stop: Timeout: fail to stopping processes")
		}
	}

	return nil

}

func (t *TaskCmd) Restart(req *CmdArg, resp *[]Proc) error {

	err := t.Stop(req, resp)
	if err != nil {
		return err
	}

	return t.Start(req, resp)
}

func (t *TaskCmd) Status(req *CmdArg, resp *[]Proc) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	procsCh := make(chan []Proc, 1)
	errCh := make(chan error)

	t.c.Subscribe(procsCh)
	defer t.c.Unsubscribe(procsCh)

	err := t.c.SendCmd(procCmd{cmd: procGetStatus, resp: errCh, arg: *req})
	if err != nil {
		return err
	}

	for {
		select {
		case err := <-errCh:
			if err != nil {
				return err
			}
		case *resp = <-procsCh:
			return nil
		case <-ctx.Done():
			slog.Warn("TaskCmd.Status: Timeout: fail to get Status")
			return fmt.Errorf("TaskCmd.Status: Timeout: fail to get Status")
		}
	}

	return nil
}
