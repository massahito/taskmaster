package taskmaster

import (
	"encoding/json"
	"log/slog"
	"reflect"
	"syscall"
	"time"
)

type Config struct {
	Path    string
	Socket  Socket
	Cluster map[string]Group
	Log     Log
}

type Socket struct {
	Path  string
	Chmod uint32
	Chown string
}

type Group struct {
	Priority uint
	Progs    map[string]Program
}

type RestartPolicy uint

const (
	RestartAlways RestartPolicy = iota
	RestartNever
	RestartUnexpected
)

type Program struct {
	Cmd          []string
	Numproc      uint8
	Priority     uint8
	Directory    string
	Autostart    bool
	Startretries uint8
	Stopasgroup  bool
	Exitcodes    []uint8
	Startsecs    time.Duration
	Stopwaitsecs time.Duration
	Autorestart  RestartPolicy
	Stopsignal   syscall.Signal
	Environment  []string
	Umask        int
	Stdout       string
	Stderr       string
}

func (p Program) IsSame(other Program) bool {
	return (p.Numproc == other.Numproc &&
		p.Priority == other.Priority &&
		p.Directory == other.Directory &&
		p.Autostart == other.Autostart &&
		p.Startretries == other.Startretries &&
		p.Stopasgroup == other.Stopasgroup &&
		p.Startsecs == other.Startsecs &&
		p.Stopwaitsecs == other.Stopwaitsecs &&
		p.Autorestart == other.Autorestart &&
		p.Stopsignal == other.Stopsignal &&
		p.Umask == other.Umask &&
		p.Stdout == other.Stdout &&
		p.Stderr == other.Stderr &&
		reflect.DeepEqual(p.Cmd, other.Cmd) &&
		reflect.DeepEqual(p.Exitcodes, other.Exitcodes) &&
		reflect.DeepEqual(p.Environment, other.Environment))
}

type Log struct {
	Path   string
	Size   uint
	Backup uint
	Level  slog.Level
}

func cloneConfig(cfg Config) (ret Config, err error) {
	data, err := json.Marshal(cfg)
	if err != nil {
		return cfg, err
	}

	err = json.Unmarshal(data, &ret)
	if err != nil {
		return cfg, err
	}

	return ret, nil
}
