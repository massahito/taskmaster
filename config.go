package taskmaster

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/massahito/taskmaster/internal/config"
	"gopkg.in/yaml.v3"
)

func PrintExamle() {
	fmt.Fprint(os.Stdout, config.SampleConfig)
}

// Parse parses a configuration file from path and create [Config].
//
// the configuration file specified path should be formatted in yaml.
func Parse(path string) (Config, error) {
	var cfg config.YamlConfig

	file, err := os.OpenFile(path, os.O_RDONLY, 0444)
	if err != nil {
		slog.Error("config.Parse: os.OpenFile", "error", err.Error())
		return Config{}, configError("fail to open %s", path)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	err = decoder.Decode(&cfg)
	if err != nil {
		slog.Error("config.Parse: yaml.Decoder.Decode", "error", err.Error())
		return Config{}, configError(err.Error())
	}

	return parseConfig(path, cfg)
}

func parseConfig(path string, cfg config.YamlConfig) (Config, error) {
	socket, err := parseSocket(cfg.Socket)
	if err != nil {
		slog.Error("config.Parse: parseSocket", "error", err.Error())
		return Config{}, err
	}

	cluster, err := parseCluster(cfg.Cluster)
	if err != nil {
		slog.Error("config.Parse: parseCluster", "error", err.Error())
		return Config{}, err
	}

	log, err := parseLog(cfg.Log)
	if err != nil {
		slog.Error("config.Parse: parseLog", "error", err.Error())
		return Config{}, err
	}

	return Config{
		Path:    path,
		Socket:  socket,
		Cluster: cluster,
		Log:     log,
	}, nil
}

func parseSocket(skt config.YamlSocket) (Socket, error) {
	return Socket{
		Path: skt.Path,
		Mode: skt.Mode,
		UID:  skt.UID,
		GID:  skt.GID,
	}, nil
}

func parseLog(log config.YamlLog) (Log, error) {
	level, err := parseLogLevel(log.Level)
	if err != nil {
		return Log{}, err
	}

	if log.Path == "" {
		log.Path = "/tmp/taskmaster.log"
	}

	return Log{
		Path:  log.Path,
		Level: level,
	}, nil
}

func parseLogLevel(level string) (slog.Level, error) {
	switch level {
	case "":
		return slog.LevelInfo, nil
	case "DEBUG":
		return slog.LevelDebug, nil
	case "INFO":
		return slog.LevelInfo, nil
	case "WARN":
		return slog.LevelWarn, nil
	case "ERROR":
		return slog.LevelError, nil
	default:
		return slog.LevelError, configError("Invalid value \"%s\" for 'log.level'. Expected one of: DEBUG, INFO, WARN, ERROR.", level)
	}
}

func parseCluster(grps map[string]config.YamlGroup) (map[string]Group, error) {
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

func parsePrograms(progs map[string]config.YamlProgram) (map[string]Program, error) {
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

func parseProgram(prog config.YamlProgram) (Program, error) {
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
