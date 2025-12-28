package config

import "os"

type YamlConfig struct {
	Cluster map[string]YamlGroup `yaml:"cluster"`
	Socket  YamlSocket           `yaml:"socket"`
	Log     YamlLog              `yaml:log`
}

type YamlSocket struct {
	Path string      `yaml:"path"`
	Mode os.FileMode `yaml:"mode"`
	UID  int         `yaml:"UID"`
	GID  int         `yaml:"GID"`
}

func (s *YamlSocket) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type rawSocket YamlSocket
	raw := rawSocket{
		Path: "/tmp/taskmaster.sock",
		Mode: 0660,
		UID:  os.Getuid(),
		GID:  os.Getgid(),
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}

	*s = YamlSocket(raw)
	return nil
}

type YamlLog struct {
	Path  string `yaml:"path"`
	Level string `yaml:"level"`
}

func (l *YamlLog) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type rawLog YamlLog
	raw := rawLog{
		Path:  "/tmp/taskmaster.log",
		Level: "INFO",
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}

	*l = YamlLog(raw)
	return nil
}

type YamlGroup struct {
	Priority uint8                  `yaml:"priority"`
	Programs map[string]YamlProgram `yaml:"programs"`
}

func (g *YamlGroup) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type rawGroup YamlGroup
	raw := rawGroup{
		Priority: 255,
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}

	*g = YamlGroup(raw)
	return nil
}

type YamlProgram struct {
	Cmd          []string            `yaml:"cmd"`
	Numproc      uint8               `yaml:"numproc"`
	Priority     uint8               `yaml:"priority"`
	Directory    string              `yaml:"directory"`
	Autostart    bool                `yaml:"autostart"`
	Autorestart  string              `yaml:"autorestart"`
	Startsecs    uint8               `yaml:"startsecs"`
	Startretries uint8               `yaml:"startretries"`
	Stopwaitsecs uint8               `yaml:"stopwaitsecs"`
	Stopasgroup  bool                `yaml:"stopasgroup"`
	Stopsignal   string              `yaml:"stopsignal"`
	Exitcodes    []uint8             `yaml:"exitcodes"`
	Environment  []map[string]string `yaml:"environment"`
	Umask        int                 `yaml:"umask"`
	Stdout       string              `yaml:"stdout"`
	Stderr       string              `yaml:"stderr"`
}

func (p *YamlProgram) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type rawProgram YamlProgram
	raw := rawProgram{
		Numproc:      1,
		Priority:     255,
		Directory:    "/tmp",
		Autostart:    false,
		Autorestart:  "unexpected",
		Startsecs:    1,
		Startretries: 3,
		Stopwaitsecs: 10,
		Stopasgroup:  false,
		Stopsignal:   "TERM",
		Exitcodes:    []uint8{0},
		Umask:        022,
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}

	*p = YamlProgram(raw)
	return nil
}

const SampleConfig string = `
# ------------------------------------------------------------------------------
# Socket configuration
# Configures the UNIX domain socket used by the taskserver
# ------------------------------------------------------------------------------
socket:
  path: /tmp/taskmaster.sock        # Filesystem path of the socket file
  mode: 660                         # Octal file mode applied to the socket
  GID: 1000                         # Group ID owning the socket
  UID: 1000                         # User ID owning the socket

# ------------------------------------------------------------------------------
# Cluster configuration
# Defines process groups and their managed programs
# Cluster must have at least one group in its mapping
# Group must have at lease one program in its programs mapping
# ------------------------------------------------------------------------------
cluster:

  sleepGroup:
    priority: 1                     # Group scheduling priority.
    programs:
      sleepProgram1:
        cmd: ["/bin/sleep", "30"]   # Command and arguments
        numproc: 1                  # Number of process instances
        priority: 255               # Program priority within the group
        directory: /tmp             # Working directory
        autostart: 'No'             # Start automatically on launch
        autorestart: unexpected     # Restart only on unexpected exit
        startsecs: 1                # Seconds process must stay running
        startretries: 3             # Retry count on startup failure
        stopwaitsecs: 10            # Grace period before forced stop
        stopasgroup: 'No'           # Stop process group together
        exitcodes:                  # Expected exit codes
          - 0
        stopsignal: TERM            # Signal used to stop the process
        environment:                # Environment variables
          - HOME: /root
        umask: 022                  # File creation mask
        stdout: /tmp/slpout(%d).log # Output file for standard output
        stderr: /tmp/slperr(%d).log # Output file for standard error

  logGroup:
    priority: 120
    programs:
      tail:
        cmd: ["/usr/bin/tail", "-f", "/tmp/taskmaster.log"]
        numproc: 10
        priority: 1
        directory: /tmp
        autostart: 'Yes'
        autorestart: always
        startsecs: 2
        startretries: 5
        stopwaitsecs: 15
        stopasgroup: 'No'
        exitcodes:
          - 0
        stopsignal: TERM
        environment:
          - SHELL: /bin/sh
          - HOME: /root
        umask: 022
        stdout: /tmp/tailout(%d).log
        stderr: /tmp/tailerr(%d).log

# ------------------------------------------------------------------------------
# Logging configuration
# Controls taskserver's internal logging
# ------------------------------------------------------------------------------
log:
  path: /tmp/taskmaster.log         # Log file location
  level: INFO                       # Log verbosity level

`
