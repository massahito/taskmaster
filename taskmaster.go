package taskmaster

import (
	"log/slog"
	"syscall"
	"time"
)

type Config struct {
	Socket  Socket
	Cluster map[string]Group
	Logging Logging
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

type Logging struct {
	Path   string
	Size   uint
	Backup uint
	Level  slog.Level
}
