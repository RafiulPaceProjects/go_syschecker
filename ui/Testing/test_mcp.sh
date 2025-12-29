#!/bin/bash
# Test script for MCP server and tool calling

cd "$(dirname "$0")"

echo "🧪 Testing MCP Server and Tool Calling"
echo "======================================="
echo ""

# Test 1: Check if server binary exists
echo "✓ Test 1: Check MCP server binary"
if [ -f "../../syschecker-mcp" ]; then
    echo "  ✅ MCP server binary found"
else
    echo "  ❌ MCP server binary not found"
    exit 1
fi

# Test 2: Check if chatbot binary exists
echo ""
echo "✓ Test 2: Check chatbot binary"
if [ -f "./chatbot" ]; then
    echo "  ✅ Chatbot binary found"
else
    echo "  ❌ Chatbot binary not found"
    exit 1
fi

# Test 3: Check environment variables
echo ""
echo "✓ Test 3: Check environment variables"
if [ -f "env/.env" ]; then
    source env/.env
    if [ -n "$GEMINI_API_KEY" ]; then
        echo "  ✅ GEMINI_API_KEY is set"
    else
        echo "  ❌ GEMINI_API_KEY is not set"
        exit 1
    fi
else
    echo "  ❌ env/.env file not found"
    exit 1
fi

# Test 4: Test /metrics command
echo ""
echo "✓ Test 4: Testing get_realtime_metrics tool"
echo "/metrics" | timeout 10s ./chatbot > /tmp/mcp_test_metrics.txt 2>&1 &
METRICS_PID=$!
sleep 3
kill $METRICS_PID 2>/dev/null
wait $METRICS_PID 2>/dev/null

if grep -q "Fetching metrics" /tmp/mcp_test_metrics.txt; then
    echo "  ✅ Metrics tool callable"
    if grep -q "CPU" /tmp/mcp_test_metrics.txt || grep -q "stats" /tmp/mcp_test_metrics.txt; then
        echo "  ✅ Metrics returned data"
    else
        echo "  ⚠️  Metrics response may be incomplete"
        cat /tmp/mcp_test_metrics.txt
    fi
else
    echo "  ❌ Metrics tool failed"
    cat /tmp/mcp_test_metrics.txt
fi

# Test 5: Test ask_syschecker tool
echo ""
echo "✓ Test 5: Testing ask_syschecker tool"
echo "What is the current CPU usage?" | timeout 15s ./chatbot > /tmp/mcp_test_ask.txt 2>&1 &
ASK_PID=$!
sleep 8
kill $ASK_PID 2>/dev/null
wait $ASK_PID 2>/dev/null

if grep -q "Thinking" /tmp/mcp_test_ask.txt; then
    echo "  ✅ Ask tool callable"
    if grep -q "Bot:" /tmp/mcp_test_ask.txt; then
        echo "  ✅ Ask tool returned response"
    else
        echo "  ⚠️  Ask response may be incomplete"
        cat /tmp/mcp_test_ask.txt
    fi
else
    echo "  ❌ Ask tool failed"
    cat /tmp/mcp_test_ask.txt
fi

echo ""
echo "======================================="
echo "🎉 MCP Server Tests Complete"
echo ""
echo "To run interactively: ./chatbot"
echo "Available commands:"
echo "  /metrics - Test get_realtime_metrics tool"
echo "  /help    - Show help"
echo "  /exit    - Exit"
echo "  <question> - Test ask_syschecker tool"
