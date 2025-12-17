package taskmaster

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type caller struct {
	ctrl *controller
}

func (c *caller) status(req *CmdArg, resp *[]Proc) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	procsCh := make(chan []Proc, 1)
	errCh := make(chan error)

	c.ctrl.Subscribe(procsCh)
	defer c.ctrl.Unsubscribe(procsCh)

	err := c.ctrl.SendCmd(procCmd{cmd: procGetStatus, resp: errCh, arg: *req})
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

func (c *caller) start(req *CmdArg, resp *[]Proc) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	procsCh := make(chan []Proc, 1)
	errCh := make(chan error)

	c.ctrl.Subscribe(procsCh)
	defer c.ctrl.Unsubscribe(procsCh)

	err := c.ctrl.SendCmd(procCmd{cmd: procStart, resp: errCh, arg: *req})
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
			if checkStatus(*resp, ProcStarting|ProcRunning|ProcBackoff, *req) {
				return nil
			}
		case <-ctx.Done():
			slog.Warn("TaskCmd.Start: Timeout: fail to start processes")
			return fmt.Errorf("TaskCmd.Start: Timeout: fail to start processes")
		}
	}

	return nil
}

func (c *caller) stop(req *CmdArg, resp *[]Proc) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	procsCh := make(chan []Proc, 1)
	errCh := make(chan error)

	c.ctrl.Subscribe(procsCh)
	defer c.ctrl.Unsubscribe(procsCh)

	err := c.ctrl.SendCmd(procCmd{cmd: procStop, resp: errCh, arg: *req})
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
			slog.Warn("TaskCmd.Stop: Timeout: fail to stop processes")
			return fmt.Errorf("TaskCmd.Stop: Timeout: fail to stop processes")
		}
	}

	return nil
}

func (c *caller) halt(req *CmdArg, resp *[]Proc) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	procsCh := make(chan []Proc, 1)
	errCh := make(chan error)

	c.ctrl.Subscribe(procsCh)
	defer c.ctrl.Unsubscribe(procsCh)

	err := c.ctrl.SendCmd(procCmd{cmd: procStop, resp: errCh, arg: *req})
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
			if checkStatus(*resp, ProcStopped|ProcExited|ProcFatal, *req) {
				return nil
			}
		case <-ctx.Done():
			slog.Warn("TaskCmd.StopAndWait: Timeout: fail to stop processes")
			return fmt.Errorf("TaskCmd.StopAndWait: Timeout: fail to stop processes")
		}
	}

	return nil
}

func (c *caller) update(oldCfg, newCfg Config, req *CmdArg, resp *[]Proc) error {
	// stop
	for gname, oldGrp := range oldCfg.Cluster {
		arg := &CmdArg{Gname: gname, Id: -1}
		newGrp, ok := newCfg.Cluster[gname]
		if !ok {
			err := c.halt(arg, resp)
			if err != nil {
				return err
			}
			c.deleteProc(arg, resp)
		}

		for pname, oldProg := range oldGrp.Progs {
			newProg, ok := newGrp.Progs[pname]
			if !ok || !isSameProgram(newProg, oldProg) {
				arg.Pname = pname
				err := c.halt(arg, resp)
				if err != nil {
					return err
				}
				err = c.deleteProc(arg, resp)
				if err != nil {
					return err
				}
			}
		}

	}

	// start
	for gname, newGrp := range newCfg.Cluster {
		arg := &CmdArg{Gname: gname, Id: -1}
		oldGrp, ok := oldCfg.Cluster[gname]
		if !ok {
			err := c.createProc(oldCfg, arg, resp)
			if err != nil {
				return err
			}
			c.autoStart(arg, resp)
		}

		for pname, newProg := range newGrp.Progs {
			oldProg, ok := oldGrp.Progs[pname]
			if !ok || !isSameProgram(newProg, oldProg) {
				arg.Pname = pname
				err := c.createProc(newCfg, arg, resp)
				if err != nil {
					return err
				}
				err = c.autoStart(arg, resp)
				if err != nil {
					return err
				}
			}
		}

	}

	return nil

}

func (c *caller) createProc(cfg Config, req *CmdArg, resp *[]Proc) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	procsCh := make(chan []Proc, 1)
	errCh := make(chan error)

	c.ctrl.Subscribe(procsCh)
	defer c.ctrl.Unsubscribe(procsCh)

	err := c.ctrl.SendCmd(procCmd{cmd: procCreate, resp: errCh, arg: *req, cfg: cfg})
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
			slog.Warn("TaskCmd.createProc: Timeout: fail to get Status")
			return fmt.Errorf("TaskCmd.createProc: Timeout: fail to get Status")
		}
	}

	return nil
}

func (c *caller) deleteProc(req *CmdArg, resp *[]Proc) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	procsCh := make(chan []Proc, 1)
	errCh := make(chan error)

	c.ctrl.Subscribe(procsCh)
	defer c.ctrl.Unsubscribe(procsCh)

	err := c.ctrl.SendCmd(procCmd{cmd: procDelete, resp: errCh, arg: *req})
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
			slog.Warn("TaskCmd.deleteProcs: Timeout: fail to get Status")
			return fmt.Errorf("TaskCmd.deleteProcs: Timeout: fail to get Status")
		}
	}

	return nil
}

func (c *caller) autoStart(req *CmdArg, resp *[]Proc) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	procsCh := make(chan []Proc, 1)
	errCh := make(chan error)

	c.ctrl.Subscribe(procsCh)
	defer c.ctrl.Unsubscribe(procsCh)

	err := c.ctrl.SendCmd(procCmd{cmd: procAutoStart, resp: errCh, arg: *req})
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
			if checkStatus(*resp, ProcStarting|ProcRunning|ProcBackoff, *req) {
				return nil
			}
		case <-ctx.Done():
			slog.Warn("TaskCmd.autoStart: Timeout: fail to start processes")
			return fmt.Errorf("TaskCmd.autoStart: Timeout: fail to start processes")
		}
	}

	return nil
}
