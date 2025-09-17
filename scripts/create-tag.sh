#!/bin/bash

# Git tag creation script for metrics-agent
# This script creates a git tag with the provided version name and asks for confirmation before pushing

set -e

# Disable pager for all commands
export PAGER=cat
export GIT_PAGER=cat

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "ℹ️ $1"
}

print_success() {
    echo -e "✅ $1"
}

print_warning() {
    echo -e "⚠️ $1"
}

print_error() {
    echo -e "❌ $1"
}

# Function to confirm action
confirm() {
    read -p "$(echo -e ${YELLOW}$1${NC}) [y/N]: " -n 1 -r
    echo
    [[ $REPLY =~ ^[Yy]$ ]]
}

# Function to show usage
show_usage() {
    echo "Usage: $0 [version]"
    echo ""
    echo "Creates a git tag with the specified version name and optionally pushes it to remote."
    echo "If no version is provided, shows recent tags and suggests the next version."
    echo ""
    echo "Examples:"
    echo "  $0 v1.0.0"
    echo "  $0 1.2.3"
    echo "  $0 release-2024-01-15"
    echo "  $0                    # Interactive mode with suggestions"
    echo ""
    echo "The version name will be used as-is for the tag name."
}

# Function to get the last 5 tags
get_last_tags() {
    git tag -l --sort=-version:refname | head -5
}

# Function to suggest next version
suggest_next_version() {
    local last_tag=$(git tag -l --sort=-version:refname | head -1)
    
    if [ -z "$last_tag" ]; then
        echo "v1.0.0"
        return
    fi
    
    # Try to parse semantic version (v1.2.3 or 1.2.3)
    if [[ $last_tag =~ ^v?([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
        local major=${BASH_REMATCH[1]}
        local minor=${BASH_REMATCH[2]}
        local patch=${BASH_REMATCH[3]}
        
        # Increment patch version
        local next_patch=$((patch + 1))
        
        # Check if last tag had 'v' prefix
        if [[ $last_tag =~ ^v ]]; then
            echo "v${major}.${minor}.${next_patch}"
        else
            echo "${major}.${minor}.${next_patch}"
        fi
    else
        # For non-semantic versions, suggest a timestamp-based version
        echo "v$(date +%Y.%m.%d)"
    fi
}

# Function to get tag version interactively
get_version_interactive() {
    echo ""
    print_status "Recent tags:"
    local last_tags=$(get_last_tags)
    if [ -n "$last_tags" ]; then
        echo "$last_tags" | sed 's/^/  - /'
    else
        echo "  (no tags found)"
    fi
    
    echo ""
    local suggestion=$(suggest_next_version)
    print_status "Suggested next version: $suggestion"
    echo ""
    
    echo -ne "${YELLOW}Use suggested version '$suggestion'? [Y/n]: ${NC}"
    read -r
    echo
    
    if [[ $REPLY =~ ^[Nn] ]]; then
        echo ""
        read -p "$(echo -e ${YELLOW}Enter custom version name: ${NC})" VERSION
        if [ -z "$VERSION" ]; then
            print_error "No version provided"
            exit 1
        fi
    else
        VERSION="$suggestion"
    fi
    
    echo ""
    print_status "Selected version: $VERSION"
}

# Check for help option
if [ $# -eq 1 ] && [[ "$1" == "--help" || "$1" == "-h" ]]; then
    show_usage
    exit 0
fi

# Check if version argument is provided
if [ $# -eq 0 ]; then
    # Interactive mode - show recent tags and suggest next version
    get_version_interactive
else
    VERSION="$1"
fi

# Check if we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    print_error "Not in a git repository"
    exit 1
fi

# Check if working directory is clean
if ! git diff-index --quiet HEAD --; then
    print_warning "Working directory is not clean"
    echo "Uncommitted changes detected:"
    git status --porcelain
    echo ""
    if ! confirm "Continue with uncommitted changes?"; then
        print_status "Aborted by user"
        exit 0
    fi
fi

# Check if tag already exists
if git tag -l | grep -q "^${VERSION}$"; then
    print_error "Tag '$VERSION' already exists"
    echo ""
    echo "Existing tags:"
    git tag -l | grep -E "^${VERSION}" | sed 's/^/  - /'
    echo ""
    if ! confirm "Do you want to delete the existing tag and recreate it?"; then
        print_status "Aborted by user"
        exit 0
    fi
    
    print_status "Deleting existing tag '$VERSION'..."
    git tag -d "$VERSION" 2>/dev/null || true
    
    # Check if tag exists on remote
    if git ls-remote --tags origin | grep -q "refs/tags/${VERSION}$"; then
        print_status "Deleting remote tag '$VERSION'..."
        git push origin ":refs/tags/${VERSION}" 2>/dev/null || true
    fi
fi

# Create the tag
print_status "Creating tag '$VERSION'..."
git tag "$VERSION"

# Show the new tag
print_success "Tag '$VERSION' created successfully"
echo ""
print_status "Tag details:"
echo "  Tag: $VERSION"
echo "  Commit: $(git rev-parse --short "$VERSION")"
echo "  Author: $(git log -1 --format="%an <%ae>" "$VERSION")"
echo "  Date: $(git log -1 --format="%ad" --date=short "$VERSION")"
echo ""

# Ask for confirmation to push
if confirm "Do you want to push the tag '$VERSION' to remote?"; then
    print_status "Pushing tag '$VERSION' to remote..."
    if git push origin "$VERSION"; then
        print_success "Tag '$VERSION' pushed to remote successfully"
    else
        print_error "Failed to push tag '$VERSION' to remote"
        exit 1
    fi
else
    print_status "Tag '$VERSION' created locally but not pushed to remote"
    echo "You can push it later with: git push origin $VERSION"
fi

echo ""
print_success "Tag creation completed!"
