package taskmaster

import (
	"strconv"
	"strings"
	"time"
)

// ProcStatus represents the lifecycle state of a managed process.
type ProcStatus int32

const (
	_ ProcStatus = 1 << iota

	// ProcStopped indicates the process is currently stopped.
	ProcStopped

	// ProcStopping indicates a stop signal has been sent and the process is shutting down.
	ProcStopping

	// ProcStarting indicates the process has been started but is not yet considered running.
	ProcStarting

	// ProcRunning indicates the process is running successfully.
	ProcRunning

	// ProcBackoff indicates the process is temporarily prevented from restarting
	// This can be seen when the starting process has exited before elapsing Startsecs.
	ProcBackoff

	// ProcExited indicates the process has exited.
	ProcExited

	// ProcFatal indicates the process is in a permanent failure state
	ProcFatal
)

func (status ProcStatus) String() string {
	switch status {
	default:
		return "Unknown"
	case ProcStopped:
		return "Stopped"
	case ProcStopping:
		return "Stopping"
	case ProcStarting:
		return "Starting"
	case ProcRunning:
		return "Running"
	case ProcBackoff:
		return "Backoff"
	case ProcExited:
		return "Exited"
	case ProcFatal:
		return "Fatal"
	}
}

// IsStartable reports whether a start action is valid for this state.
func (status ProcStatus) IsStartable() bool {
	return status == ProcExited || status == ProcStopped || status == ProcFatal
}

// IsStartable reports whether a stop action is valid for this state.
func (status ProcStatus) IsStoppable() bool {
	return status == ProcRunning || status == ProcStarting
}

// IsDead reports whether the process's status can be changed without user interaction.
func (status ProcStatus) IsDead() bool {
	return status == ProcExited || status == ProcStopped || status == ProcFatal
}

// Proc describes a single managed process instance.
type Proc struct {
	// Gname is Group name of this process.
	Gname string

	// Pname is Program name of this process.
	Pname string

	// ID is the instance index within the program
	ID uint8

	// Stdout is the file path where the process's standard output is written.
	Stdout string

	// Stderr is the file path where the process's standard error is written.
	Stderr string

	// Status is the current lifecycle state of the process
	Status ProcStatus

	// Prog is the Program configuration used for this process.
	Prog Program

	// PID is the OS process ID. It is 0 when the process is not running.
	PID int

	// Retry is the number of start retry attempts made for this instance.
	// This value is reset after this process is dead.
	Retry uint8

	// Time is the timestamp of the most recent process's state transition (start/stop/exit).
	Time time.Time
}

// Procs is a snapshot list of process instances.
type Procs []Proc

// CheckStatus reports whether all processes selected by req have a status
// that intersects the provided status mask.
//
// If no processes match req, CheckStatus returns true.
//
// For detailed selection rules, see [CmdArg].
func (p Procs) CheckStatus(status ProcStatus, req CmdArg) bool {

	for _, ps := range p {
		if req.Gname == "" || ps.Gname == req.Gname {
			if req.Pname == "" || ps.Pname == req.Pname {
				if req.ID < 0 || int(ps.ID) == req.ID {
					if (ps.Status & status) == 0 {
						return false
					}
				}
			}
		}
	}

	return true
}

// IsAllProcDead reports all the managed processes are dead.
func (p Procs) IsAllProcDead() bool {
	for _, ps := range p {
		if !ps.Status.IsDead() {
			return false
		}
	}
	return true
}

type procRef *Proc
type procRefs []*Proc

// Procs returns a snapshot copy of the referenced processes.
// Note: Program fields inside Proc are copied shallowly.
func (p procRefs) Procs() (ret Procs) {
	for _, ps := range p {
		ret = append(ret, *ps)
	}
	return ret
}

func (p procRefs) searchByPID(pid int) procRef {
	for _, proc := range p {
		if proc.PID == pid {
			return proc
		}
	}

	return nil
}

func (p procRefs) filterByGroup(gname string) (ret procRefs) {
	for _, ps := range p {
		if ps.Gname == gname {
			ret = append(ret, ps)
		}
	}
	return ret
}

func (p procRefs) filterByProg(gname, pname string) (ret procRefs) {
	for _, ps := range p {
		if ps.Gname == gname && ps.Pname == pname {
			ret = append(ret, ps)
		}
	}
	return ret
}

func (p procRefs) filterByID(gname, pname string, id uint8) (ret procRef) {
	for _, ps := range p {
		if ps.Gname == gname && ps.Pname == pname && ps.ID == id {
			return ps
		}
	}
	return nil
}

func replaceUint(str, key string, n uint8) string {
	rep := strconv.FormatUint(uint64(n), 10)
	return strings.Replace(str, key, rep, -1)
}

func buildProcRefFromGroup(groups map[string]Group) (procs procRefs) {

	for gname, group := range groups {
		for pname, prog := range group.Progs {
			procs = append(procs, buildProcRef(gname, pname, prog)...)
		}
	}

	return
}

func buildProcRef(gname, pname string, prog Program) (procs procRefs) {
	Stdout := prog.Stdout
	Stderr := prog.Stderr
	for i := uint8(0); i < prog.Numproc; i++ {
		if prog.Numproc != 1 && prog.Stdout != "" {
			Stdout = replaceUint(prog.Stdout, "(%d)", i)
		}
		if prog.Numproc != 1 && prog.Stderr != "" {
			Stderr = replaceUint(prog.Stderr, "(%d)", i)
		}
		p := &Proc{
			PID:    0,
			Retry:  0,
			Gname:  gname,
			Pname:  pname,
			ID:     i,
			Status: ProcStopped,
			Prog:   prog,
			Stdout: Stdout,
			Stderr: Stderr,
		}
		procs = append(procs, p)
	}
	return
}
