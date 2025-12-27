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
socket: # socket configures the Controller's UNIX domain socket.
  path: /tmp/taskmaster.sock # path is the filesystem path of the socket file.
  mode: 660 # mode is the file mode to apply to the socket file (e.g. 0600).
  GID: 1000 # GID is the group ID that should own the socket file.
  UID: 1000 # UID is the user ID that should own the socket file.
 
cluster:
  sleepGroup: # group name.
    priority: 1 # group priority
    programs: # program directive
      sleepProgram1: # 
        cmd: ["/bin/sleep", "30"]
        numproc: 1
        priority: 255
        directory: /tmp
        autostart: 'No'
        autorestart: unexpected
        startsecs: 1
        startretries: 3
        stopwaitsecs: 10
        stopasgroup: 'No'
        exitcodes:
          - 0
        stopsignal: TERM
        environment:
          - HOME: /root
        umask: 022
      sleepProgram2:
        cmd: ["/bin/sleep", "15"]
        numproc: 5
        stdout: /tmp/sleepProgram2_stdout(%d).log
        stderr: /tmp/sleepProgram2_stderr(%d).log
  logGroup:
    priority: 120
    programs:
      tail:
        cmd: ["/usr/bin/tail", "-f", "/var/log/messages"]
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
        stdout: /tmp/tail_stdout(%d).log
        stderr: /tmp/tail_stderr(%d).log
log:
  path: /tmp/taskmaster.log
  level: INFO
`
