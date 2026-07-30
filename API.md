# API Documentation

This document describes the API endpoints provided by the mini-orca daemon.

## General Endpoints

### Health Check
- **URL**: `/health`
- **Method**: `GET`
- **Description**: Returns the health status of the daemon.
- **Response**: `{"status":"ok"}`

### Daemon Status
- **URL**: `/status`
- **Method**: `GET`
- **Description**: Returns the running status and registered agents.
- **Response**: `{"status":"running","agents":["planner","coder","tester","reviewer"]}`

## Session Management

### Create Session
- **URL**: `/api/sessions`
- **Method**: `POST`
- **Description**: Creates a new agent session.
- **Body**: Session configuration (optional).

### List Sessions
- **URL**: `/api/sessions`
- **Method**: `GET`
- **Description**: Lists all active and past sessions.

### Get Session Status
- **URL**: `/api/sessions/{id}`
- **Method**: `GET`
- **Description**: Returns the current status of a specific session.

### Start Session
- **URL**: `/api/sessions/{id}/start`
- **Method**: `POST`
- **Description**: Starts the execution of a session.

### Pause Session
- **URL**: `/api/sessions/{id}/pause`
- **Method**: `POST`
- **Description**: Pauses a running session.

### Resume Session
- **URL**: `/api/sessions/{id}/resume`
- **Method**: `POST`
- **Description**: Resumes a paused session.

### Stop Session
- **URL**: `/api/sessions/{id}/stop`
- **Method**: `POST`
- **Description**: Stops a running session.

### Get Gate Status
- **URL**: `/api/sessions/{id}/gate`
- **Method**: `GET`
- **Description**: Returns the status of the human gate for the session.

### Respond to Gate
- **URL**: `/api/sessions/{id}/gate`
- **Method**: `POST`
- **Description**: Provides feedback or approval to a pending gate request.

## Project Management

### List Projects
- **URL**: `/api/projects`
- **Method**: `GET`
- **Description**: Lists all projects managed by the daemon.

### Create Project
- **URL**: `/api/projects`
- **Method**: `POST`
- **Description**: Registers a new project.

### List Project Files
- **URL**: `/api/projects/{id}/files`
- **Method**: `GET`
- **Description**: Lists files within a project.

### Get File Content
- **URL**: `/api/projects/files/{path}`
- **Method**: `GET`
- **Description**: Returns the content of a specific file.

## UI Rendering (HTMX)

These endpoints return HTML fragments for the HTMX-based frontend.

- `GET /api/render/phase/{name}`: Renders the UI for a specific phase.
- `GET /api/render/file-tree`: Renders the file tree component.
- `GET /api/render/activity-log`: Renders the activity log component.
- `GET /api/render/phase-tracker`: Renders the phase tracker component.
- `GET /`: Renders the main IDE page.
