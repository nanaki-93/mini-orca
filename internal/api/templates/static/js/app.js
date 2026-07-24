// Mini-Orca IDE Application
// HTMX + Alpine.js frontend for real-time monitoring

/**
 * Toast notification system for user feedback
 */
function showToast(message, type = 'info') {
    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    toast.textContent = message;
    toast.setAttribute('role', 'alert');
    toast.setAttribute('aria-live', 'polite');
    document.body.appendChild(toast);
    
    setTimeout(() => {
        toast.style.opacity = '0';
        toast.style.transform = 'translateX(100%)';
        setTimeout(() => toast.remove(), 300);
    }, 3000);
}

/**
 * Debounce utility for performance optimization
 */
function debounce(func, wait) {
    let timeout;
    return function executedFunction(...args) {
        const later = () => {
            clearTimeout(timeout);
            func(...args);
        };
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
    };
}

/**
 * Main application data for Alpine.js
 */
function appData() {
    return {
        // State
        darkMode: true,
        session: null,
        loading: false,
        error: null,
        phases: [
            { id: 'planning', name: 'Planning', icon: '🧠', status: 'pending' },
            { id: 'planning_review', name: 'Review', icon: '👁️', status: 'pending' },
            { id: 'coding', name: 'Coding', icon: '💻', status: 'pending' },
            { id: 'testing', name: 'Testing', icon: '🧪', status: 'pending' },
            { id: 'review', name: 'Review', icon: '🔍', status: 'pending' },
            { id: 'human_review', name: 'Final', icon: '✅', status: 'pending' },
        ],
        
        // UI State
        showConfigModal: false,
        showSkillsModal: false,
        selectedFile: null,
        selectedPhase: null,
        activityLog: [],
        
        // Config
        config: {
            models: {},
            agents: {},
            server: {}
        },
        
        // Skills
        skills: [],
        selectedSkills: [],
        agentSkills: {},
        selectedAgent: 'planner',
        
        // Editor
        code: '',
        functionName: '',
        functionCode: '',
        
        // Initialize
        init() {
            this.loadSession();
            this.loadConfig();
            this.loadSkills();
            
            // Refresh session periodically (debounced)
            setInterval(() => this.loadSession(), 5000);
            
            // Listen for HTMX events
            document.body.addEventListener('htmx:afterRequest', (event) => {
                this.loading = false;
            });
            
            document.body.addEventListener('htmx:beforeRequest', (event) => {
                this.loading = true;
            });
            
            // Re-evaluate Alpine.js in HTMX-loaded content
            document.body.addEventListener('htmx:afterSettle', (event) => {
                if (window.Alpine) {
                    window.Alpine.initTree(event.detail.target);
                }
            });
        },
        
        // Session Management
        async loadSession() {
            try {
                const response = await fetch('/api/session');
                if (!response.ok) throw new Error('Failed to load session');
                this.session = await response.json();
                this.updatePhaseStatus();
                this.error = null;
            } catch (error) {
                console.error('Failed to load session:', error);
                this.error = 'Connection error. Retrying...';
            }
        },
        
        updatePhaseStatus() {
            if (!this.session) return;
            
            const phaseOrder = ['planning', 'planning_review', 'coding', 'testing', 'review', 'human_review'];
            const currentPhase = this.session.phase;
            const currentIndex = phaseOrder.indexOf(currentPhase);
            
            this.phases.forEach((phase, index) => {
                if (index < currentIndex) {
                    phase.status = 'completed';
                } else if (index === currentIndex) {
                    phase.status = 'current';
                } else {
                    phase.status = 'pending';
                }
            });
        },
        
        // Config Management
        async loadConfig() {
            try {
                const response = await fetch('/api/config');
                this.config = await response.json();
            } catch (error) {
                console.error('Failed to load config:', error);
            }
        },
        
        async saveConfig(newConfig) {
            try {
                const response = await fetch('/api/config', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(newConfig)
                });
                if (!response.ok) throw new Error('Failed to save config');
                await this.loadConfig();
                showToast('Configuration saved', 'success');
            } catch (error) {
                console.error('Failed to save config:', error);
                showToast('Failed to save config', 'error');
            }
        },
        
        // Skills Management
        async loadSkills() {
            try {
                const response = await fetch('/api/skills');
                this.skills = await response.json();
            } catch (error) {
                console.error('Failed to load skills:', error);
                // Fallback to defaults
                this.skills = [
                    { id: 1, name: 'solid_principles', type: 'knowledge', priority: 5, description: 'Apply SOLID principles' },
                    { id: 2, name: 'clean_code', type: 'knowledge', priority: 5, description: 'Write clean code' },
                    { id: 3, name: 'function_generation', type: 'tool', priority: 4, description: 'Generate functions' },
                ];
            }
        },
        
        async toggleSkill(skillId) {
            const skill = this.skills.find(s => s.id === skillId);
            if (skill) {
                skill.enabled = !skill.enabled;
            }
        },
        
        async addSkill(newSkill) {
            try {
                const response = await fetch('/api/skills', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(newSkill)
                });
                if (!response.ok) throw new Error('Failed to add skill');
                await this.loadSkills();
                showToast('Skill added', 'success');
            } catch (error) {
                console.error('Failed to add skill:', error);
                showToast('Failed to add skill', 'error');
            }
        },
        
        async deleteSkill(skillId) {
            try {
                const response = await fetch(`/api/skills/${skillId}`, {
                    method: 'DELETE'
                });
                if (!response.ok) throw new Error('Failed to delete skill');
                await this.loadSkills();
                showToast('Skill deleted', 'success');
            } catch (error) {
                console.error('Failed to delete skill:', error);
                showToast('Failed to delete skill', 'error');
            }
        },
        
        async exportSkills() {
            const config = this.skills.filter(s => s.enabled);
            const blob = new Blob([JSON.stringify(config, null, 2)], { type: 'application/json' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = 'skills-config.json';
            a.click();
            URL.revokeObjectURL(url);
            showToast('Skills exported', 'success');
        },
        
        async importSkills(file) {
            try {
                const text = await file.text();
                const skills = JSON.parse(text);
                skills.forEach(skill => {
                    if (!this.skills.find(s => s.id === skill.id)) {
                        this.skills.push(skill);
                    }
                });
                await this.saveSkills();
                showToast('Skills imported', 'success');
            } catch (error) {
                console.error('Failed to import skills:', error);
                showToast('Failed to import skills', 'error');
            }
        },
        
        async saveSkills() {
            try {
                await fetch('/api/skills', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(this.skills)
                });
                showToast('Skills saved', 'success');
            } catch (error) {
                console.error('Failed to save skills:', error);
                showToast('Failed to save skills', 'error');
            }
        },
        
        // UI Helpers
        toggleDarkMode() {
            this.darkMode = !this.darkMode;
            document.documentElement.className = this.darkMode ? 'dark' : 'light';
            localStorage.setItem('darkMode', this.darkMode);
        },
        
        refreshFiles: debounce(function() {
            const fileTreeContent = document.getElementById('file-tree-content');
            if (fileTreeContent) {
                fileTreeContent.innerHTML = '<div class="text-xs text-gray-400 px-2 py-1">Refreshing...</div>';
                setTimeout(() => {
                    fileTreeContent.innerHTML = '<div class="text-xs text-gray-400 px-2 py-1">Files refreshed</div>';
                }, 1000);
            }
        }, 500),
        
        // Phase Actions
        async approvePlan() {
            try {
                const response = await fetch('/api/approve', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        phase: 'planning_review',
                        message: 'Plan approved'
                    })
                });
                if (!response.ok) throw new Error('Failed to approve');
                await this.loadSession();
                showToast('Plan approved', 'success');
            } catch (error) {
                console.error('Failed to approve plan:', error);
                showToast('Failed to approve plan', 'error');
            }
        },
        
        async requestChanges() {
            try {
                const response = await fetch('/api/reject', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        phase: 'planning_review',
                        message: 'Changes requested'
                    })
                });
                if (!response.ok) throw new Error('Failed to reject');
                await this.loadSession();
                showToast('Changes requested', 'warning');
            } catch (error) {
                console.error('Failed to request changes:', error);
                showToast('Failed to request changes', 'error');
            }
        },
        
        async approveSession() {
            try {
                const response = await fetch('/api/approve', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        phase: 'human_review',
                        message: 'Session approved'
                    })
                });
                if (!response.ok) throw new Error('Failed to approve');
                await this.loadSession();
                showToast('Session approved', 'success');
            } catch (error) {
                console.error('Failed to approve session:', error);
                showToast('Failed to approve session', 'error');
            }
        },
        
        async rejectSession() {
            try {
                const response = await fetch('/api/reject', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        phase: 'human_review',
                        message: 'Session rejected'
                    })
                });
                if (!response.ok) throw new Error('Failed to reject');
                await this.loadSession();
                showToast('Session rejected', 'warning');
            } catch (error) {
                console.error('Failed to reject session:', error);
                showToast('Failed to reject session', 'error');
            }
        },
        
        // Editor Actions
        async saveFunction() {
            const code = document.querySelector('textarea[x-model="code"]')?.value;
            if (code) {
                try {
                    await fetch('/api/save', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ code })
                    });
                    showToast('Code saved', 'success');
                } catch (error) {
                    console.error('Failed to save code:', error);
                    showToast('Failed to save code', 'error');
                }
            }
        },
        
        async formatCode() {
            try {
                const response = await fetch('/api/format', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ code: this.code })
                });
                if (!response.ok) throw new Error('Failed to format');
                const result = await response.json();
                this.code = result.formatted;
                showToast('Code formatted', 'success');
            } catch (error) {
                console.error('Failed to format code:', error);
                showToast('Failed to format code', 'error');
            }
        },
        
        async insertFunction() {
            const code = document.querySelector('textarea[x-model="functionCode"]')?.value;
            const name = document.querySelector('input[x-model="functionName"]')?.value;
            
            if (!code || !name) {
                showToast('Please provide both function name and code', 'warning');
                return;
            }
            
            try {
                const response = await fetch('/api/insert', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        function_code: code,
                        file_path: 'main.go',
                        position: 'append',
                    })
                });
                if (!response.ok) throw new Error('Failed to insert');
                await this.loadSession();
                showToast('Function inserted', 'success');
            } catch (error) {
                console.error('Failed to insert function:', error);
                showToast('Failed to insert function', 'error');
            }
        },
        
        // Utility
        getPhaseName(phase) {
            const phaseMap = {
                'planning': 'Planning',
                'planning_review': 'Planning Review',
                'coding': 'Coding',
                'testing': 'Testing',
                'review': 'Review',
                'human_review': 'Human Review',
                'complete': 'Complete',
                'cancelled': 'Cancelled'
            };
            return phaseMap[phase] || phase;
        },
        
        getPhaseColor(status) {
            const colorMap = {
                'completed': 'text-green-400',
                'current': 'text-blue-400',
                'pending': 'text-gray-500'
            };
            return colorMap[status] || 'text-gray-500';
        }
    };
}

// Initialize dark mode from localStorage
(function() {
    const saved = localStorage.getItem('darkMode');
    if (saved !== null) {
        document.documentElement.className = saved === 'true' ? 'dark' : 'light';
    }
})();
