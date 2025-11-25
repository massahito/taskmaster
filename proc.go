package taskmaster

import "time"

type Proc struct {
	Pid    int
	Retry  uint8
	Gname  string
	Pname  string
	Id     uint8
	Status ProcStatus
	Time   time.Time
	Prog   Program
}

type ProcStatus uint8

const (
	ProcStopped ProcStatus = iota
	ProcStopping
	ProcStarting
	ProcRunning
	ProcBackoff
	ProcExited
	ProcFatal
)

func copyProcs(procs []*Proc) []Proc {
	ret := []Proc{}

	for _, ps := range procs {
		ret = append(ret, *ps)
	}
	return ret
}

func genProcs(groups map[string]Group) []*Proc {

	procs := []*Proc{}

	for gname, group := range groups {
		for pname, prog := range group.Progs {
			for i := uint8(0); i < prog.Numproc; i++ {
				p := &Proc{
					Pid:    0,
					Retry:  0,
					Gname:  gname,
					Pname:  pname,
					Id:     i,
					Status: ProcStopped,
					Prog:   prog,
				}
				procs = append(procs, p)
			}
		}
	}
	return procs
}

func searchProc(procs []*Proc, pid int) *Proc {
	for _, proc := range procs {
		if proc.Pid == pid {
			return proc
		}
	}

	return nil
}

func getGroupProcs(procs []*Proc, gname string) []*Proc {
	ret := []*Proc{}
	for _, ps := range procs {
		if ps.Gname == gname {
			ret = append(ret, ps)
		}
	}
	return ret
}

func getProgProcs(procs []*Proc, gname, pname string) []*Proc {
	ret := []*Proc{}
	for _, ps := range procs {
		if ps.Gname == gname && ps.Pname == pname {
			ret = append(ret, ps)
		}
	}
	return ret
}

func getIDProc(procs []*Proc, gname, pname string, id uint8) *Proc {
	for _, ps := range procs {
		if ps.Gname == gname && ps.Pname == pname && ps.Id == id {
			return ps
		}
	}
	return nil
}

func isStartable(status ProcStatus) bool {
	return status == ProcExited || status == ProcStopped || status == ProcFatal
}

func isStoppable(status ProcStatus) bool {
	return status == ProcRunning || status == ProcStarting
}

func isDead(status ProcStatus) bool {
	return status == ProcExited || status == ProcStopped || status == ProcFatal
}

func isContainBackoff(procs []*Proc) bool {
	for _, ps := range procs {
		if ps.Status == ProcBackoff {
			return true
		}
	}
	return false
}

func isAllProcDead(procs []Proc) bool {
	for _, ps := range procs {
		if !isDead(ps.Status) {
			return false
		}
	}
	return true
}
