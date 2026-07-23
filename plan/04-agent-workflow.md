# Agent Workflow Specification

## Overview
This document outlines the multi-agent workflow for the mini-orca project, defining the roles, responsibilities, and interactions between agents in each phase of the development process.

## Phase Structure

### 1. Planning Phase
#### Agent: Planner
#### Responsibilities:
- Analyze user requirements and project specifications
- Create detailed development plan with milestones
- Break down requirements into implementable tasks
- Define code structure and architecture decisions
- Generate project roadmap with timelines

#### Output:
- Project plan document (JSON format)
- Task breakdown with dependencies
- Architecture decisions and design patterns
- Implementation timeline estimates

#### Configuration:
- Model: GPT-4, Claude, or other planning-focused LLMs
- Parameters: Temperature (0.3-0.7), Max tokens (2000+)
- Prompt template: Requirements analysis, task decomposition

### 2. Coding Phase
#### Agent: Coder
#### Responsibilities:
- Generate specific code functions based on plan
- Follow project architecture and coding standards
- Implement features as defined in the plan
- Handle code generation with proper syntax and structure

#### Output:
- Generated code files (JavaScript/TypeScript)
- Code documentation and comments
- Unit test stubs for new functions

#### Configuration:
- Model: CodeLlama, GPT-3.5, or other code generation models
- Parameters: Temperature (0.2-0.4), Max tokens (1500+)
- Prompt template: Code generation, function implementation

### 3. Testing Phase
#### Agent: Tester
#### Responsibilities:
- Execute tests on generated code
- Validate code functionality and performance
- Check for edge cases and error handling
- Generate test reports and coverage metrics

#### Output:
- Test results (pass/fail, coverage percentage)
- Error logs and stack traces
- Performance benchmarks
- Test report document

#### Configuration:
- Model: GPT-4, CodeBERT, or testing-focused LLMs
- Parameters: Temperature (0.1-0.3), Max tokens (1000+)
- Prompt template: Test execution, code validation

### 4. Review Phase
#### Agent: Reviewer
#### Responsibilities:
- Perform code quality review
- Check for security vulnerabilities and best practices
- Validate adherence to project standards
- Suggest improvements and optimizations

#### Output:
- Code review comments and suggestions
- Quality metrics (code complexity, maintainability)
- Security vulnerability assessments
- Review summary report

#### Configuration:
- Model: GPT-4, SonarQube, or code review models
- Parameters: Temperature (0.2-0.5), Max tokens (1500+)
- Prompt template: Code review, security checks

### 5. User Approval Phase
#### Agent: User Interface
#### Responsibilities:
- Display code changes for user review
- Collect user feedback and approval/rejection
- Handle modification requests from users
- Update project state based on user decisions

#### Output:
- Change summary for user review
- Approval/rejection status
- User feedback comments

#### Configuration:
- Model: Not applicable (user-driven decision)
- Parameters: None (user interaction only)

## Workflow Execution

### Phase 1: Planning
1. User provides requirements and project specs
2. Planner analyzes and creates project plan
3. User reviews and confirms the plan
4. Plan validation (if rejected, return to planning)

### Phase 2: Coding
1. Coder generates code for specific function
2. Code is validated against project structure
3. Function is added to project scope

### Phase 3: Testing
1. Tester executes tests on generated code
2. Test results are analyzed and reported
3. Issues are logged for potential fixes

### Phase 4: Review
1. Reviewer analyzes code quality and security
2. Comments and suggestions are generated
3. Review report is created for user review

### Phase 5: User Approval
1. User reviews all changes and feedback
2. User approves or rejects the changes
3. If rejected, process returns to appropriate phase

## Agent Interactions

### Data Flow Between Agents
1. Planning agent outputs plan to coders for implementation
2. Coding agent outputs code artifacts to testers
3. Testing agent outputs test results to reviewers
4. Review agent outputs review comments to users

### Error Handling and Retry Logic
- If coding fails, return to planning or fix the plan
- If testing fails, generate detailed error report
- If review fails, provide suggestions for improvement
- User rejection triggers appropriate phase re-execution

## Configuration Management

### Model Selection per Phase
Each agent can use different models with specific parameters:

#### Planning Phase Models:
- GPT-4 (default)
- Claude
- Gemini Pro

#### Coding Phase Models:
- CodeLlama
- GPT-3.5
- Tabnine

#### Testing Phase Models:
- GPT-4
- CodeBERT
- DeepSeek

#### Review Phase Models:
- GPT-4
- SonarQube
- CodeGemma

### Configuration Parameters
Each model can be configured with:
- Temperature (0.0-1.0) - Controls randomness
- Max tokens - Maximum output length
- Top-p - Token sampling parameter
- Frequency penalty - Reduces repetition
- Presence penalty - Encourages diversity

## Implementation Requirements

### API Endpoints
- `/api/workflow/start` - Start new workflow
- `/api/workflow/status` - Get current workflow status
- `/api/workflow/approve` - Approve/reject changes
- `/api/config/models` - Get/set model configurations

### Data Structures
- Plan object with tasks and dependencies
- Code artifact with metadata and version info
- Test results with pass/fail metrics
- Review comments with severity levels

### Error Handling
- Graceful degradation when models are unavailable
- Fallback strategies for failed phases
- User-friendly error messages and recovery options

## Future Extensibility
- Support for custom agent implementations
- Plugin architecture for new models
- Integration with version control systems
- Support for native app development (Kotlin)