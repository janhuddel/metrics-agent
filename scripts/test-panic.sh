#!/bin/bash

# Test script for panic simulation in metrics-agent
# This script helps you test the panic recovery mechanism

echo "=== Metrics Agent Panic Test Script ==="
echo "Note: Restart limit is configurable in metrics-agent.json (default: 3)"
echo ""

# Function to show current status
show_status() {
    echo "Current status:"
    if [ -f "/tmp/metrics-agent-panic-demo" ]; then
        echo "  🚨 PANIC TRIGGER FILE EXISTS - Demo module will panic on next tick"
        echo "  ⚠️  WARNING: After 3 panics, the program will exit!"
    else
        echo "  ✅ No panic trigger file - Demo module running normally"
    fi
    echo ""
}

# Function to create panic trigger
trigger_panic() {
    echo "Creating panic trigger file..."
    touch /tmp/metrics-agent-panic-demo
    echo "✅ Panic trigger file created: /tmp/metrics-agent-panic-demo"
    echo "   Demo module will panic on its next 5-second tick"
    echo ""
}

# Function to remove panic trigger
remove_panic() {
    echo "Removing panic trigger file..."
    rm -f /tmp/metrics-agent-panic-demo
    echo "✅ Panic trigger file removed"
    echo "   Demo module will resume normal operation after restart"
    echo ""
}

# Main menu
while true; do
    show_status
    echo "Options:"
    echo "  1) Trigger panic in demo module (will cause restart limit test)"
    echo "  2) Remove panic trigger (allow normal operation)"
    echo "  3) Show status"
    echo "  4) Test restart limit (trigger 4 panics to exit program)"
    echo "  5) Exit"
    echo ""
    read -p "Choose option (1-5): " choice
    
    case $choice in
        1)
            trigger_panic
            ;;
        2)
            remove_panic
            ;;
        3)
            show_status
            ;;
        4)
            echo "Testing restart limit - this will cause the program to exit after 4 panics..."
            echo "Creating persistent panic trigger..."
            touch /tmp/metrics-agent-panic-demo
            echo "✅ Panic trigger created. The demo module will panic every 5 seconds."
            echo "   After 3 restarts, the program will exit."
            echo "   Watch the metrics-agent logs to see the restart limit in action."
            echo ""
            ;;
        5)
            echo "Exiting..."
            exit 0
            ;;
        *)
            echo "Invalid option. Please choose 1-5."
            ;;
    esac
    
    echo "Press Enter to continue..."
    read
    clear
done
