#!/bin/bash

export PROJECT_NAME="golang_template_infra"
export COMPOSE_DIR="infra"

# Host-side env for compose (ports, passwords). Keep next to app env on the agent.
export JENKINS_ENV_FILE="/var/lib/jenkins/api/golang-template/golang_template_infra.env"
export JENKINS_WORKSPACE_PATTERN="golang-template"
