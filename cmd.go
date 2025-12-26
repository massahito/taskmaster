package taskmaster

import (
	"fmt"
	"os"
	"sync"
)

// TaskCmd provides an interface for interacting with a Controller.
//
// It is typically exposed as a net/rpc object.
//
// The associated Controller must be started via Controller.Start
// before any TaskCmd methods are called.
type TaskCmd struct {
	caller      *caller
	cfg         Config
	pid         int
	isBusy      bool
	isBusyMutex sync.Mutex
}

// NewTaskCmd returns a new TaskCmd instance bound to the given Controller.
func NewTaskCmd(cfg Config, ctrl *Controller) *TaskCmd {
	return &TaskCmd{
		caller: &caller{ctrl: ctrl},
		cfg:    cfg,
		pid:    os.Getpid(),
	}
}

// CmdArg selects the target of a TaskCmd operation.
//
// A CmdArg may target all processes, a group, a program, or a single process
// instance, depending on which fields are set.
//
// Selection rules:
//   - All processes:   Gname == "", Pname == "", ID < 0
//   - A group:         Gname != "", Pname == "", ID < 0
//   - A program:       Gname != "", Pname != "", ID < 0
//   - A process:       Gname != "", Pname != "", ID >= 0 (instance index)
//
// By convention, ID < 0 means "all instances".
type CmdArg struct {
	// Gname is the group name. An empty string matches all groups.
	Gname string

	// Pname is the program name within the group. An empty string matches all programs.
	Pname string

	// ID selects a specific process instance. A negative value matches all instances.
	ID int
}

func isGeneralCmd(arg CmdArg) bool {
	if arg.Gname == "" && arg.Pname == "" && arg.ID < 0 {
		return true
	}
	return false
}

func isGroupCmd(arg CmdArg) bool {
	if arg.Gname != "" && arg.Pname == "" && arg.ID < 0 {
		return true
	}
	return false
}

func isProgCmd(arg CmdArg) bool {
	if arg.Gname != "" && arg.Pname != "" && arg.ID < 0 {
		return true
	}
	return false
}

func isProcCmd(arg CmdArg) bool {
	if arg.Gname != "" && arg.Pname != "" && 0 <= arg.ID {
		return true
	}
	return false
}

type cmd uint32

const (
	procAutoStart cmd = iota
	procGetStatus
	procStart
	procStop
	procShutDown
	procCreate
	procDelete
	procExit
	procStartCheck
	procStopCheck
	procFail
)

func (c cmd) IsUserCmd() bool {
	switch c {
	default:
		return false
	case procAutoStart, procGetStatus, procStart, procStop, procShutDown, procCreate, procDelete:
		return true
	}
}

type procCmd struct {
	pid   int
	cmd   cmd
	arg   CmdArg
	cfg   Config
	state os.ProcessState
	resp  chan<- error
}

// Pid returns the PID of the TaskCmd process.
//
// It is intended for net/rpc clients. The req argument is ignored.
// If another exclusive operation is in progress, Pid returns an error.
func (t *TaskCmd) Pid(_ *CmdArg, pid *int) error {
	t.isBusyMutex.Lock()
	if t.isBusy {
		t.isBusyMutex.Unlock()
		return fmt.Errorf("the TaskServer is busy")
	}
	t.isBusyMutex.Unlock()

	*pid = t.pid
	return nil
}

// Start starts the processes selected by req and stores the resulting
// process snapshot in resp.
//
// If the TaskCmd is busy, Start returns an error.
// Selection rules are defined by CmdArg.
func (t *TaskCmd) Start(req *CmdArg, resp *Procs) error {
	t.isBusyMutex.Lock()
	if t.isBusy {
		t.isBusyMutex.Unlock()
		return fmt.Errorf("the TaskServer is busy")
	}
	t.isBusyMutex.Unlock()

	return t.caller.start(req, resp)
}

// Stop stops the processes selected by req and stores the resulting
// process snapshot in resp.
//
// If the TaskCmd is busy, Stop returns an error.
// Selection rules are defined by CmdArg.
func (t *TaskCmd) Stop(req *CmdArg, resp *Procs) error {
	t.isBusyMutex.Lock()
	if t.isBusy {
		t.isBusyMutex.Unlock()
		return fmt.Errorf("the TaskServer is busy")
	}
	t.isBusyMutex.Unlock()

	return t.caller.stop(req, resp)
}

// Restart stops and then starts the processes selected by req, storing the
// resulting process snapshot in resp.
//
// If stopping fails, Restart returns that error and does not attempt to start.
//
// If the TaskCmd is busy, Restart returns an error.
// Selection rules are defined by CmdArg.
func (t *TaskCmd) Restart(req *CmdArg, resp *Procs) error {
	t.isBusyMutex.Lock()
	if t.isBusy {
		t.isBusyMutex.Unlock()
		return fmt.Errorf("the TaskServer is busy")
	}
	t.isBusyMutex.Unlock()

	err := t.caller.halt(req, resp)
	if err != nil {
		return err
	}
	return t.caller.start(req, resp)
}

// Status reports the current status of processes selected by req and stores a
// snapshot in resp.
//
// If the TaskCmd is busy, Status returns an error.
// Selection rules are defined by CmdArg.
func (t *TaskCmd) Status(req *CmdArg, resp *Procs) error {
	t.isBusyMutex.Lock()
	if t.isBusy {
		t.isBusyMutex.Unlock()
		return fmt.Errorf("the TaskServer is busy")
	}
	t.isBusyMutex.Unlock()

	return t.caller.status(req, resp)
}

// Update reloads the configuration from the config file and applies it to the
// Controller, scoped by req, storing a resulting process snapshot in resp.
//
// Update is an exclusive operation: while it runs, other TaskCmd methods
// return an error indicating the server is busy.
//
// Selection rules are defined by CmdArg.
func (t *TaskCmd) Update(req *CmdArg, resp *Procs) error {
	t.isBusyMutex.Lock()
	if t.isBusy {
		t.isBusyMutex.Unlock()
		return fmt.Errorf("the TaskServer is busy")
	}
	t.isBusy = true
	t.isBusyMutex.Unlock()

	defer func() {
		t.isBusyMutex.Lock()
		t.isBusy = false
		t.isBusyMutex.Unlock()
	}()

	cfg, err := Parse(t.cfg.Path)
	if err != nil {
		return err
	}

	oldCfg := t.cfg
	newCfg, err := updateConfig(oldCfg, cfg, req)
	if err != nil {
		return err
	}

	err = t.caller.update(oldCfg, newCfg, req, resp)
	if err != nil {
		return err
	}

	t.cfg = newCfg

	return nil
}

func updateConfig(oldCfg, newCfg Config, arg *CmdArg) (Config, error) {
	dst, err := oldCfg.Clone()
	if err != nil {
		return oldCfg, err
	}

	if isGeneralCmd(*arg) {
		dst.Cluster = newCfg.Cluster
		return dst, nil
	} else if isGroupCmd(*arg) {
		return updateGroupConfig(dst, newCfg, arg)
	} else if isProgCmd(*arg) {
		return updateProgConfig(dst, newCfg, arg)
	}
	return Config{}, fmt.Errorf("invalid argument scope")
}

func updateGroupConfig(oldCfg, newCfg Config, req *CmdArg) (Config, error) {
	gname := req.Gname
	_, oldOk := oldCfg.Cluster[gname]
	newGrp, newOk := newCfg.Cluster[gname]

	switch {
	case oldOk && newOk:
		oldCfg.Cluster[gname] = newGrp
		return oldCfg, nil
	case oldOk && !newOk:
		delete(oldCfg.Cluster, gname)
		return oldCfg, nil
	case !oldOk && newOk:
		oldCfg.Cluster[gname] = newGrp
		return oldCfg, nil
	}

	return oldCfg, fmt.Errorf("can't find Group named %s", gname)
}

func updateProgConfig(oldCfg, newCfg Config, req *CmdArg) (Config, error) {
	gname := req.Gname
	pname := req.Pname
	oldGrp, oldOk := oldCfg.Cluster[gname]
	newGrp, newOk := newCfg.Cluster[gname]

	switch {
	case oldOk && newOk:
		_, oldOk := oldGrp.Progs[pname]
		newProg, newOk := newGrp.Progs[pname]
		switch {
		case oldOk && newOk:
			oldCfg.Cluster[gname].Progs[pname] = newProg
			return oldCfg, nil
		case oldOk && !newOk:
			delete(oldCfg.Cluster[gname].Progs, pname)
			return oldCfg, nil
		case !oldOk && newOk:
			oldCfg.Cluster[gname].Progs[pname] = newProg
			return oldCfg, nil
		}
		return oldCfg, fmt.Errorf("can't find the Program named %s", pname)
	case oldOk && !newOk:
		_, oldOk := oldGrp.Progs[pname]
		if !oldOk {
			return oldCfg, fmt.Errorf("can't find the Program named %s", pname)
		}
		delete(oldCfg.Cluster[gname].Progs, pname)
		return oldCfg, nil
	case !oldOk && newOk:
		newProg, newOk := newGrp.Progs[pname]
		if !newOk {
			return oldCfg, fmt.Errorf("can't find the Program named %s", pname)
		}
		newGrp.Progs = map[string]Program{pname: newProg}
		oldCfg.Cluster[gname] = newGrp
		return oldCfg, nil
	}

	return oldCfg, fmt.Errorf("can't find the Group named %s", gname)
}
