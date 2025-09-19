#!/bin/bash

# EMQX MQTT Broker Control Script
# This script manages an EMQX broker running in a Docker container

set -e

# Configuration
CONTAINER_NAME="emqx-broker"
IMAGE_NAME="emqx/emqx:latest"
MQTT_PORT="1883"
DASHBOARD_PORT="18083"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DATA_VOLUME="emqx-data"
RULES_FILE="$SCRIPT_DIR/acl.conf"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Docker is running
check_docker() {
    if ! docker info >/dev/null 2>&1; then
        log_error "Docker is not running. Please start Docker and try again."
        exit 1
    fi
}

# Check if container exists
container_exists() {
    docker ps -a --format "table {{.Names}}" | grep -q "^${CONTAINER_NAME}$"
}

# Check if container is running
container_running() {
    docker ps --format "table {{.Names}}" | grep -q "^${CONTAINER_NAME}$"
}

# Create data volume if it doesn't exist
create_volume() {
    if ! docker volume ls --format "table {{.Name}}" | grep -q "^${DATA_VOLUME}$"; then
        log_info "Creating data volume: ${DATA_VOLUME}"
        docker volume create "${DATA_VOLUME}"
        log_success "Data volume created successfully"
    fi
}

# Start the EMQX broker
start_broker() {
    log_info "Starting EMQX MQTT broker..."
    
    check_docker
    create_volume
    
    if container_running; then
        log_warning "Container ${CONTAINER_NAME} is already running"
        return 0
    fi
    
    if container_exists; then
        log_info "Starting existing container..."
        docker start "${CONTAINER_NAME}"
    else
        log_info "Creating and starting new container..."
        docker run -d \
            --name "${CONTAINER_NAME}" \
            -p "${MQTT_PORT}:1883" \
            -p "${DASHBOARD_PORT}:18083" \
            -v "${DATA_VOLUME}:/opt/emqx/data" \
            -v "${RULES_FILE}:/opt/emqx/data/authz/acl.conf" \
            --restart unless-stopped \
            "${IMAGE_NAME}"
    fi
    
    # Wait a moment for the container to start
    sleep 3
    
    if container_running; then
        log_success "EMQX broker started successfully"
        log_info "MQTT broker available at: localhost:${MQTT_PORT}"
        log_info "Dashboard available at: http://localhost:${DASHBOARD_PORT}"
        log_info "Default credentials: admin/public"
    else
        log_error "Failed to start EMQX broker"
        exit 1
    fi
}

# Stop the EMQX broker
stop_broker() {
    log_info "Stopping EMQX MQTT broker..."
    
    check_docker
    
    if ! container_exists; then
        log_warning "Container ${CONTAINER_NAME} does not exist"
        return 0
    fi
    
    if ! container_running; then
        log_warning "Container ${CONTAINER_NAME} is not running"
        return 0
    fi
    
    docker stop "${CONTAINER_NAME}"
    log_success "EMQX broker stopped successfully"
}

# Restart the EMQX broker
restart_broker() {
    log_info "Restarting EMQX MQTT broker..."
    stop_broker
    sleep 2
    start_broker
}

# Show status of the EMQX broker
show_status() {
    log_info "EMQX MQTT broker status:"
    echo
    
    check_docker
    
    if ! container_exists; then
        log_warning "Container ${CONTAINER_NAME} does not exist"
        return 0
    fi
    
    if container_running; then
        log_success "Status: RUNNING"
        echo
        log_info "Container details:"
        docker ps --filter "name=${CONTAINER_NAME}" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
        echo
        log_info "Access information:"
        echo "  MQTT Broker: localhost:${MQTT_PORT}"
        echo "  Dashboard: http://localhost:${DASHBOARD_PORT}"
        echo "  Default credentials: admin/public"
        echo
        log_info "Container logs (last 10 lines):"
        docker logs --tail 10 "${CONTAINER_NAME}"
    else
        log_warning "Status: STOPPED"
        echo
        log_info "Container details:"
        docker ps -a --filter "name=${CONTAINER_NAME}" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
    fi
}

# Show container logs
show_logs() {
    log_info "Showing EMQX MQTT broker logs..."
    echo
    
    check_docker
    
    if ! container_exists; then
        log_error "Container ${CONTAINER_NAME} does not exist"
        exit 1
    fi
    
    if ! container_running; then
        log_warning "Container ${CONTAINER_NAME} is not running"
        log_info "Showing last 50 lines of logs:"
        docker logs --tail 50 "${CONTAINER_NAME}"
    else
        log_info "Tailing live logs (press Ctrl+C to stop):"
        docker logs -f "${CONTAINER_NAME}"
    fi
}

# Show usage information
show_usage() {
    echo "Usage: $0 {start|stop|restart|status|logs}"
    echo
    echo "Commands:"
    echo "  start   - Start the EMQX MQTT broker"
    echo "  stop    - Stop the EMQX MQTT broker"
    echo "  restart - Restart the EMQX MQTT broker"
    echo "  status  - Show the status of the EMQX MQTT broker"
    echo "  logs    - Show/tail the logs of the EMQX MQTT broker"
    echo
    echo "Configuration:"
    echo "  Container name: ${CONTAINER_NAME}"
    echo "  MQTT port: ${MQTT_PORT}"
    echo "  Dashboard port: ${DASHBOARD_PORT}"
    echo "  Data volume: ${DATA_VOLUME}"
    echo "  Rules file: ${RULES_FILE}"
}

# Main script logic
case "${1:-}" in
    start)
        start_broker
        ;;
    stop)
        stop_broker
        ;;
    restart)
        restart_broker
        ;;
    status)
        show_status
        ;;
    logs)
        show_logs
        ;;
    *)
        show_usage
        exit 1
        ;;
esac
