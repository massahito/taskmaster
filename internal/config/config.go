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
	Priority uint                   `yaml:"priority"`
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
