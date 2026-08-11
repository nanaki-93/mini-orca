/**
 * Mini-Orca Chat Client
 * Handles chat messaging, response display, streaming updates, and action buttons.
 */

(function () {
    'use strict';

    // ─── State ───────────────────────────────────────────────────────────────
    const ChatState = {
        isSending: false,
        currentPhase: '',
        focusedFile: null,
        messageHistory: []
    };

    // ─── DOM References ──────────────────────────────────────────────────────
    const DOM = {
        get messages() {
            return document.getElementById('chat-messages');
        },
        get input() {
            return document.getElementById('chat-input');
        },
        get sendBtn() {
            return document.getElementById('chat-send-btn');
        },
        get phaseIndicator() {
            return document.getElementById('chat-phase-indicator');
        },
        get inputContainer() {
            return document.getElementById('chat-input-container');
        }
    };

    // ─── Auto-resize Textarea ────────────────────────────────────────────────
    function autoResizeTextarea(textarea) {
        if (!textarea) return;
        textarea.addEventListener('input', function () {
            this.style.height = 'auto';
            this.style.height = Math.min(this.scrollHeight, 128) + 'px'; // max-h-32 = 128px
        });
    }

    // ─── Message Display ─────────────────────────────────────────────────────
    /**
     * Append a user message to the chat.
     */
    function appendUserMessage(content) {
        if (!DOM.messages) return;

        const messageEl = document.createElement('div');
        messageEl.className = 'chat-message flex items-start gap-2';
        messageEl.setAttribute('data-role', 'user');
        messageEl.setAttribute('data-timestamp', new Date().toISOString());

        messageEl.innerHTML = `
            <div class="chat-avatar w-6 h-6 rounded-full bg-dark-600 flex items-center justify-center shrink-0">
                <svg class="w-3.5 h-3.5 text-text-secondary" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                </svg>
            </div>
            <div class="flex-1 min-w-0 max-w-[85%]">
                <div class="chat-bubble rounded-lg px-3 py-2 text-sm bg-dark-600 text-text-primary rounded-tr-none">
                    <div class="chat-content">
                        <p>${escapeHtml(content)}</p>
                    </div>
                </div>
                <div class="chat-meta mt-1 flex items-center gap-2 justify-end">
                    <span class="text-[10px] text-text-secondary">${formatTime(new Date())}</span>
                </div>
            </div>
        `;

        DOM.messages.appendChild(messageEl);
        scrollToBottom();
    }

    /**
     * Append an assistant message to the chat.
     */
    function appendAssistantMessage(content, phase, agent) {
        if (!DOM.messages) return;

        const messageEl = document.createElement('div');
        messageEl.className = 'chat-message flex items-start gap-2';
        messageEl.setAttribute('data-role', 'assistant');
        messageEl.setAttribute('data-phase', phase || '');
        messageEl.setAttribute('data-timestamp', new Date().toISOString());

        const agentBadge = agent
            ? `<span class="px-1.5 py-0.5 text-[10px] font-medium rounded-full bg-purple-600/20 text-purple-400">${agent}</span>`
            : '';

        messageEl.innerHTML = `
            <div class="chat-avatar w-6 h-6 rounded-full bg-blue-600 flex items-center justify-center shrink-0">
                <svg class="w-3.5 h-3.5 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
                </svg>
            </div>
            <div class="flex-1 min-w-0 max-w-[85%]">
                <div class="chat-phase-badge flex items-center gap-1.5 mb-1">
                    <span class="px-1.5 py-0.5 text-[10px] font-medium rounded-full bg-blue-600/20 text-blue-400">${phase || 'coding'}</span>
                    ${agentBadge}
                </div>
                <div class="chat-bubble rounded-lg px-3 py-2 text-sm bg-dark-700 text-text-primary rounded-tl-none">
                    <div class="chat-content whitespace-pre-wrap">${escapeHtml(content)}</div>
                </div>
                <div class="chat-meta mt-1 flex items-center gap-2">
                    <span class="text-[10px] text-text-secondary">${formatTime(new Date())}</span>
                </div>
            </div>
        `;

        DOM.messages.appendChild(messageEl);
        scrollToBottom();
    }

    /**
     * Show loading indicator in chat.
     */
    function showLoading() {
        if (!DOM.messages) return;

        const loadingEl = document.createElement('div');
        loadingEl.id = 'chat-loading-indicator';
        loadingEl.className = 'chat-message flex items-start gap-2';
        loadingEl.setAttribute('data-role', 'assistant');

        loadingEl.innerHTML = `
            <div class="chat-avatar w-6 h-6 rounded-full bg-blue-600 flex items-center justify-center shrink-0">
                <svg class="w-3.5 h-3.5 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
                </svg>
            </div>
            <div class="flex-1 min-w-0 max-w-[85%]">
                <div class="chat-bubble rounded-lg rounded-tl-none px-3 py-2 bg-dark-700">
                    <div class="flex items-center gap-1.5">
                        <div class="w-1.5 h-1.5 rounded-full bg-blue-400 animate-bounce" style="animation-delay: 0ms"></div>
                        <div class="w-1.5 h-1.5 rounded-full bg-blue-400 animate-bounce" style="animation-delay: 150ms"></div>
                        <div class="w-1.5 h-1.5 rounded-full bg-blue-400 animate-bounce" style="animation-delay: 300ms"></div>
                    </div>
                </div>
            </div>
        `;

        DOM.messages.appendChild(loadingEl);
        scrollToBottom();
    }

    /**
     * Remove loading indicator.
     */
    function removeLoading() {
        const loadingEl = document.getElementById('chat-loading-indicator');
        if (loadingEl) {
            loadingEl.remove();
        }
    }

    // ─── Phase Updates ───────────────────────────────────────────────────────
    /**
     * Update the phase indicator in the chat header.
     */
    function updatePhase(phase) {
        if (!DOM.phaseIndicator) return;
        ChatState.currentPhase = phase;
        DOM.phaseIndicator.textContent = phase || 'coding';
        DOM.phaseIndicator.setAttribute('aria-label', `Current phase: ${phase || 'coding'}`);
    }

    // ─── Streaming Updates ───────────────────────────────────────────────────
    /**
     * Handle streaming updates from the pipeline.
     * This can be called when SSE or polling provides phase updates.
     */
    function handleStreamUpdate(data) {
        if (!data) return;

        // Update phase if provided
        if (data.phase) {
            updatePhase(data.phase);
        }

        // Append content if provided
        if (data.content) {
            if (data.role === 'user') {
                appendUserMessage(data.content);
            } else {
                appendAssistantMessage(data.content, data.phase, data.agent);
            }
        }

        // Handle actions if provided
        if (data.actions) {
            appendActionButtons(data.actions);
        }
    }

    // ─── Action Buttons (Accept/Edit/Refuse) ─────────────────────────────────
    /**
     * Append action buttons to the chat for user decision points.
     */
    function appendActionButtons(actions) {
        if (!DOM.messages) return;

        const actionsEl = document.createElement('div');
        actionsEl.className = 'chat-actions flex flex-wrap gap-2 mt-2 ml-8';

        actions.forEach(action => {
            const btn = document.createElement('button');
            btn.className = `px-3 py-1.5 text-xs font-medium rounded-lg transition-colors ${getActionButtonClass(action.type)}`;
            btn.textContent = action.label;
            btn.setAttribute('data-action', action.type);
            btn.addEventListener('click', () => handleAction(action));
            actionsEl.appendChild(btn);
        });

        DOM.messages.appendChild(actionsEl);
        scrollToBottom();
    }

    /**
     * Get CSS class for action button based on type.
     */
    function getActionButtonClass(type) {
        switch (type) {
            case 'accept':
                return 'bg-green-600 hover:bg-green-500 text-white';
            case 'edit':
                return 'bg-blue-600 hover:bg-blue-500 text-white';
            case 'refuse':
                return 'bg-red-600 hover:bg-red-500 text-white';
            case 'retry':
                return 'bg-yellow-600 hover:bg-yellow-500 text-white';
            default:
                return 'bg-dark-600 hover:bg-dark-500 text-text-primary';
        }
    }

    /**
     * Handle action button click.
     */
    function handleAction(action) {
        if (ChatState.isSending) return;

        // Remove action buttons
        const actionsEl = DOM.messages?.querySelector('.chat-actions');
        if (actionsEl) {
            actionsEl.remove();
        }

        // Send action response
        sendAction(action.type, action.value);
    }

    /**
     * Send an action response to the server.
     */
    async function sendAction(actionType, value) {
        try {
            const response = await fetch('/api/chat/action', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    action: actionType,
                    value: value || null
                })
            });

            if (!response.ok) {
                throw new Error(`HTTP ${response.status}`);
            }

            const data = await response.json();
            handleStreamUpdate(data);
        } catch (error) {
            console.error('Action error:', error);
            appendAssistantMessage(`Error processing action: ${error.message}`, 'error');
        }
    }

    // ─── Message Sending ─────────────────────────────────────────────────────
    /**
     * Send a chat message to the server.
     */
    async function sendMessage() {
        const input = DOM.input;
        if (!input) return;

        const message = input.value.trim();
        if (!message || ChatState.isSending) return;

        if (!ChatState.focusedFile) {
            alert('Please open a file before sending a message.');
            return;
        }

        // Set sending state
        ChatState.isSending = true;
        setInputDisabled(true);

        // Append user message
        appendUserMessage(message);

        // Clear input
        input.value = '';
        input.style.height = 'auto';

        // Show loading
        showLoading();

        try {
            const response = await fetch('/api/chat/message', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    message: message,
                    file_path: ChatState.focusedFile || null
                })
            });

            removeLoading();

            if (!response.ok) {
                const errorData = await response.json().catch(() => ({}));
                throw new Error(errorData.error || `HTTP ${response.status}`);
            }

            const data = await response.json();

            // Update phase from response
            if (data.phase) {
                updatePhase(data.phase);
            }

            // Append assistant response
            if (data.output) {
                appendAssistantMessage(data.output, data.phase, data.agent);
            }

            // Handle actions if provided
            if (data.actions) {
                appendActionButtons(data.actions);
            }

            // Update message history
            ChatState.messageHistory.push(
                { role: 'user', content: message, timestamp: new Date() },
                { role: 'assistant', content: data.output, phase: data.phase, timestamp: new Date() }
            );
        } catch (error) {
            removeLoading();
            appendAssistantMessage(`Error: ${error.message}`, 'error');
            console.error('Chat error:', error);
        } finally {
            ChatState.isSending = false;
            setInputDisabled(false);
            input.focus();
        }
    }

    // ─── Helpers ─────────────────────────────────────────────────────────────
    /**
     * Set input disabled state.
     */
    function setInputDisabled(disabled) {
        const input = DOM.input;
        const sendBtn = DOM.sendBtn;
        if (input) input.disabled = disabled;
        if (sendBtn) sendBtn.disabled = disabled;
    }

    /**
     * Scroll chat to bottom.
     */
    function scrollToBottom() {
        if (DOM.messages) {
            DOM.messages.scrollTop = DOM.messages.scrollHeight;
        }
    }

    /**
     * Escape HTML to prevent XSS.
     */
    function escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    /**
     * Format time for display.
     */
    function formatTime(date) {
        return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    }

    /**
     * Clear file context indicator.
     */
    window.clearFileContext = function () {
        ChatState.focusedFile = null;
        const contextEl = document.querySelector('.chat-file-context');
        if (contextEl) {
            contextEl.remove();
        }
        // Update hint text
        const hintEl = document.querySelector('#chat-input-container .mt-1.5 span:last-child');
        if (hintEl) {
            hintEl.remove();
        }
    };

    /**
     * Set file context for chat.
     */
    window.setFileContext = function (filePath) {
        ChatState.focusedFile = filePath;
    };

    /**
     * Load chat history from server.
     */
    async function loadChatHistory() {
        try {
            const response = await fetch('/api/chat/history');
            if (!response.ok) return;

            const history = await response.json();
            ChatState.messageHistory = history;

            // Clear and rebuild messages
            if (DOM.messages) {
                DOM.messages.innerHTML = '';

                // Add welcome message
                const welcomeEl = document.createElement('div');
                welcomeEl.className = 'chat-message flex items-start gap-2';
                welcomeEl.innerHTML = `
                    <div class="chat-avatar w-6 h-6 rounded-full bg-blue-600 flex items-center justify-center shrink-0">
                        <svg class="w-3.5 h-3.5 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
                        </svg>
                    </div>
                    <div class="flex-1 min-w-0 max-w-[85%]">
                        <div class="chat-bubble rounded-lg rounded-tl-none px-3 py-2 text-sm bg-dark-700 text-text-primary">
                            <div class="chat-content">
                                <p>Hello! I'm Mini-Orca, your AI development assistant. How can I help you today?</p>
                            </div>
                        </div>
                    </div>
                `;
                DOM.messages.appendChild(welcomeEl);

                // Rebuild history (skip welcome message)
                for (const entry of history) {
                    if (entry.role === 'user') {
                        appendUserMessage(entry.content);
                    } else {
                        appendAssistantMessage(entry.content, entry.phase);
                    }
                }
            }
        } catch (error) {
            console.error('Failed to load chat history:', error);
        }
    }

    // ─── HTMX Event Handlers ─────────────────────────────────────────────────
    /**
     * Handle HTMX response swapping for chat messages.
     */
    document.addEventListener('htmx:afterSwap', function (event) {
        if (event.detail.target.id === 'chat-messages' ||
            event.detail.target.closest('#chat-messages')) {
            scrollToBottom();
        }
    });

    /**
     * Handle HTMX errors in chat.
     */
    document.addEventListener('htmx:responseError', function (event) {
        console.error('HTMX chat error:', event.detail);
        removeLoading();
        appendAssistantMessage('Network error. Please try again.', 'error');
    });

    // ─── Initialization ──────────────────────────────────────────────────────
    function init() {
        // Setup textarea auto-resize
        if (DOM.input) {
            autoResizeTextarea(DOM.input);

            // Override HTMX Enter key behavior to use our sendMessage function
            DOM.input.addEventListener('keydown', function(event) {
                if (event.key === 'Enter' && !event.shiftKey) {
                    event.preventDefault();
                    event.stopPropagation();
                    sendMessage();
                }
            });
        }

        // Setup send button
        if (DOM.sendBtn) {
            DOM.sendBtn.onclick = (e) => {
                e.preventDefault();
                sendMessage();
            };
        }

        // Load chat history
        loadChatHistory();

        // Listen for phase updates via HTMX
        document.addEventListener('htmx:configRequest', function (event) {
            // Update phase when phase render endpoint is called
            if (event.detail.requestConfig?.target?.id === 'chat-phase-indicator') {
                updatePhase(event.detail.elt?.textContent?.trim() || '');
            }
        });

        console.log('[Mini-Orca] Chat client initialized');
    }

    // Initialize when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();
