# spec

## Terminology

- `TaskClient`: the client-side process/program for taskmaster. The target of this spec.
- `TaskServer`: the server-side process/program for taskmaster.
- `Cluster`: The group of several `Group`s. Can be specified in configuration.
- `Group:` The group of several `Program`s. Can be specified in Cluster's value as associative array in configuration.
- `Program`: The details of execution. Can be specified in `Group`'s value as associative array in configuration.
- `Process`: The unit of execution. created the same number of `Numproc` in configuration for each `Program`.
- `Gname`: `Group`'s name.
- `Pname`: `Program`'s name.
- `Id`: `Process`'s id.
- `Pid`: process id in unix meaning.

## Basic requirements

- The binary should be in `client/taskclient`.

- TaskClient must:

  - have shell-like/REPL interface.
  - accept user's command.
  - output the results of user's command.
  - communicate TaskServer via RPC protocol or Signal.

- TaskClient must be able to:
  - take the configuration from command line option `-c`.
  - determine the UNIX socket path which is used to communicate TaskServer from passed configuration.

## Command syntax

TaskClient must have shell-like/REPL interface and accept user's command.

This is an EBNF of command line syntax.

```
command      = proc-command, " ", scope | serv-command;
proc-command = "status" | "start" | "stop" | "restart";
serv-command = "reload" | "shutdown";
scope        = Gname, [":", Pname, [":", Id]] | "all";
```

## Command Semantics

TaskClient must communicate TaskServer via RPC protocol or Signal to execute user's command.

The set of method TaskClient should call is in [cmd.go](https://github.com/massahito/taskmaster/blob/main/cmd.go) of `taskmaster` module.

`serv-command` may use [CmdArg](https://github.com/massahito/taskmaster/blob/1da2d4ba1dbc4b022f1455c8a57663df02b4a355/cmd.go#L23-L27) for specifying the command scope.

`CmdArg` should be set:

- `Gname` and `Pname` are empty string, and `Id` is negative for "all" scope. 
- `Gname` is set, `Pname` are empty string, and `Id` is negative for `Group` scope. 
- `Gname` and `Pname` are set, and `Id` is negative for `Program` scope. 
- `Gname` and `Pname` are set, and `Id` is non-negative for `Process` scope.

```
Scope     Syntax example              CmdArg (Gname, Pname, Id)
---------------------------------------------------------------
all       status all                                ("", "", -1)
group     status groupname                          ("groupname", "", -1)
program   status groupname:programname              ("groupname", "programname", -1)
process   status groupname:programname:0            ("groupname", "programname", 0)
```

- status

  - must call [TaskCmd.Status](https://github.com/massahito/taskmaster/blob/1da2d4ba1dbc4b022f1455c8a57663df02b4a355/cmd.go#L153).
  - CmdArg must be set properly.
  - status MUST output the status lines for all processes matching the requested scope, scoped client-side.

- start

  - must call [TaskCmd.Start](https://github.com/massahito/taskmaster/blob/1da2d4ba1dbc4b022f1455c8a57663df02b4a355/cmd.go#L83).
  - CmdArg must be set properly.
  - status MUST output the status lines for all processes matching the requested scope, scoped client-side.

- stop

  - must call [TaskCmd.Stop](https://github.com/massahito/taskmaster/blob/1da2d4ba1dbc4b022f1455c8a57663df02b4a355/cmd.go#L118).
  - CmdArg must be set properly.
  - status MUST output the status lines for all processes matching the requested scope, scoped client-side.

- restart

  - must call [TaskCmd.Restart](https://github.com/massahito/taskmaster/blob/4c3465b7b7d11cdbe2bd82f2302ccc0e707b8152/cmd.go#L153).
  - CmdArg must be set properly.
  - status MUST output the status lines for all processes matching the requested scope, scoped client-side.

- reload

  - must call [TaskCmd.Pid](https://github.com/massahito/taskmaster/blob/1da2d4ba1dbc4b022f1455c8a57663df02b4a355/cmd.go#L78) to get TaskServer's process id.
  - must send `SIGHUP` signal to TaskServer's process id.
  - must output nothing.

- shutdown
  - must call [TaskCmd.Pid](https://github.com/massahito/taskmaster/blob/1da2d4ba1dbc4b022f1455c8a57663df02b4a355/cmd.go#L78) to get TaskServer's process id.
  - must send `SIGTERM` signal to TaskServer's process id.
  - must output nothing.

## Output format

`serv-command` in this client (reload, shutdown) do not use CmdArg for scoping; they always act on the TaskServer itself.

`proc-command` described in `Command syntax` section should output the status of Processes. Since TaskServer always response the status of all processes, scoping should be done in TaskClient.

At least, `proc-command` should output: 

- [`Gname`, `Pname`, `Id`](https://github.com/massahito/taskmaster/blob/1da2d4ba1dbc4b022f1455c8a57663df02b4a355/proc.go#L12-L14) in some format. 
- [Status](https://github.com/massahito/taskmaster/blob/1da2d4ba1dbc4b022f1455c8a57663df02b4a355/proc.go#L17) of `Process` 
- [Pid](https://github.com/massahito/taskmaster/blob/1da2d4ba1dbc4b022f1455c8a57663df02b4a355/proc.go#L10) of `Process`. 
- Execution time.

Examle of output

```bash
thegroupname:theprogramname:0   RUNNING   pid 1445, 0:01:00
thegroupname:theprogramname:1   RUNNING   pid 1446, 0:01:00
thegroupname:theprogramname:2   RUNNING   pid 1447, 0:01:00
```

