#!/bin/bash

# Test script for panic recovery in Elixir metrics agent
# This script provides an interactive menu to test the panic recovery mechanism

PANIC_FILE="/tmp/metrics-agent-panic-demo"

echo "Metrics Agent Panic Recovery Test"
echo "================================="
echo ""

while true; do
    echo "Current status:"
    if [ -f "$PANIC_FILE" ]; then
        echo "  Panic trigger: ACTIVE (file exists)"
    else
        echo "  Panic trigger: INACTIVE (file does not exist)"
    fi
    echo ""
    
    echo "Options:"
    echo "1) Trigger panic in demo module"
    echo "2) Remove panic trigger"
    echo "3) Check current status"
    echo "4) Exit"
    echo ""
    
    read -p "Select option (1-4): " choice
    
    case $choice in
        1)
            echo "Creating panic trigger file..."
            touch "$PANIC_FILE"
            echo "Panic trigger activated. The demo module should panic on next metric collection."
            echo "Watch the logs to see the panic recovery in action."
            ;;
        2)
            echo "Removing panic trigger file..."
            rm -f "$PANIC_FILE"
            echo "Panic trigger deactivated. The demo module should work normally now."
            ;;
        3)
            echo "Current status:"
            if [ -f "$PANIC_FILE" ]; then
                echo "  Panic trigger: ACTIVE"
                echo "  File: $PANIC_FILE"
                echo "  Size: $(stat -f%z "$PANIC_FILE" 2>/dev/null || stat -c%s "$PANIC_FILE" 2>/dev/null || echo "unknown") bytes"
                echo "  Modified: $(stat -f%Sm "$PANIC_FILE" 2>/dev/null || stat -c%y "$PANIC_FILE" 2>/dev/null || echo "unknown")"
            else
                echo "  Panic trigger: INACTIVE"
            fi
            ;;
        4)
            echo "Cleaning up..."
            rm -f "$PANIC_FILE"
            echo "Goodbye!"
            exit 0
            ;;
        *)
            echo "Invalid option. Please select 1-4."
            ;;
    esac
    
    echo ""
    echo "Press Enter to continue..."
    read
    clear
done
