#!/usr/bin/env python3
"""Bind Archive Center server process groups to the POSIX launcher lifetime."""

import os
import signal
import sys
import time


SHUTDOWN_TIMEOUT_SECONDS = 10.0
POLL_INTERVAL_SECONDS = 0.1


def read_process_groups(path):
    groups = set()
    try:
        with open(path, "r", encoding="utf-8") as handle:
            for raw_line in handle:
                try:
                    process_group = int(raw_line.strip())
                except ValueError:
                    continue
                if process_group > 1:
                    groups.add(process_group)
    except FileNotFoundError:
        pass
    return groups


def process_group_exists(process_group):
    try:
        os.killpg(process_group, 0)
        return True
    except ProcessLookupError:
        return False
    except PermissionError:
        return True


def signal_process_group(process_group, requested_signal):
    try:
        os.killpg(process_group, requested_signal)
    except ProcessLookupError:
        pass


def stop_process_groups(pid_file, known_groups=None, retired_groups=None):
    known = set() if known_groups is None else set(known_groups)
    retired = set() if retired_groups is None else set(retired_groups)
    term_sent = set()
    deadline = time.monotonic() + SHUTDOWN_TIMEOUT_SECONDS

    while True:
        for process_group in read_process_groups(pid_file):
            if process_group not in retired:
                known.add(process_group)

        active = set()
        for process_group in known:
            if process_group_exists(process_group):
                active.add(process_group)
            else:
                retired.add(process_group)
        known = active

        for process_group in known - term_sent:
            signal_process_group(process_group, signal.SIGTERM)
            term_sent.add(process_group)

        if not known:
            return
        if time.monotonic() >= deadline:
            break
        time.sleep(POLL_INTERVAL_SECONDS)

    for process_group in known:
        if process_group_exists(process_group):
            signal_process_group(process_group, signal.SIGKILL)


def run_managed_process(pid_file, parent_pid, command):
    if os.getppid() != parent_pid:
        return 143

    os.setsid()
    descriptor = os.open(pid_file, os.O_WRONLY | os.O_CREAT | os.O_APPEND, 0o600)
    try:
        os.write(descriptor, (str(os.getpid()) + "\n").encode("ascii"))
        os.fsync(descriptor)
    finally:
        os.close(descriptor)

    if os.getppid() != parent_pid:
        return 143

    for requested_signal in (signal.SIGHUP, signal.SIGINT, signal.SIGTERM):
        signal.signal(requested_signal, signal.SIG_DFL)
    os.execvp(command[0], command)
    return 127


def watch_parent(parent_pid, pid_file, stop_file, done_file):
    for requested_signal in (signal.SIGHUP, signal.SIGINT, signal.SIGTERM):
        signal.signal(requested_signal, signal.SIG_IGN)

    known = set()
    retired = set()
    while os.getppid() == parent_pid and not os.path.exists(stop_file):
        for process_group in read_process_groups(pid_file):
            if process_group not in known and process_group not in retired:
                if process_group_exists(process_group):
                    known.add(process_group)
                else:
                    retired.add(process_group)
        for process_group in tuple(known):
            if not process_group_exists(process_group):
                known.remove(process_group)
                retired.add(process_group)
        time.sleep(POLL_INTERVAL_SECONDS)

    stop_process_groups(pid_file, known, retired)
    try:
        with open(done_file, "w", encoding="ascii") as handle:
            handle.write("stopped\n")
    except OSError:
        pass
    return 0


def main():
    if len(sys.argv) < 2:
        return 64
    mode = sys.argv[1]
    if mode == "run" and len(sys.argv) >= 5:
        return run_managed_process(sys.argv[2], int(sys.argv[3]), sys.argv[4:])
    if mode == "watch" and len(sys.argv) == 6:
        return watch_parent(int(sys.argv[2]), sys.argv[3], sys.argv[4], sys.argv[5])
    if mode == "stop" and len(sys.argv) == 3:
        stop_process_groups(sys.argv[2])
        return 0
    return 64


if __name__ == "__main__":
    sys.exit(main())
