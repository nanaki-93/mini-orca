function appData() {
    return {
        // State
        darkMode: true,
        session: null,
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
        selectedFile: null,
        selectedPhase: null,
        
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
        
        // Initialize
        init() {
            this.loadSession();
            this.loadConfig();
            this.loadSkills();
            
            // Refresh session periodically
            setInterval(() => this.loadSession(), 5000);
        },
        
        // Session Management
        async loadSession() {
            try {
                const response = await fetch('/api/session');
                this.session = await response.json();
                this.updatePhaseStatus();
            } catch (error) {
                console.error('Failed to load session:', error);
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
                await fetch('/api/config', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(newConfig)
                });
                this.loadConfig();
            } catch (error) {
                console.error('Failed to save config:', error);
            }
        },
        
        // Skills Management
        async loadSkills() {
            // In production, fetch from API
            this.skills = [
                { id: 1, name: 'solid_principles', type: 'knowledge', priority: 5, description: 'Apply SOLID principles' },
                { id: 2, name: 'clean_code', type: 'knowledge', priority: 5, description: 'Write clean code' },
                { id: 3, name: 'function_generation', type: 'tool', priority: 4, description: 'Generate functions' },
                // Add more skills...
            ];
        },
        
        async toggleSkill(skillId) {
            const skill = this.skills.find(s => s.id === skillId);
            if (skill) {
                skill.enabled = !skill.enabled;
            }
        },
        
        async addSkill(newSkill) {
            // In production, save to API
            this.skills.push({
                id: Date.now(),
                ...newSkill,
                enabled: true
            });
        },
        
        async deleteSkill(skillId) {
            this.skills = this.skills.filter(s => s.id !== skillId);
        },
        
        async exportSkills() {
            const config = this.skills.filter(s => s.enabled);
            const blob = new Blob([JSON.stringify(config, null, 2)], { type: 'application/json' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = 'skills-config.json';
            a.click();
        },
        
        async importSkills(file) {
            const text = await file.text();
            const skills = JSON.parse(text);
            skills.forEach(skill => {
                if (!this.skills.find(s => s.id === skill.id)) {
                    this.skills.push(skill);
                }
            });
        },
        
        // UI Helpers
        toggleDarkMode() {
            this.darkMode = !this.darkMode;
        },
        
        refreshFiles() {
            // Refresh file tree
            const fileTreeContent = document.getElementById('file-tree-content');
            if (fileTreeContent) {
                fileTreeContent.innerHTML = '<div class="text-xs text-gray-400 px-2 py-1">Refreshing...</div>';
                setTimeout(() => {
                    fileTreeContent.innerHTML = '<div class="text-xs text-gray-400 px-2 py-1">Files refreshed</div>';
                }, 1000);
            }
        },
        
        // Phase Actions
        async approvePlan() {
            try {
                await fetch('/api/approve', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        phase: 'planning_review',
                        message: 'Plan approved'
                    })
                });
                this.loadSession();
            } catch (error) {
                console.error('Failed to approve plan:', error);
            }
        },
        
        async requestChanges() {
            try {
                await fetch('/api/reject', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        phase: 'planning_review',
                        message: 'Changes requested'
                    })
                });
                this.loadSession();
            } catch (error) {
                console.error('Failed to request changes:', error);
            }
        },
        
        async approveSession() {
            try {
                await fetch('/api/approve', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        phase: 'human_review',
                        message: 'Session approved'
                    })
                });
                this.loadSession();
            } catch (error) {
                console.error('Failed to approve session:', error);
            }
        },
        
        async rejectSession() {
            try {
                await fetch('/api/reject', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        phase: 'human_review',
                        message: 'Session rejected'
                    })
                });
                this.loadSession();
            } catch (error) {
                console.error('Failed to reject session:', error);
            }
        },
        
        // Editor Actions
        async saveFunction() {
            const code = document.querySelector('textarea[x-model="code"]')?.value;
            if (code) {
                // Save function logic
                console.log('Saving function:', code);
            }
        },
        
        async formatCode() {
            // Format code logic
            console.log('Formatting code...');
        },
        
        async insertFunction() {
            const code = document.querySelector('textarea[x-model="functionCode"]')?.value;
            const name = document.querySelector('input[x-model="functionName"]')?.value;
            
            if (code && name) {
                try {
                    await fetch('/api/insert', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({
                            function_code: code,
                            file_path: 'main.go',
                            position: 'append',
                        })
                    });
                } catch (error) {
                    console.error('Failed to insert function:', error);
                }
            }
        },
        
        async generateWithAI() {
            // AI generation logic
            console.log('Generating with AI...');
        },
        
        async confirmInsertion() {
            // Confirm insertion logic
            console.log('Confirming insertion...');
        },
    };
}
