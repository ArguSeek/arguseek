#!/usr/bin/env node

/**
 * Simple HTTP MCP Bridge for ArguSeek
 * Bridges Claude Desktop (stdio) to ArguSeek HTTP MCP server
 */

const readline = require('readline');
const fetch = require('node-fetch');

// Configuration from environment variables
const ARGUSEEK_URL = process.env.ARGUSEEK_URL || 'http://localhost:8080/mcp';

// Server info
const SERVER_INFO = {
  name: 'arguseek',
  version: '1.0.0'
};

// Set up stdio communication
const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout,
  terminal: false
});

// HTTP request function
async function makeHttpRequest(payload) {
  let timedOut = false;
  try {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => {
      timedOut = true;
      controller.abort();
    }, 90000); // 90 second timeout

    const response = await fetch(ARGUSEEK_URL, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(payload),
      signal: controller.signal,
    });

    clearTimeout(timeoutId);

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${await response.text()}`);
    }

    return await response.json();
  } catch (error) {
    if (error.name === 'AbortError') {
      const message = timedOut ? 'Request timed out after 90 seconds' : 'Request was cancelled by user';
      console.error('HTTP request failed:', message);
      throw new Error(message);
    }
    console.error('HTTP request failed:', error.message);
    throw error;
  }
}

// Initialize remote server
async function initializeRemoteServer() {
  const payload = {
    jsonrpc: '2.0',
    id: 'init',
    method: 'initialize',
    params: {
      protocolVersion: '2024-11-05',
      capabilities: {},
      clientInfo: {
        name: 'arguseek-http-bridge',
        version: '1.0.0',
      },
    },
  };
  
  try {
    const response = await makeHttpRequest(payload);
    if (response.result) {
      console.error('Remote server initialized successfully');
      return response.result;
    } else {
      throw new Error('Invalid initialize response');
    }
  } catch (error) {
    console.error('Failed to initialize remote server:', error.message);
    // Don't throw error, continue with basic capabilities
    return {
      protocolVersion: '2024-11-05',
      capabilities: { tools: { listChanged: false } },
      serverInfo: { name: 'arguseek', version: '1.0.0' }
    };
  }
}

// Send JSON-RPC response
function sendResponse(id, result, error = null) {
  const response = {
    jsonrpc: '2.0',
    id: id !== undefined && id !== null ? id : 0, // Use 0 instead of null for unknown IDs
  };
  
  if (error) {
    response.error = error;
  } else {
    response.result = result !== undefined ? result : {};
  }
  
  console.log(JSON.stringify(response));
}

// Handle incoming requests
function handleRequest(request) {
  try {
    const req = JSON.parse(request);
    console.error('Handling request:', req.method, 'with ID:', req.id);

    switch (req.method) {
      case 'initialize':
        // Return our server capabilities immediately
        const initResult = {
          protocolVersion: '2024-11-05',
          capabilities: {
            tools: {
              listChanged: false
            }
          },
          serverInfo: SERVER_INFO
        };
        sendResponse(req.id, initResult);
        break;

      case 'notifications/initialized':
        // MCP protocol notification - no response needed
        console.error('Received initialized notification');
        break;

      case 'tools/list':
        // Forward to remote server
        const listPayload = {
          jsonrpc: '2.0',
          id: req.id,
          method: 'tools/list'
        };

        makeHttpRequest(listPayload)
          .then(response => {
            sendResponse(req.id, response.result || { tools: [] });
          })
          .catch(error => {
            console.error('tools/list error:', error.message);
            sendResponse(req.id, null, {
              code: -32603,
              message: 'Internal error: ' + error.message
            });
          });
        break;

      case 'tools/call':
        // Forward to remote server
        const callPayload = {
          jsonrpc: '2.0',
          id: req.id,
          method: 'tools/call',
          params: req.params
        };

        makeHttpRequest(callPayload)
          .then(response => {
            if (response.result) {
              // Server now returns MCP-compliant format, pass through directly
              sendResponse(req.id, response.result);
            } else if (response.error) {
              sendResponse(req.id, null, response.error);
            } else {
              sendResponse(req.id, null, {
                code: -32603,
                message: 'Unexpected response format'
              });
            }
          })
          .catch(error => {
            console.error('tools/call error:', error.message);
            sendResponse(req.id, null, {
              code: -32603,
              message: 'Internal error: ' + error.message
            });
          });
        break;

      default:
        sendResponse(req.id, null, {
          code: -32601,
          message: 'Method not found'
        });
    }
  } catch (error) {
    console.error('Error handling request:', error.message);
    // Try to extract ID from the original request, fallback to 0
    let requestId = 0;
    try {
      const req = JSON.parse(request);
      requestId = req.id !== undefined && req.id !== null ? req.id : 0;
    } catch (e) {
      // If we can't parse the request, use 0
    }

    sendResponse(requestId, null, {
      code: -32700,
      message: 'Parse error'
    });
  }
}

// Main function
async function main() {
  try {
    
    // Handle incoming requests
    rl.on('line', handleRequest);
    
    // Handle shutdown
    process.on('SIGINT', () => {
      console.error('Shutting down...');
      rl.close();
      process.exit(0);
    });
    
    process.on('SIGTERM', () => {
      console.error('Shutting down...');
      rl.close();
      process.exit(0);
    });
    
  } catch (error) {
    console.error('Failed to start server:', error.message);
    process.exit(1);
  }
}

// Start the server
main();