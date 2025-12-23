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

func (c *caller) status(req *CmdArg, resp *Procs) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	procsCh := make(chan Procs, 1)
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
				slog.Error("caller.status", "error", err.Error())
				return err
			}
		case *resp = <-procsCh:
			return nil
		case <-ctx.Done():
			slog.Warn("caller.status: Timeout: fail to get Status")
			return fmt.Errorf("Status: Timeout: fail to get Status")
		}
	}

	return nil
}

func (c *caller) start(req *CmdArg, resp *Procs) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	procsCh := make(chan Procs, 1)
	errCh := make(chan error)

	c.ctrl.Subscribe(procsCh)
	defer c.ctrl.Unsubscribe(procsCh)

	err := c.ctrl.SendCmd(procCmd{cmd: procStart, resp: errCh, arg: *req})
	if err != nil {
		slog.Error("caller.start", "error", err.Error())
		return err
	}

	for {
		select {
		case err := <-errCh:
			if err != nil {
				slog.Error("caller.start", "error", err.Error())
				return err
			}
		case *resp = <-procsCh:
			if (*resp).CheckStatus(ProcStarting|ProcRunning|ProcBackoff, *req) {
				return nil
			}
		case <-ctx.Done():
			slog.Warn("caller.start: Timeout: fail to start processes")
			return fmt.Errorf("Start: Timeout: fail to start processes")
		}
	}

	return nil
}

func (c *caller) stop(req *CmdArg, resp *Procs) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	procsCh := make(chan Procs, 1)
	errCh := make(chan error)

	c.ctrl.Subscribe(procsCh)
	defer c.ctrl.Unsubscribe(procsCh)

	err := c.ctrl.SendCmd(procCmd{cmd: procStop, resp: errCh, arg: *req})
	if err != nil {
		slog.Error("caller.stop", "error", err.Error())
		return err
	}

	for {
		select {
		case err := <-errCh:
			if err != nil {
				slog.Error("caller.stop", "error", err.Error())
				return err
			}
		case *resp = <-procsCh:
			if (*resp).CheckStatus(ProcStopping|ProcStopped|ProcExited|ProcFatal, *req) {
				return nil
			}
		case <-ctx.Done():
			slog.Warn("caller.stop: Timeout: fail to stop processes")
			return fmt.Errorf("Stop: Timeout: fail to stop processes")
		}
	}

	return nil
}

func (c *caller) halt(req *CmdArg, resp *Procs) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	procsCh := make(chan Procs, 1)
	errCh := make(chan error)

	c.ctrl.Subscribe(procsCh)
	defer c.ctrl.Unsubscribe(procsCh)

	err := c.ctrl.SendCmd(procCmd{cmd: procStop, resp: errCh, arg: *req})
	if err != nil {
		slog.Error("caller.halt", "error", err.Error())
		return err
	}

	for {
		select {
		case err := <-errCh:
			if err != nil {
				slog.Error("caller.halt", "error", err.Error())
				return err
			}
		case *resp = <-procsCh:
			if (*resp).CheckStatus(ProcStopped|ProcExited|ProcFatal, *req) {
				return nil
			}
		case <-ctx.Done():
			slog.Warn("caller.halt: Timeout: fail to stop processes")
			return fmt.Errorf("Halt: Timeout: fail to stop processes")
		}
	}

	return nil
}

func (c *caller) update(oldCfg, newCfg Config, req *CmdArg, resp *Procs) error {
	// stop
	for gname, oldGrp := range oldCfg.Cluster {
		arg := &CmdArg{Gname: gname, ID: -1}
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
		arg := &CmdArg{Gname: gname, ID: -1}
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

func (c *caller) createProc(cfg Config, req *CmdArg, resp *Procs) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	procsCh := make(chan Procs, 1)
	errCh := make(chan error)

	c.ctrl.Subscribe(procsCh)
	defer c.ctrl.Unsubscribe(procsCh)

	err := c.ctrl.SendCmd(procCmd{cmd: procCreate, resp: errCh, arg: *req, cfg: cfg})
	if err != nil {
		slog.Error("caller.createProc", "error", err.Error())
		return err
	}

	for {
		select {
		case err := <-errCh:
			if err != nil {
				slog.Error("caller.createProc", "error", err.Error())
				return err
			}
		case *resp = <-procsCh:
			return nil
		case <-ctx.Done():
			slog.Warn("caller.createProc: Timeout: fail to create Processes")
			return fmt.Errorf("CreateProc: Timeout: fail to create Processes")
		}
	}

	return nil
}

func (c *caller) deleteProc(req *CmdArg, resp *Procs) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	procsCh := make(chan Procs, 1)
	errCh := make(chan error)

	c.ctrl.Subscribe(procsCh)
	defer c.ctrl.Unsubscribe(procsCh)

	err := c.ctrl.SendCmd(procCmd{cmd: procDelete, resp: errCh, arg: *req})
	if err != nil {
		slog.Error("caller.deleteProc", "error", err.Error())
		return err
	}

	for {
		select {
		case err := <-errCh:
			if err != nil {
				slog.Error("caller.deleteProc", "error", err.Error())
				return err
			}
		case *resp = <-procsCh:
			return nil
		case <-ctx.Done():
			slog.Warn("caller.deleteProcs: Timeout: fail to delete Processes")
			return fmt.Errorf("DeleteProcs: Timeout: fail to delete Processes")
		}
	}

	return nil
}

func (c *caller) autoStart(req *CmdArg, resp *Procs) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	procsCh := make(chan Procs, 1)
	errCh := make(chan error)

	c.ctrl.Subscribe(procsCh)
	defer c.ctrl.Unsubscribe(procsCh)

	err := c.ctrl.SendCmd(procCmd{cmd: procAutoStart, resp: errCh, arg: *req})
	if err != nil {
		slog.Error("caller.autoStart", "error", err.Error())
		return err
	}

	for {
		select {
		case err := <-errCh:
			if err != nil {
				slog.Error("caller.autoStart", "error", err.Error())
				return err
			}
		case *resp = <-procsCh:
			if (*resp).CheckStatus(ProcStarting|ProcRunning|ProcBackoff, *req) {
				return nil
			}
		case <-ctx.Done():
			slog.Warn("caller.autoStart: Timeout: fail to start processes")
			return fmt.Errorf("AutoStart: Timeout: fail to start processes")
		}
	}

	return nil
}
