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

type Log struct {
	Path   string
	Size   uint
	Backup uint
	Level  slog.Level
}

func isSameProgram(a, b Program) bool {
	return (a.Numproc == b.Numproc &&
		a.Priority == b.Priority &&
		a.Directory == b.Directory &&
		a.Autostart == b.Autostart &&
		a.Startretries == b.Startretries &&
		a.Stopasgroup == b.Stopasgroup &&
		a.Startsecs == b.Startsecs &&
		a.Stopwaitsecs == b.Stopwaitsecs &&
		a.Autorestart == b.Autorestart &&
		a.Stopsignal == b.Stopsignal &&
		a.Umask == b.Umask &&
		a.Stdout == b.Stdout &&
		a.Stderr == b.Stderr &&
		reflect.DeepEqual(a.Cmd, b.Cmd) &&
		reflect.DeepEqual(a.Exitcodes, b.Exitcodes) &&
		reflect.DeepEqual(a.Environment, b.Environment))
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
