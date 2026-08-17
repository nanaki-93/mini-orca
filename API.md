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
- **Description**: Generates one named function or class in one selected file. The daemon supplies the active project's analysis, complete file inventory, build metadata, and bounded source context.
- **Body**: 
```json
{
  "message": "Precise behavior to implement",
  "file_path": "relative/path/to/selected-file.go",
  "target_symbol": "Type.Method"
}
```
- **Response**: JSON agent result containing a one-file generated preview. The endpoint never writes generated code automatically.

## Project API

### Import and analyze a project

- **URL**: `/api/projects/import`
- **Method**: `POST`
- **Body**: `{"project_path":"/absolute/path/to/project"}`
- **Description**: Activates the directory, scans its files, asks the configured LLM for an architectural analysis, and writes `.mini-orca/analysis.md`.

### Current project information

- **URL**: `/api/projects/current`
- **Method**: `GET`
- **Description**: Returns project type, build file, file/source/line totals, languages, file inventory, AI status, and summary.

### Selected file information

- **URL**: `/api/projects/current/files/info?path=relative/path`
- **Method**: `GET`
- **Description**: Returns safe, project-relative file metadata and text content. Binary files and files over 1 MiB are not displayed.

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
