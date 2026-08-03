# Task 5.3: Update Templates

## Goal
Update and delete HTML templates to reflect the new 4-phase workflow. Remove planning templates and update remaining ones.

## Files to Delete
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/api/templates/phases/planning.html`
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/api/templates/phases/planning-review.html`

## Files to Modify
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/api/templates/phases/coding.html`
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/api/templates/phases/testing.html`
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/api/templates/phases/review.html`
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/api/templates/phases/human-review.html`
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/api/templates/components/phase-tracker.html`
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/api/templates/components/dashboard.html`
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/api/templates/ide.html`
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/api/templates/components/session-controls.html`
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/api/templates/components/project-modal.html`
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/api/templates/base.html` (if needed)

## Detailed Steps

### Step 1: Delete Planning Templates
```bash
rm internal/api/templates/phases/planning.html
rm internal/api/templates/phases/planning-review.html
```

### Step 2: Update phases/coding.html
**Change the heading from:**
```html
<h2>Coding Phase</h2>
<p>Implementing plan units...</p>
```

**To:**
```html
<h2>Coding Phase</h2>
<p>Generating code for: {{ .FeatureRequest }}</p>
<p>Target file: {{ .CodingPhaseData.CurrentFile }}</p>
```

**Remove any references to:**
- Unit queue / unit list
- Plan units display
- "Units completed: X/Y"
- Per-unit file operations

**Add:**
- Feature request display
- Generated code preview area
- Target file indicator
- Code line count / size

### Step 3: Update phases/testing.html
**Change the heading from:**
```html
<h2>Testing Phase</h2>
<p>Testing plan units...</p>
```

**To:**
```html
<h2>Testing Phase</h2>
<p>Testing generated code...</p>
```

**Remove any references to:**
- Per-unit test results
- Plan unit association

**Keep:**
- Test output display
- Coverage display
- Test results summary

### Step 4: Update phases/review.html
**Change the heading from:**
```html
<h2>Review Phase</h2>
<p>Reviewing plan units...</p>
```

**To:**
```html
<h2>Review Phase</h2>
<p>Reviewing generated code...</p>
```

**Remove any references to:**
- Plan comparison
- Per-unit review

**Keep:**
- Code review display
- Issues list
- Score display
- Suggestions

### Step 5: Update phases/human-review.html
**This template mostly stays the same, but update the content:**

**Change from:**
```html
<h2>Human Review</h2>
<p>Reviewing plan and units...</p>
```

**To:**
```html
<h2>Human Review</h2>
<p>Reviewing feature implementation...</p>
```

**Update the output display to show:**
- Feature request
- Generated code
- Target file
- Review summary (if available)

### Step 6: Update components/phase-tracker.html
**Change the phase list from 6 to 4:**

**From:**
```html
<div class="phase-item" data-phase="planning">
<div class="phase-item" data-phase="planning_review">
<div class="phase-item" data-phase="coding">
<div class="phase-item" data-phase="testing">
<div class="phase-item" data-phase="review">
<div class="phase-item" data-phase="human_review">
```

**To:**
```html
<div class="phase-item" data-phase="coding">
<div class="phase-item" data-phase="testing">
<div class="phase-item" data-phase="review">
<div class="phase-item" data-phase="human_review">
```

**Update the connector lines to only connect 4 phases.**

### Step 7: Update components/dashboard.html
**Update the dashboard to show 4 phases instead of 6.**

### Step 8: Update ide.html
**Update the session creation form:**

**From:**
```html
<form id="create-session-form">
  <input name="goal" placeholder="Describe your goal..." />
  <input name="project_path" placeholder="Project path..." />
  <button type="submit">Create Session</button>
</form>
```

**To:**
```html
<form id="create-session-form">
  <div class="form-group">
    <label for="feature-request">Feature Request</label>
    <textarea name="feature_request" id="feature-request" 
              placeholder="Describe the feature you want to add..." required></textarea>
  </div>
  <div class="form-group">
    <label for="project-path">Project Path</label>
    <input type="text" name="project_path" id="project-path" 
           placeholder="/path/to/existing/project" required />
  </div>
  <button type="submit">Start Implementation</button>
</form>
```

### Step 9: Update components/session-controls.html
**Remove any "Start Planning" button.**

**Update the status display to show the 4 phases.**

### Step 10: Update components/project-modal.html
**Ensure the modal clearly says "Open Existing Project" instead of "Create Project".**

## Verification
- `planning.html` and `planning-review.html` are deleted
- `phase-tracker.html` shows 4 phases
- `ide.html` has feature request input instead of goal input
- All remaining templates reference `FeatureRequest` instead of `Goal` where appropriate
- No template references `Plan` or `PlanUnit`
