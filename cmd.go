package taskmaster

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"
)

type TaskCmd struct {
	c *controller
}

func NewTaskCmd(c *controller) *TaskCmd {
	return &TaskCmd{
		c: c,
	}
}

type CmdArg struct {
	Gname string
	Pname string
	Id    uint8
}

type cmd uint8

const (
	procStartUp cmd = iota
	procGetStatus
	procShutDown
	procExit
	procStartCheck
	procStopCheck
)

type procCmd struct {
	pid   int
	cmd   cmd
	arg   CmdArg
	state os.ProcessState
	resp  chan<- error
}

func (t *TaskCmd) Return(req *int, resp *int) error {
	*resp = *req
	return nil
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

func (t *TaskCmd) Shutdown(req *CmdArg, resp *[]Proc) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	procsCh := make(chan []Proc, 1)
	errCh := make(chan error)

	t.c.Subscribe(procsCh)
	defer t.c.Unsubscribe(procsCh)

	err := t.c.SendCmd(procCmd{cmd: procShutDown, resp: errCh, arg: *req})
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
			if isAllProcDead(*resp) {
				return nil
			}
		case <-ctx.Done():
			slog.Warn("fail to stop gracefully")
			return fmt.Errorf("fail to stop gracefully")
		}
	}
}
