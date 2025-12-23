package taskmaster

import (
	"strconv"
	"strings"
	"time"
)

type ProcStatus int32

const (
	ProcStopped ProcStatus = 1 << iota
	ProcStopping
	ProcStarting
	ProcRunning
	ProcBackoff
	ProcExited
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

func (status ProcStatus) IsStartable() bool {
	return status == ProcExited || status == ProcStopped || status == ProcFatal
}

func (status ProcStatus) IsStoppable() bool {
	return status == ProcRunning || status == ProcStarting
}

func (status ProcStatus) IsDead() bool {
	return status == ProcExited || status == ProcStopped || status == ProcFatal
}

type Proc struct {
	PID    int
	Retry  uint8
	Gname  string
	Pname  string
	ID     uint8
	Stdout string
	Stderr string
	Status ProcStatus
	Time   time.Time
	Prog   Program
}

type procRef *Proc
type procRefs []*Proc
type Procs []Proc

// TODO fix shallow copy of p.Prog
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

func (p Procs) IsAllProcDead() bool {
	for _, ps := range p {
		if !ps.Status.IsDead() {
			return false
		}
	}
	return true
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
