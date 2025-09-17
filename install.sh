#!/bin/bash

# Metrics Agent Installation Script
# This script installs the latest version of metrics-agent from GitHub releases

set -euo pipefail

# Configuration
REPO="janhuddel/metrics-agent"
BINARY_NAME="metrics-agent"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/metrics-agent"
DATA_DIR="/var/lib/metrics-agent"
CONFIG_FILE="$CONFIG_DIR/metrics-agent.json"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
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

# Check if running as root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "This script must be run as root (use sudo)"
        exit 1
    fi
}

# Detect system architecture
detect_arch() {
    local arch=$(uname -m)
    case $arch in
        x86_64)
            echo "amd64"
            ;;
        aarch64|arm64)
            echo "arm64"
            ;;
        armv7l)
            echo "armv7"
            ;;
        armv6l)
            echo "armv6"
            ;;
        *)
            log_error "Unsupported architecture: $arch"
            exit 1
            ;;
    esac
}

# Detect operating system
detect_os() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    case $os in
        linux)
            echo "linux"
            ;;
        *)
            log_error "Unsupported operating system: $os"
            exit 1
            ;;
    esac
}

# Get latest release version from GitHub API
get_latest_version() {
    local version
    version=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    
    if [[ -z "$version" ]]; then
        log_error "Failed to get latest version from GitHub API"
        exit 1
    fi
    
    echo "$version"
}

# Download and install binary
install_binary() {
    local version=$1
    local os=$2
    local arch=$3
    
    log_info "Installing metrics-agent version $version for $os-$arch"
    
    # Construct download URL
    local download_url="https://github.com/$REPO/releases/download/$version/${BINARY_NAME}-${os}-${arch}"
    
    log_info "Downloading from: $download_url"
    
    # Download binary
    if ! curl -L -o "/tmp/$BINARY_NAME" "$download_url"; then
        log_error "Failed to download binary from GitHub releases"
        exit 1
    fi
    
    # Make binary executable
    chmod +x "/tmp/$BINARY_NAME"
    
    # Install binary
    if ! mv "/tmp/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"; then
        log_error "Failed to install binary to $INSTALL_DIR"
        exit 1
    fi
    
    log_success "Binary installed to $INSTALL_DIR/$BINARY_NAME"
}

# Create directories
create_directories() {
    log_info "Creating directories..."
    
    # Create config directory
    if ! mkdir -p "$CONFIG_DIR"; then
        log_error "Failed to create config directory: $CONFIG_DIR"
        exit 1
    fi
    
    # Create data directory
    if ! mkdir -p "$DATA_DIR"; then
        log_error "Failed to create data directory: $DATA_DIR"
        exit 1
    fi
    
    log_success "Directories created successfully"
}

# Create default configuration
create_config() {
    log_info "Creating default configuration..."
    
    # Check if config already exists
    if [[ -f "$CONFIG_FILE" ]]; then
        log_warning "Configuration file already exists: $CONFIG_FILE"
        log_warning "Skipping configuration creation to preserve existing settings"
        return
    fi
    
    # Create default configuration
    cat > "$CONFIG_FILE" << 'EOF'
{
    "log_level": "info",
    "module_restart_limit": 3,
    "modules": {
        "tasmota": {
            "enabled": false,
            "friendly_name_overrides": {},
            "custom": {
                "broker": "tcp://localhost:1883",
                "timeout": "30s",
                "keep_alive": "60s",
                "ping_timeout": "10s"
            }
        },
        "netatmo": {
            "enabled": false,
            "friendly_name_overrides": {},
            "custom": {
                "client_id": "your_netatmo_client_id",
                "client_secret": "your_netatmo_client_secret",
                "timeout": "30s",
                "interval": "5m"
            }
        },
        "demo": {
            "enabled": false,
            "friendly_name_overrides": {},
            "custom": {}
        }
    }
}
EOF
    
    # Set proper permissions
    chmod 644 "$CONFIG_FILE"
    
    log_success "Default configuration created at $CONFIG_FILE"
    log_warning "All modules are disabled by default for security"
    log_warning "Edit $CONFIG_FILE to enable and configure modules"
}

# Set proper permissions
set_permissions() {
    log_info "Setting permissions..."
    
    # Set data directory permissions (telegraf user if it exists, otherwise root)
    if id "telegraf" &>/dev/null; then
        chown telegraf:telegraf "$DATA_DIR"
        log_info "Set data directory ownership to telegraf:telegraf"
    else
        log_warning "telegraf user not found, keeping data directory owned by root"
    fi
    
    chmod 755 "$DATA_DIR"
    chmod 755 "$CONFIG_DIR"
    
    log_success "Permissions set successfully"
}

# Verify installation
verify_installation() {
    log_info "Verifying installation..."
    
    # Check if binary exists and is executable
    if [[ ! -x "$INSTALL_DIR/$BINARY_NAME" ]]; then
        log_error "Binary not found or not executable: $INSTALL_DIR/$BINARY_NAME"
        exit 1
    fi
    
    # Test binary version
    local version_output
    if version_output=$("$INSTALL_DIR/$BINARY_NAME" -version 2>&1); then
        log_success "Binary is working correctly"
        log_info "Version: $version_output"
    else
        log_warning "Could not get version information from binary"
    fi
    
    # Check directories
    if [[ ! -d "$CONFIG_DIR" ]]; then
        log_error "Config directory not found: $CONFIG_DIR"
        exit 1
    fi
    
    if [[ ! -d "$DATA_DIR" ]]; then
        log_error "Data directory not found: $DATA_DIR"
        exit 1
    fi
    
    log_success "Installation verification completed"
}

# Print post-installation instructions
print_instructions() {
    echo
    log_success "Installation completed successfully!"
    echo
    echo "Next steps:"
    echo "1. Edit the configuration file:"
    echo "   sudo nano $CONFIG_FILE"
    echo
    echo "2. Enable and configure the modules you need:"
    echo "   - Set \"enabled\": true for modules you want to use"
    echo "   - Configure module-specific settings in the \"custom\" section"
    echo
    echo "3. Test the configuration:"
    echo "   $INSTALL_DIR/$BINARY_NAME -c $CONFIG_FILE"
    echo
    echo "4. For Telegraf integration, add to your telegraf.conf:"
    echo "   [[inputs.execd]]"
    echo "     command = [\"$INSTALL_DIR/$BINARY_NAME\", \"-c\", \"$CONFIG_FILE\"]"
    echo "     signal = \"STDIN\""
    echo "     restart_delay = \"10s\""
    echo
    echo "5. Restart Telegraf service:"
    echo "   sudo systemctl restart telegraf"
    echo
    log_info "For more information, see the README.md file"
}

# Main installation function
main() {
    echo "Metrics Agent Installation Script"
    echo "================================="
    echo
    
    # Check prerequisites
    check_root
    
    # Check for required tools
    if ! command -v curl &> /dev/null; then
        log_error "curl is required but not installed"
        exit 1
    fi
    
    # Detect system
    local os=$(detect_os)
    local arch=$(detect_arch)
    log_info "Detected system: $os-$arch"
    
    # Get latest version
    local version=$(get_latest_version)
    log_info "Latest version: $version"
    
    # Install binary
    install_binary "$version" "$os" "$arch"
    
    # Create directories
    create_directories
    
    # Create configuration
    create_config
    
    # Set permissions
    set_permissions
    
    # Verify installation
    verify_installation
    
    # Print instructions
    print_instructions
}

# Run main function
main "$@"
