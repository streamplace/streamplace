#!/bin/bash

# Script to clean up git worktrees that have finished their work
# Checks for uncommitted changes, unpushed commits, and merged branches
#
# Usage:
#   cleanup-worktrees.sh                    # Scan all child directories for git repositories with worktrees
#   cleanup-worktrees.sh <repository-dir>   # Check worktrees in a specific repository

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to check worktrees in current repository
check_worktrees() {
    # Get list of all worktrees
    local worktrees=$(git worktree list --porcelain | grep "^worktree " | cut -d' ' -f2-)

    # Array to store worktrees that can be removed
    local removable_worktrees=()

    for worktree in $worktrees; do
        # Skip the main worktree (development)
        if [[ "$worktree" == "$MAIN_WORKTREE" ]]; then
            continue
        fi

        echo -e "\nChecking: $worktree"
        cd "$worktree"

        # Get the branch name
        branch=$(git branch --show-current)
        echo "Branch: $branch"

        # Check for uncommitted changes
        if ! git diff --quiet || ! git diff --staged --quiet; then
            echo -e "${RED}✗ Has uncommitted changes${NC}"
            continue
        fi
        echo -e "${GREEN}✓ No uncommitted changes${NC}"

        # Check for untracked files
        if [[ -n $(git ls-files --others --exclude-standard) ]]; then
            echo -e "${YELLOW}⚠ Has untracked files${NC}"
            # List untracked files
            git ls-files --others --exclude-standard | sed 's/^/  /'
        fi

        # Check for unpushed commits
        upstream=$(git rev-parse --abbrev-ref --symbolic-full-name @{u} 2>/dev/null || echo "")
        if [[ -n "$upstream" ]]; then
            unpushed=$(git rev-list HEAD..."$upstream" --count 2>/dev/null || echo "0")
            if [[ "$unpushed" -gt 0 ]]; then
                echo -e "${RED}✗ Has $unpushed unpushed commit(s)${NC}"
                continue
            fi
            echo -e "${GREEN}✓ All commits pushed${NC}"
        else
            echo -e "${YELLOW}⚠ No upstream branch set${NC}"
            # Check if there are any commits not in development
            cd "$MAIN_WORKTREE"
            unmerged=$(git rev-list development.."$branch" --count 2>/dev/null || echo "0")
            cd "$worktree"
            if [[ "$unmerged" -gt 0 ]]; then
                echo -e "${RED}✗ Has $unmerged commit(s) not in development${NC}"
                continue
            fi
        fi

        # Check if branch is merged into development
        cd "$MAIN_WORKTREE"
        if git merge-base --is-ancestor "$branch" development 2>/dev/null; then
            echo -e "${GREEN}✓ Branch is merged into development${NC}"
            removable_worktrees+=("$worktree|$branch")
        else
            echo -e "${YELLOW}⚠ Branch is not merged into development${NC}"
            # Check if there are any commits in this branch that aren't in development
            unique_commits=$(git rev-list development.."$branch" --count 2>/dev/null || echo "0")
            if [[ "$unique_commits" -eq 0 ]]; then
                echo -e "${GREEN}  But no unique commits found${NC}"
                removable_worktrees+=("$worktree|$branch")
            else
                echo -e "${RED}  Has $unique_commits unique commit(s)${NC}"
            fi
        fi
    done

    echo -e "\n================================"
    echo "Summary:"
    echo "================================"

    if [[ ${#removable_worktrees[@]} -eq 0 ]]; then
        echo -e "${YELLOW}No worktrees are ready for cleanup.${NC}"
        return
    fi

    echo -e "${GREEN}The following worktrees can be safely removed:${NC}"
    for item in "${removable_worktrees[@]}"; do
        IFS='|' read -r worktree branch <<< "$item"
        echo "  - $worktree (branch: $branch)"
    done

    echo -e "\n${YELLOW}Do you want to remove these worktrees? (y/N)${NC}"
    read -r response

    if [[ "$response" =~ ^[Yy]$ ]]; then
        for item in "${removable_worktrees[@]}"; do
            IFS='|' read -r worktree branch <<< "$item"
            echo -e "\nRemoving worktree: $worktree"
            cd "$MAIN_WORKTREE"
            git worktree remove "$worktree" --force
            echo -e "${GREEN}✓ Removed${NC}"

            # Ask if the branch should be deleted
            echo -e "${YELLOW}Delete branch '$branch'? (y/N)${NC}"
            read -r delete_response
            if [[ "$delete_response" =~ ^[Yy]$ ]]; then
                git branch -d "$branch" --force 2>/dev/null || git branch -D "$branch"
                echo -e "${GREEN}✓ Branch deleted${NC}"
            fi
        done
        echo -e "\n${GREEN}Cleanup complete!${NC}"
    else
        echo -e "${YELLOW}Cleanup cancelled.${NC}"
    fi
}

# Function to process a single repository
process_repository() {
    local repo_dir="$1"
    local repo_name=$(basename "$repo_dir")

    cd "$repo_dir"

    # Check if it's a git repository
    if ! git rev-parse --git-dir > /dev/null 2>&1; then
        return
    fi

    # Check if this repo has worktrees
    local worktree_count=$(git worktree list | wc -l)
    if [ "$worktree_count" -le 1 ]; then
        return
    fi

    echo -e "\n${YELLOW}Repository: $repo_name${NC}"
    echo -e "${YELLOW}Path: $repo_dir${NC}"

    # Get the main worktree directory
    MAIN_WORKTREE=$(git worktree list | head -n1 | awk '{print $1}')
    cd "$MAIN_WORKTREE"

    # Process worktrees for this repository
    check_worktrees
}

# Check if directory parameter is provided
if [ $# -eq 0 ]; then
    echo -e "${GREEN}Scanning for git repositories in current directory...${NC}"
    echo "================================"

    # Save current directory
    START_DIR=$(pwd)

    # Find all directories that might be git repositories
    for dir in */; do
        if [ -d "$dir" ]; then
            cd "$START_DIR"
            process_repository "$START_DIR/$dir"
        fi
    done

    cd "$START_DIR"
else
    REPO_DIR="$1"

    # Check if the provided directory exists
    if [ ! -d "$REPO_DIR" ]; then
        echo -e "${RED}Error: Directory '$REPO_DIR' does not exist${NC}"
        exit 1
    fi

    # Change to the repository directory
    cd "$REPO_DIR"

    # Check if it's a git repository
    if ! git rev-parse --git-dir > /dev/null 2>&1; then
        echo -e "${RED}Error: '$REPO_DIR' is not a git repository${NC}"
        exit 1
    fi

    # Get the main worktree directory
    MAIN_WORKTREE=$(git worktree list | head -n1 | awk '{print $1}')
    cd "$MAIN_WORKTREE"

    echo "Checking worktrees for cleanup..."
    echo "================================"
    check_worktrees
fi
