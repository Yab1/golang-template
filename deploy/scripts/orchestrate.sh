#!/bin/bash
# Docker orchestration — start, stop, restart, status (not a deploy).

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/load-config.sh"

COMMAND="${1:-start}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}$1${NC}"; }
log_success() { echo -e "${GREEN}$1${NC}"; }
log_warning() { echo -e "${YELLOW}$1${NC}"; }
log_error() { echo -e "${RED}$1${NC}" >&2; }

docker_compose() {
    docker compose "$@" 2> >(grep -vE 'The "[^"]*" variable is not set|Docker Compose is configured to build using Bake' >&2)
}

usage() {
    echo "Usage: $0 [command]"
    echo
    echo "Commands:"
    echo "  start    Start services"
    echo "  stop     Stop services"
    echo "  restart  Restart services"
    echo "  status   Show service status"
}

check_docker() {
    if ! docker info >/dev/null 2>&1; then
        log_error "Docker is not running"
        exit 1
    fi
}

check_compose_file() {
    if [[ ! -f "$COMPOSE_FILE" ]]; then
        log_error "Compose file not found: $COMPOSE_FILE"
        exit 1
    fi
}

prepare_compose_env() {
    export DOCKER_BUILD_CONTEXT="$PROJECT_DIR"
    export DOCKER_BUILDKIT=1
    export COMPOSE_DOCKER_CLI_BUILD=1
}

start() {
    cd "$PROJECT_DIR"
    prepare_compose_env
    log_info "Building images..."
    docker_compose -f "$COMPOSE_FILE" build --parallel golang_template_api_blue
    log_info "Starting services..."
    docker_compose -f "$COMPOSE_FILE" up -d
    docker_compose -f "$COMPOSE_FILE" ps
}

stop() {
    cd "$PROJECT_DIR"
    prepare_compose_env
    docker_compose -f "$COMPOSE_FILE" down --remove-orphans
}

restart() {
    stop
    start
}

ps() {
    cd "$PROJECT_DIR"
    prepare_compose_env
    docker_compose -f "$COMPOSE_FILE" ps
}

main() {
    [[ "$1" == "-h" || "$1" == "--help" ]] && { usage; exit 0; }

    check_docker
    check_compose_file
    load_env_if_present || true

    case "$COMMAND" in
        "start"|"up") start ;;
        "stop"|"down") stop ;;
        "restart") restart ;;
        "status"|"ps") ps ;;
        *)
            log_error "Unknown command: $COMMAND"
            usage
            exit 1
            ;;
    esac
}

main "$@"
