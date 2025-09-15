#!/bin/bash

# Cleanup script for metrics-agent test artifacts
# This script will remove:
# 1. All git tags (local and remote)
# 2. All GitHub releases
# 3. All GitHub Actions workflow runs

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

# Configuration
REPO_OWNER="janhuddel"
REPO_NAME="metrics-agent"
GITHUB_TOKEN="${GITHUB_TOKEN:-}"

# Check if GitHub token is provided
if [ -z "$GITHUB_TOKEN" ]; then
    echo -e "${RED}Error: GITHUB_TOKEN environment variable is required${NC}"
    echo "Please set your GitHub token:"
    echo "export GITHUB_TOKEN=your_github_token_here"
    echo ""
    echo "You can create a token at: https://github.com/settings/tokens"
    echo "Required scopes: repo, admin:org (for deleting releases and workflow runs)"
    exit 1
fi

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to confirm action
confirm() {
    read -p "$(echo -e ${YELLOW}$1${NC}) [y/N]: " -n 1 -r
    echo
    [[ $REPLY =~ ^[Yy]$ ]]
}

# Check if we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    print_error "Not in a git repository"
    exit 1
fi

# Check if gh CLI is installed
if ! command -v gh &> /dev/null; then
    print_error "GitHub CLI (gh) is not installed"
    echo "Please install it from: https://cli.github.com/"
    exit 1
fi

# Authenticate with GitHub CLI
print_status "Authenticating with GitHub..."
gh auth status > /dev/null 2>&1 || {
    print_error "Not authenticated with GitHub CLI"
    echo "Please run: gh auth login"
    exit 1
}

print_status "Starting cleanup of test artifacts for $REPO_OWNER/$REPO_NAME"
echo ""

# 1. Clean up git tags
print_status "Step 1: Cleaning up git tags..."

# Get all local tags
LOCAL_TAGS=$(git tag -l)
if [ -n "$LOCAL_TAGS" ]; then
    echo "Local tags found:"
    echo "$LOCAL_TAGS" | sed 's/^/  - /'
    echo ""
    
    if confirm "Delete all local tags?"; then
        echo "$LOCAL_TAGS" | xargs git tag -d
        print_success "Deleted all local tags"
    else
        print_warning "Skipped local tag deletion"
    fi
else
    print_status "No local tags found"
fi

# Get all remote tags
REMOTE_TAGS=$(git ls-remote --tags origin | grep -v '\^{}' | sed 's/.*refs\/tags\///')
if [ -n "$REMOTE_TAGS" ]; then
    echo "Remote tags found:"
    echo "$REMOTE_TAGS" | sed 's/^/  - /'
    echo ""
    
    if confirm "Delete all remote tags?"; then
        echo "$REMOTE_TAGS" | xargs -I {} git push origin :refs/tags/{}
        print_success "Deleted all remote tags"
    else
        print_warning "Skipped remote tag deletion"
    fi
else
    print_status "No remote tags found"
fi

echo ""

# 2. Clean up GitHub releases
print_status "Step 2: Cleaning up GitHub releases..."

# Get all releases
RELEASES=$(gh api repos/$REPO_OWNER/$REPO_NAME/releases --paginate --jq '.[].id' 2>/dev/null || echo "")
if [ -n "$RELEASES" ]; then
    echo "Releases found:"
    gh api repos/$REPO_OWNER/$REPO_NAME/releases --paginate --jq '.[] | "  - \(.tag_name) (ID: \(.id))"' 2>/dev/null | cat || echo "  (Unable to fetch release details)"
    echo ""
    
    if confirm "Delete all releases?"; then
        echo "$RELEASES" | while read -r release_id; do
            if [ -n "$release_id" ]; then
                print_status "Deleting release ID: $release_id"
                gh api -X DELETE repos/$REPO_OWNER/$REPO_NAME/releases/$release_id 2>/dev/null | cat || print_warning "Failed to delete release ID: $release_id"
            fi
        done
        print_success "Deleted all releases"
    else
        print_warning "Skipped release deletion"
    fi
else
    print_status "No releases found"
fi

echo ""

# 3. Clean up GitHub Actions workflow runs
print_status "Step 3: Cleaning up GitHub Actions workflow runs..."

# Get all workflow runs
WORKFLOW_RUNS=$(gh api repos/$REPO_OWNER/$REPO_NAME/actions/runs --paginate --jq '.workflow_runs[].id' 2>/dev/null || echo "")
if [ -n "$WORKFLOW_RUNS" ]; then
    echo "Workflow runs found:"
    gh api repos/$REPO_OWNER/$REPO_NAME/actions/runs --paginate --jq '.workflow_runs[] | "  - Run ID: \(.id), Workflow: \(.name), Status: \(.status), Conclusion: \(.conclusion)"' 2>/dev/null | cat || echo "  (Unable to fetch workflow run details)"
    echo ""
    
    if confirm "Delete all workflow runs?"; then
        echo "$WORKFLOW_RUNS" | while read -r run_id; do
            if [ -n "$run_id" ]; then
                print_status "Deleting workflow run ID: $run_id"
                gh api -X DELETE repos/$REPO_OWNER/$REPO_NAME/actions/runs/$run_id 2>/dev/null | cat || print_warning "Failed to delete workflow run ID: $run_id"
            fi
        done
        print_success "Deleted all workflow runs"
    else
        print_warning "Skipped workflow run deletion"
    fi
else
    print_status "No workflow runs found"
fi

echo ""
print_success "Cleanup completed!"
echo ""
print_status "Summary of what was cleaned:"
echo "  ✓ Git tags (local and remote)"
echo "  ✓ GitHub releases"
echo "  ✓ GitHub Actions workflow runs"
echo ""
print_status "You can now create a fresh release to test the updated pipeline."
