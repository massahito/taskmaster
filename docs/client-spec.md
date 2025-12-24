# spec

## Terminology

- `TaskClient`: client-side process/program for taskmaster. The target of this spec.
- `TaskServer`: server-side process/program for taskmaster.
- `Cluster`: group of several `Group`s. Can be specified in configuration.
- `Group:` group of several `Program`s. Can be specified in Cluster's value as associative array in configuration.
- `Program`: details of execution. Can be specified in `Group`'s value as associative array in configuration.
- `Process`: unit of execution. created the same number of `Numproc` in configuration for each `Program`.
- `Gname`: `Group`'s name.
- `Pname`: `Program`'s name.
- `ID`: `Process`'s ID.
- `PID`: OS process ID in unix meaning.

## Basic requirements

- The binary should be in `client/taskclient`

- TaskClient must:

  - have shell-like/REPL interface
  - accept user's command
  - output the results of user's command
  - communicate TaskServer via RPC protocol or Signal

- TaskClient must be able to:
  - take the configuration from command line option `-c`
  - determine the UNIX socket path which is used to communicate TaskServer from passed configuration

## Command syntax

TaskClient must have shell-like/REPL interface and accept user's command.

This is an EBNF of command line syntax.

```
command      = proc-command, " ", scope | serv-command;
proc-command = "status" | "start" | "stop" | "restart" | "update";
serv-command = "reload" | "shutdown";
scope        = Gname, [":", Pname, [":", ID]] | "all";
```

## Command Semantics

TaskClient must communicate TaskServer via RPC protocol or Signal to execute user's command.

The set of method TaskClient should call is in [cmd.go](https://github.com/massahito/taskmaster/blob/main/cmd.go) of `taskmaster` module.

`serv-command` may use [CmdArg](https://github.com/massahito/taskmaster/blob/dff96770bde87c8a0f48e8bf39eba0e50cc94ace/cmd.go#L32-L53) for specifying the command scope.

`CmdArg` should be set:

- `Gname` and `Pname` are empty string, and `ID` is negative for "all" scope
- `Gname` is set, `Pname` are empty string, and `ID` is negative for `Group` scope
- `Gname` and `Pname` are set, and `ID` is negative for `Program` scope
- `Gname` and `Pname` are set, and `ID` is non-negative for `Process` scope

```
Scope     Syntax example              CmdArg (Gname, Pname, ID)
---------------------------------------------------------------
all       status all                                ("", "", -1)
group     status groupname                          ("groupname", "", -1)
program   status groupname:programname              ("groupname", "programname", -1)
process   status groupname:programname:0            ("groupname", "programname", 0)
```

- status

  - must call [TaskCmd.Status](https://github.com/massahito/taskmaster/blob/dff96770bde87c8a0f48e8bf39eba0e50cc94ace/cmd.go#L178-L192)
  - CmdArg must be set properly
  - status must output the status lines for all processes matching the requested scope, scoped client-side

- start

  - must call [TaskCmd.Start](https://github.com/massahito/taskmaster/blob/dff96770bde87c8a0f48e8bf39eba0e50cc94ace/cmd.go#L124-L138)
  - CmdArg must be set properly
  - status must output the status lines for all processes matching the requested scope, scoped client-side

- stop

  - must call [TaskCmd.Stop](https://github.com/massahito/taskmaster/blob/dff96770bde87c8a0f48e8bf39eba0e50cc94ace/cmd.go#L140-L154)
  - CmdArg must be set properly
  - status must output the status lines for all processes matching the requested scope, scoped client-side

- restart

  - must call [TaskCmd.Restart](https://github.com/massahito/taskmaster/blob/dff96770bde87c8a0f48e8bf39eba0e50cc94ace/cmd.go#L156-L176)
  - CmdArg must be set properly
  - status must output the status lines for all processes matching the requested scope, scoped client-side

- update

  - must call [TaskCmd.Update](https://github.com/massahito/taskmaster/blob/dff96770bde87c8a0f48e8bf39eba0e50cc94ace/cmd.go#L194-L235)
  - CmdArg must be set properly
  - status must output the status lines for all processes matching the requested scope, scoped client-side

- reload

  - must call [TaskCmd.Pid](https://github.com/massahito/taskmaster/blob/dff96770bde87c8a0f48e8bf39eba0e50cc94ace/cmd.go#L108-L122) to get TaskServer's process id
  - must send `SIGHUP` signal to TaskServer's process id
  - must output nothing

- shutdown
  - must call [TaskCmd.Pid](https://github.com/massahito/taskmaster/blob/dff96770bde87c8a0f48e8bf39eba0e50cc94ace/cmd.go#L108-L122) to get TaskServer's process id
  - must send `SIGTERM` signal to TaskServer's process id
  - must output nothing

## Output format

`serv-command` in this client (reload, shutdown) do not use CmdArg for scoping; they always act on the TaskServer itself.

`proc-command` described in `Command syntax` section should output the status of Processes. Since TaskServer always response the status of all processes, scoping should be done in TaskClient.

At least, `proc-command` should output: 

- [`Gname`, `Pname`, `ID`](https://github.com/massahito/taskmaster/blob/dff96770bde87c8a0f48e8bf39eba0e50cc94ace/proc.go#L77-L83) in some format
- [Status](https://github.com/massahito/taskmaster/blob/dff96770bde87c8a0f48e8bf39eba0e50cc94ace/proc.go#L92) of `Process`
- [Pid](https://github.com/massahito/taskmaster/blob/dff96770bde87c8a0f48e8bf39eba0e50cc94ace/proc.go#L98) of `Process`
- Execution time

Examle of output

```bash
thegroupname:theprogramname:0   RUNNING   pid 1445, 0:01:00
thegroupname:theprogramname:1   RUNNING   pid 1446, 0:01:00
thegroupname:theprogramname:2   RUNNING   pid 1447, 0:01:00
```

