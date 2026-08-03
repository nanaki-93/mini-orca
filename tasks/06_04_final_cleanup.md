# Task 6.4: Final Cleanup & Verification

## Goal
Final cleanup, build verification, and end-to-end testing to ensure the simplified app works correctly.

## Detailed Steps

### Step 1: Remove Unused Imports
Run `goimports` or manually remove unused imports across all modified files:
```bash
goimports -w $(find . -name "*.go" -not -path "./.git/*")
```

### Step 2: Format Code
```bash
go fmt ./...
```

### Step 3: Vet Code
```bash
go vet ./...
```
Fix any issues reported.

### Step 4: Build the Daemon
```bash
go build -o mini-orca-daemon ./cmd/daemon
```
Verify no compilation errors.

### Step 5: Run All Unit Tests
```bash
go test ./... -v
```
Fix any failing tests.

### Step 6: Check for Remaining Planning References
```bash
grep -r -i "planning" --include="*.go" --include="*.html" --include="*.md" --include="*.yaml" .
```
Review all results and remove/update any that are still relevant.

### Step 7: Check for Remaining Plan/PlanUnit References
```bash
grep -r "PlanUnit\|Plan\b" --include="*.go" . | grep -v "DEPRECATED" | grep -v "_test.go"
```
Remove any remaining references (deprecated methods are OK).

### Step 8: Verify State Store
```bash
go run -exec "echo" ./cmd/daemon 2>&1 | head -20
```
Verify the daemon starts without errors (dry run).

### Step 9: Smoke Test (if LLM available)
```bash
# Start the daemon
./mini-orca-daemon &
DAEMON_PID=$!

# Wait for startup
sleep 3

# Create a session
curl -s -X POST http://localhost:8080/api/sessions \
  -H "Content-Type: application/json" \
  -d '{
    "feature_request": "Add a hello world handler",
    "project_path": ".",
    "project_type": "go"
  }'

# Check session status
curl -s http://localhost:8080/api/sessions

# Stop the daemon
kill $DAEMON_PID
```

### Step 10: Verify API Endpoints
Test all remaining endpoints:
```bash
# Health check
curl -s http://localhost:8080/health

# Status
curl -s http://localhost:8080/status

# System info
curl -s http://localhost:8080/api/system/info

# List projects
curl -s http://localhost:8080/api/projects

# List sessions
curl -s http://localhost:8080/api/sessions
```

### Step 11: Verify No Breaking Changes to Stable APIs
The following APIs should remain unchanged:
- `GET /health`
- `GET /status`
- `GET /api/system/info`
- `GET /api/projects`
- `POST /api/projects`
- `GET /api/projects/:id/files`
- `GET /api/projects/files/:path`
- Static file serving (`/static/`)
- Error pages (`/error/404`, `/error/500`)

### Step 12: Final Git Status Check
```bash
git status
```
Review all changes before committing.

## Acceptance Criteria
- [ ] `go build ./cmd/daemon/` succeeds with no errors
- [ ] `go vet ./...` reports no issues
- [ ] `go test ./...` passes all tests
- [ ] No references to `PhasePlanning` or `PhasePlanningReview` in non-test code
- [ ] No references to `Plan`, `PlanUnit`, `AtomicUnit` in non-test code (except deprecated methods)
- [ ] Session creation uses `feature_request` field
- [ ] Sessions start at `PhaseCoding`
- [ ] Phase tracker shows 4 phases
- [ ] All API endpoints respond correctly
- [ ] Documentation updated
- [ ] Release notes added

## Post-Implementation Checklist
1. Commit changes with descriptive message
2. Update version number in `internal/version/version.go`
3. Create release tag if ready
4. Notify team of breaking changes
5. Update any external documentation or demos
