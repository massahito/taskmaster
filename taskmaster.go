package taskmaster

import (
	"encoding/json"
	"log/slog"
	"os"
	"reflect"
	"syscall"
	"time"
)

// Config contains settings loaded from a configuration file.
//
// It is typically created by [Parse] and passed to [NewController] and [TaskCmd].
type Config struct {
	// Path is the path to the configuration file used to build this Config.
	Path string

	// Socket configures the Controller's UNIX domain socket.
	Socket Socket

	// Cluster maps group names to their configuration.
	Cluster map[string]Group

	// Log configures logging output and verbosity.
	Log Log
}

// Clone clones the identical Config.
// It performs a deep copy of all relevant fields.
func (c Config) Clone() (ret Config, err error) {
	data, err := json.Marshal(c)
	if err != nil {
		return c, err
	}

	err = json.Unmarshal(data, &ret)
	if err != nil {
		return c, err
	}

	return ret, nil
}

// Socket contains configuration for a UNIX domain socket.
type Socket struct {
	// Path is the filesystem path of the socket file.
	Path string

	// Mode is the file mode to apply to the socket file (e.g. 0600).
	Mode os.FileMode

	// UID is the user ID that should own the socket file.
	UID int

	// GID is the group ID that should own the socket file.
	GID int
}

// Group groups Programs and defines their startup priority relative to other groups.
type Group struct {
	// Priority orders groups during startup. Lower values start first.
	Priority uint8

	// Progs maps program names to their Program configuration.
	Progs map[string]Program
}

// RestartPolicy defines how the Controller reacts when a process exits.
type RestartPolicy uint

const (
	_ RestartPolicy = iota
	// RestartAlways restarts the process regardless of exit code.
	RestartAlways
	// RestartNever never restarts the process after it exits.
	RestartNever
	// RestartUnexpected restarts the process only if its exit code
	// is not listed in Program.Exitcodes.
	RestartUnexpected
)

// Program describes how a program should be started and supervised.
//
// A single Program definition may result in multiple OS processes
// based on the Numproc setting.
type Program struct {
	// Cmd is the command to execute. It is typically an argv-style slice,
	// where Cmd[0] is the executable and the remaining elements are arguments.
	Cmd []string

	// Numproc is the number of OS processes to start for this Program.
	Numproc uint8

	// Priority is used to order program startup within a group.
	// Programs with smaller Priority values are started first.
	Priority uint8

	// Directory is the working directory for the started process.
	Directory string

	// Environment is the list of environment variables for the process,
	// in the form "KEY=value".
	Environment []string

	// Umask sets the process umask.
	Umask int

	// Stdout is the temprate file path where the process's standard output is written.
	Stdout string
	// Stderr is the temprate file path where the process's standard error is written.
	Stderr string

	// Autostart indicates whether the program is started automatically
	// when the Controller starts.
	Autostart bool

	// Startretries is the maximum number of retry attempts when the program
	// fails to start.
	Startretries uint8

	// Stopasgroup indicates whether the Controller sends the stop signal
	// to the entire process group instead of only the main process.
	Stopasgroup bool

	// Exitcodes is the list of acceptable exit codes for the program.
	// This list is only used when Autorestart is RestartUnexpected.
	Exitcodes []uint8

	// Startsecs is the duration the Controller waits after starting a program
	// before considering it successfully running.
	Startsecs time.Duration

	// Stopwaitsecs is the maximum duration the Controller waits after sending
	// the stop signal before forcefully terminating the program.
	Stopwaitsecs time.Duration

	// Autorestart defines how the Controller reacts when a program exits.
	Autorestart RestartPolicy

	// Stopsignal is the signal used to stop the program.
	Stopsignal syscall.Signal
}

// IsSame reports whether other has the same configuration as p.
// It performs a deep comparison of all relevant fields.
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

// Log configures logging output and verbosity.
// It is typically applied via SetLogger.
type Log struct {
	// Path is the file path where log output is written.
	// If empty, logs may be written to standard error.
	Path string

	// Level is the minimum severity level that will be logged.
	Level slog.Level
}
