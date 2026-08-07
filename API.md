# API Documentation

This document describes the API endpoints provided by the mini-orca daemon.

## General Endpoints

### Health Check
- **URL**: `/health`
- **Method**: `GET`
- **Description**: Returns the health status of the daemon.
- **Response**: `{"status":"ok","version":"<version>"}`

### Daemon Status
- **URL**: `/status`
- **Method**: `GET`
- **Description**: Returns the running status and registered agents.
- **Response**: `{"status":"running","version":"<version>","agents":["coder","tester","reviewer"]}`

### System Info
- **URL**: `/api/system/info`
- **Method**: `GET`
- **Description**: Returns system information (OS, architecture, Go version, etc.).
- **Response**: JSON object with system details.

## Chat API

### Send Message
- **URL**: `/api/chat/message`
- **Method**: `POST`
- **Description**: Sends a message to start a new session or continue an existing one.
- **Body**: 
```json
{
  "message": "string - Natural language description of the feature to implement",
  "project_path": "string - Filesystem path to the existing project",
  "project_type": "string - Optional: 'go', 'python', etc."
}
```
- **Response**: Streaming SSE (Server-Sent Events) with incremental updates.

### Get Chat History
- **URL**: `/api/chat/history`
- **Method**: `GET`
- **Description**: Returns the message history for the current session.
- **Response**: JSON array of message objects.

## UI Rendering (HTMX)

These endpoints return HTML fragments for the HTMX-based frontend.

### Render Phase
- **URL**: `/api/render/phase/{name}`
- **Method**: `GET`
- **Description**: Renders the UI for a specific phase.

### Render File Tree
- **URL**: `/api/render/file-tree`
- **Method**: `GET`
- **Description**: Renders the file tree component.

### Render Dashboard
- **URL**: `/api/render/dashboard`
- **Method**: `GET`
- **Description**: Renders the dashboard component.

### Expand Folder
- **URL**: `/api/tree/expand`
- **Method**: `GET`
- **Description**: Expands a folder in the file tree.

### View File
- **URL**: `/api/files/view`
- **Method**: `GET`
- **Description**: Displays the content of a file.

### Main Page
- **URL**: `/`
- **Method**: `GET`
- **Description**: Renders the main IDE page.

## Static Files

- **URL**: `/static/*`
- **Method**: `GET`
- **Description**: Serves static assets (CSS, JS, images).
