#!/bin/bash

export PROJECT_NAME="golang_template"
export IMAGE_NAME="golang_template:latest"
export COMPOSE_FILE="deploy/docker-compose.yml"

export JENKINS_ENV_FILE="/var/lib/jenkins/api/golang-template/golang_template.env"
export JENKINS_SECRETS_FILE="/var/lib/jenkins/api/secrets.yml"
export JENKINS_WORKSPACE_PATTERN="golang-template"
