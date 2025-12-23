package taskmaster

import (
	"fmt"
	"os"
	"sync"
)

type TaskCmd struct {
	caller      *caller
	cfg         Config
	pid         int
	isBusy      bool
	isBusyMutex sync.Mutex
}

func NewTaskCmd(cfg Config, ctrl *controller) *TaskCmd {
	return &TaskCmd{
		caller: &caller{ctrl: ctrl},
		cfg:    cfg,
		pid:    os.Getpid(),
	}
}

type CmdArg struct {
	Gname string
	Pname string
	ID    int
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

type procCmd struct {
	pid   int
	cmd   cmd
	arg   CmdArg
	cfg   Config
	state os.ProcessState
	resp  chan<- error
}

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

func (t *TaskCmd) Start(req *CmdArg, resp *Procs) error {
	t.isBusyMutex.Lock()
	if t.isBusy {
		t.isBusyMutex.Unlock()
		return fmt.Errorf("the TaskServer is busy")
	}
	t.isBusyMutex.Unlock()

	return t.caller.start(req, resp)
}

func (t *TaskCmd) Stop(req *CmdArg, resp *Procs) error {
	t.isBusyMutex.Lock()
	if t.isBusy {
		t.isBusyMutex.Unlock()
		return fmt.Errorf("the TaskServer is busy")
	}
	t.isBusyMutex.Unlock()

	return t.caller.stop(req, resp)
}

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

func (t *TaskCmd) Status(req *CmdArg, resp *Procs) error {
	t.isBusyMutex.Lock()
	if t.isBusy {
		t.isBusyMutex.Unlock()
		return fmt.Errorf("the TaskServer is busy")
	}
	t.isBusyMutex.Unlock()

	return t.caller.status(req, resp)
}

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
	dst, err := cloneConfig(oldCfg)
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
