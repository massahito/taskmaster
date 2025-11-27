package taskmaster

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"strings"
	"syscall"
	"time"
)

type YamlConfig struct {
	Cluster map[string]YamlGroup `yaml:"cluster"`
	Socket  YamlSocket           `yaml:"socket"`
}

type YamlSocket struct {
	Path string `yaml:"path"`
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

func Parse(path string) (Config, error) {
	var cfg YamlConfig

	file, err := os.OpenFile(path, os.O_RDONLY, 0444)
	if err != nil {
		return Config{}, configError("fail to open %s", path)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	err = decoder.Decode(&cfg)
	if err != nil {
		return Config{}, configError(err.Error())
	}

	return parseConfig(cfg)
}

func parseConfig(cfg YamlConfig) (Config, error) {
	socket, err := parseSocket(cfg.Socket)
	if err != nil {
		return Config{}, err
	}

	cluster, err := parseCluster(cfg.Cluster)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Socket:  socket,
		Cluster: cluster,
		Logging: Logging{Path: "/tmp/taskmaster.log"},
	}, nil
}

func parseSocket(skt YamlSocket) (Socket, error) {
	if skt.Path == "" {
		skt.Path = "/tmp/taskmaster.sock"
	}
	return Socket{
		Path: skt.Path,
	}, nil
}

func parseCluster(grps map[string]YamlGroup) (map[string]Group, error) {
	if len(grps) == 0 {
		return map[string]Group{}, configError("cluster must have at least one group.")
	}

	ret := map[string]Group{}
	for key, grp := range grps {
		progs, err := parsePrograms(grp.Programs)
		if err != nil {
			return nil, err
		}

		ret[key] = Group{
			Priority: grp.Priority,
			Progs:    progs,
		}
	}

	return ret, nil
}

func parsePrograms(progs map[string]YamlProgram) (map[string]Program, error) {
	if len(progs) == 0 {
		return nil, configError("group must have at least one program.")
	}

	ret := map[string]Program{}
	for key, prog := range progs {
		prog, err := parseProgram(prog)
		if err != nil {
			return nil, err
		}
		ret[key] = prog
	}

	return ret, nil
}

func parseProgram(prog YamlProgram) (Program, error) {
	if len(prog.Cmd) == 0 {
		return Program{}, configError("you have to specify at least cmd.")
	}
	if prog.Numproc == 0 {
		return Program{}, configError("numproc must be at least one.")
	}
	policy, err := parseRestartPolicy(prog.Autorestart)
	if err != nil {
		return Program{}, err
	}
	sig, err := parseSignal(prog.Stopsignal)
	if err != nil {
		return Program{}, err
	}
	if prog.Numproc != 1 && prog.Stdout != "" && strings.Count(prog.Stdout, "(%d)") != 1 {
		return Program{}, configError("program more than 1 process must have one '(%%d)' in stdout")
	}
	if prog.Numproc != 1 && prog.Stderr != "" && strings.Count(prog.Stderr, "(%d)") != 1 {
		return Program{}, configError("program more than 1 process must have one '(%%d)' in stderr")
	}

	env := parseEnv(prog.Environment)

	p := Program{
		Cmd:          prog.Cmd,
		Numproc:      prog.Numproc,
		Priority:     prog.Priority,
		Directory:    prog.Directory,
		Autostart:    prog.Autostart,
		Autorestart:  policy,
		Startretries: prog.Startretries,
		Stopasgroup:  prog.Stopasgroup,
		Startsecs:    time.Duration(prog.Startsecs) * time.Second,
		Stopwaitsecs: time.Duration(prog.Stopwaitsecs) * time.Second,
		Stopsignal:   sig,
		Exitcodes:    prog.Exitcodes,
		Environment:  env,
		Umask:        prog.Umask,
		Stdout:       prog.Stdout,
		Stderr:       prog.Stderr,
	}

	return p, nil
}

func parseEnv(envs []map[string]string) []string {
	ret := []string{}
	for _, env := range envs {
		for key, value := range env {
			ret = append(ret, fmt.Sprintf("%s=%s", key, value))
		}
	}

	return ret
}

func parseRestartPolicy(str string) (RestartPolicy, error) {
	switch str {
	case "always":
		return RestartAlways, nil
	case "never":
		return RestartNever, nil
	case "unexpected":
		return RestartUnexpected, nil
	default:
		return 0, configError("autorestart should be one of always, never, unexpected.")
	}
}

func parseSignal(str string) (syscall.Signal, error) {
	switch str {
	case "TERM":
		return syscall.SIGTERM, nil
	case "HUP":
		return syscall.SIGHUP, nil
	case "INT":
		return syscall.SIGINT, nil
	case "QUIT":
		return syscall.SIGQUIT, nil
	case "KILL":
		return syscall.SIGKILL, nil
	case "USR1":
		return syscall.SIGUSR1, nil
	case "USR2":
		return syscall.SIGUSR2, nil
	default:
		return syscall.Signal(0), configError("stopsignal should be one of TERM, HUP, INT, QUIT, KILL, USR1, USR2.")
	}
}

func configError(format string, a ...any) error {
	return fmt.Errorf("config: "+format, a...)
}
