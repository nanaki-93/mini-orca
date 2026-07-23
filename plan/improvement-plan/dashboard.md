# Frontend Dashboard Design

## Overview
The dashboard will provide a lightweight, real-time interface for users to monitor and control the Mini-Orca workflow. It will display project status, execution controls, and allow human approval of changes.

## Key Features

### 1. Project Status Monitoring
- Real-time visualization of project state
- Workflow progress tracking (planning, coding, testing, review)
- Change logs and status indicators
- Performance metrics

### 2. Execution Controls
- Start/stop workflow execution
- Manual trigger for each phase
- Configuration adjustment controls
- Approval/rejection interface

### 3. User Approval Workflow
- Clear presentation of generated code
- Diff view for changes
- Approval/rejection buttons
- Comments and feedback interface

### 4. Configuration Management
- Model selection per workflow phase
- Parameter adjustment controls
- Save/load configuration presets

## Dashboard Components

### 1. Header Section
- Project name and status indicator
- User controls (login/logout)
- Configuration settings button

### 2. Workflow Progress Section
- Visual workflow diagram showing current phase
- Status indicators (pending, in-progress, completed)
- Timeline of execution steps

### 3. Code Review Section
- Code display with syntax highlighting
- Diff view showing changes from previous versions
- Test results display

### 4. Controls Section
- Start/stop workflow buttons
- Manual phase triggers
- Approval/rejection controls

### 5. Configuration Section
- Model selection for each phase (planning, coding, testing, review)
- Parameter adjustment sliders/inputs
- Save/load configuration options

## Technical Requirements

### Frontend Technology
- Lightweight JavaScript framework (React or Vue.js)
- Responsive design for various screen sizes
- Real-time data updates using WebSocket or polling

### Data Display
- Real-time status updates from orchestrator
- Code rendering with syntax highlighting
- Test result visualization
- Approval workflow interface

### Integration Points
- Connect to orchestrator's status API
- Send approval/rejection signals
- Update configuration settings
- Display real-time logs and progress

## User Experience Considerations

### 1. Simplicity
- Clean, uncluttered interface
- Clear visual hierarchy
- Minimal required actions

### 2. Transparency
- Show all workflow steps and their status
- Display changes made to codebase
- Show model selection and parameters

### 3. Control
- Allow user to pause/resume workflow
- Enable manual intervention at any phase
- Provide clear approval/rejection mechanism

## Implementation Approach

1. **Dashboard Framework**: Build a React-based dashboard with Material UI components
2. **Real-time Updates**: Implement WebSocket connections to receive status updates
3. **Code Display**: Integrate syntax highlighting for code views
4. **Approval Workflow**: Create approval/rejection interface with comment support
5. **Configuration Management**: Build configuration UI with model selection per phase

## Sample Dashboard Layout

```
┌─────────────────────────────────────────────────────────┐
│ Project: Mini-Orca                                      │
│ Status: In Progress                                     │
├─────────────────────────────────────────────────────────┤
│ [Planning] [Coding] [Testing] [Review] [Approval]       │
├─────────────────────────────────────────────────────────┤
│                                                         │
│ [Start] [Pause] [Reset]    [Approve] [Reject]          │
│                                                         │
├─────────────────────────────────────────────────────────┤
│ Code Changes:                                           │
│ <Code Display Area>                                     │
│                                                         │
├─────────────────────────────────────────────────────────┤
│ Configuration:                                          │
│ Model for Planning: [Model Selector]                    │
│ Model for Coding:   [Model Selector]                    │
│ Model for Testing:  [Model Selector]                    │
│ Model for Review:   [Model Selector]                    │
└─────────────────────────────────────────────────────────┘
```

This dashboard will serve as the primary interface for users to monitor and interact with the automated workflow system, providing visibility into each step of the code generation process while maintaining human oversight and control.