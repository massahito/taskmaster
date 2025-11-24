# taskmaster

## Introduction

taskmaster is a lightweight process control system designed to start, stop, and reliably supervise long-running applications. Similar in spirit to tools like [supervisor](https://supervisord.org/), taskmaster provides a centralized and consistent way to manage multiple processes, ensuring they remain running, behave predictably, and produce organized logs.

The primary goals of taskmaster are:

1. Reliable process execution
    - Start and monitor resident processes with predictable behavior
    - Easily control groups of processes (start/stop/restart)
    - Automatically restart processes when they exit unexpectedly
2. Centralized process management
    - Provide a unified shell-like interface for managing multiple services such as web servers, workers, and scheduled jobs.
    - Allow controlled process management through user-level privileges
    - Maintain consistent configuration and lifecycle handling across all managed processes
3. Integrated log handling
    - Capture each process's stdout and stderr into dedicated log files
    - Provide basic log rotation to prevent unbounded log growth

## When taskmaster is useful

Use taskmaster when you need to:

- Run multiple background services on a single machine
- Ensure processes automatically restart on failure
- Manage application components without relying on container orchestration tools
- Keep logs organized without an external logging system
