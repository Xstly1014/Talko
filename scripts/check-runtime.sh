#!/bin/sh
command -v docker || true
command -v podman || true
ls -l /var/run/docker.sock 2>/dev/null || true
free -m
