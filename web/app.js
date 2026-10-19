(function () {
  'use strict';

  // DOM Elements
  const scrollArea = document.getElementById('chat-scroll-area');
  const messagesContainer = document.getElementById('chat-messages');
  const welcomeHero = document.getElementById('welcome-hero');
  const typingIndicator = document.getElementById('typing-indicator');
  const scrollBottomBtn = document.getElementById('scroll-bottom-btn');

  const chatForm = document.getElementById('chat-form');
  const messageInput = document.getElementById('message-input');
  const sendButton = document.getElementById('send-button');
  const charCounter = document.getElementById('char-counter');

  const statusDot = document.querySelector('.status-dot');
  const statusLabel = document.getElementById('status-label');

  const themeToggleBtn = document.getElementById('btn-theme-toggle');
  const clearChatBtn = document.getElementById('btn-clear-chat');
  const suggestionCards = document.querySelectorAll('.suggestion-card');

  let ws = null;
  let reconnectDelay = 1000;
  const maxReconnectDelay = 30000;
  let reconnectTimer = null;
  let isConnected = false;

  // ── Theme Management ────────────────────────────────────────────────────────
  function initTheme() {
    const savedTheme = localStorage.getItem('gochat-theme') || 'dark';
    document.documentElement.setAttribute('data-theme', savedTheme);
  }

  function toggleTheme() {
    const current = document.documentElement.getAttribute('data-theme') || 'dark';
    const next = current === 'dark' ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', next);
    localStorage.setItem('gochat-theme', next);
  }

  if (themeToggleBtn) {
    themeToggleBtn.addEventListener('click', toggleTheme);
  }
  initTheme();

  // ── WebSocket URL ───────────────────────────────────────────────────────────
  function getWebSocketUrl() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    return `${protocol}//${window.location.host}/ws`;
  }

  // ── Connection Status ───────────────────────────────────────────────────────
  function updateStatus(state, label) {
    isConnected = (state === 'online');

    if (statusLabel) {
      statusLabel.textContent = label;
    }

    if (statusDot) {
      statusDot.className = 'status-dot';
      statusDot.classList.add(state);
    }

    updateSendButtonState();
  }

  // ── Scroll Handling (Fixes full screen message scroll) ──────────────────────
  function scrollToBottom(smooth = true) {
    if (!scrollArea) return;
    scrollArea.scrollTo({
      top: scrollArea.scrollHeight,
      behavior: smooth ? 'smooth' : 'auto'
    });
  }

  function handleScroll() {
    if (!scrollArea || !scrollBottomBtn) return;
    const distanceToBottom = scrollArea.scrollHeight - scrollArea.scrollTop - scrollArea.clientHeight;
    // Show button if user has scrolled up more than 150px
    if (distanceToBottom > 150) {
      scrollBottomBtn.classList.add('visible');
    } else {
      scrollBottomBtn.classList.remove('visible');
    }
  }

  if (scrollArea) {
    scrollArea.addEventListener('scroll', handleScroll, { passive: true });
  }

  if (scrollBottomBtn) {
    scrollBottomBtn.addEventListener('click', function () {
      scrollToBottom(true);
    });
  }

  // ── Time Formatting ─────────────────────────────────────────────────────────
  function formatTime() {
    const now = new Date();
    return now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  }

  // ── Safe HTML Escaping & Formatting ─────────────────────────────────────────
  function escapeHTML(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
  }

  function formatMessageText(rawText) {
    // Escape raw HTML first to prevent XSS
    const escaped = escapeHTML(rawText);

    // Format inline code blocks `code`
    return escaped.replace(/`([^`]+)`/g, '<code>$1</code>');
  }

  // ── Message Rendering ───────────────────────────────────────────────────────
  function renderMessage(role, text) {
    if (!messagesContainer) return;

    // Hide welcome hero on first user message
    if (role === 'user' && welcomeHero) {
      welcomeHero.style.display = 'none';
    }

    const messageArticle = document.createElement('article');
    messageArticle.className = `message message-${role}`;
    messageArticle.setAttribute('data-role', role);

    if (role === 'assistant') {
      const avatarDiv = document.createElement('div');
      avatarDiv.className = 'message-avatar';
      avatarDiv.setAttribute('aria-hidden', 'true');
      avatarDiv.innerHTML = `
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M12 2a2 2 0 0 1 2 2v2a2 2 0 0 1-2 2 2 2 0 0 1-2-2V4a2 2 0 0 1 2-2z"></path>
          <rect x="4" y="8" width="16" height="12" rx="3"></rect>
          <line x1="9" y1="13" x2="9" y2="15"></line>
          <line x1="15" y1="13" x2="15" y2="15"></line>
        </svg>
      `;
      messageArticle.appendChild(avatarDiv);
    }

    const bodyDiv = document.createElement('div');
    bodyDiv.className = 'message-body';

    const bubbleDiv = document.createElement('div');
    bubbleDiv.className = 'message-bubble';
    bubbleDiv.innerHTML = formatMessageText(text);
    bodyDiv.appendChild(bubbleDiv);

    const metaDiv = document.createElement('div');
    metaDiv.className = 'message-meta';

    const timeSpan = document.createElement('span');
    timeSpan.className = 'message-time';
    timeSpan.textContent = formatTime();
    metaDiv.appendChild(timeSpan);

    if (role === 'assistant') {
      const copyBtn = document.createElement('button');
      copyBtn.type = 'button';
      copyBtn.className = 'copy-btn';
      copyBtn.setAttribute('title', 'Copy response');
      copyBtn.setAttribute('aria-label', 'Copy response');
      copyBtn.innerHTML = `
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
          <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
        </svg>
        <span>Copy</span>
      `;

      copyBtn.addEventListener('click', function () {
        navigator.clipboard.writeText(text).then(function () {
          copyBtn.classList.add('copied');
          copyBtn.querySelector('span').textContent = 'Copied!';
          setTimeout(function () {
            copyBtn.classList.remove('copied');
            copyBtn.querySelector('span').textContent = 'Copy';
          }, 2000);
        }).catch(function (err) {
          console.error('Copy failed:', err);
        });
      });

      metaDiv.appendChild(copyBtn);
    }

    bodyDiv.appendChild(metaDiv);
    messageArticle.appendChild(bodyDiv);

    messagesContainer.appendChild(messageArticle);

    // Smooth scroll down to newest message
    requestAnimationFrame(function () {
      scrollToBottom(true);
    });
  }

  // ── Typing Indicator ────────────────────────────────────────────────────────
  function showTypingIndicator() {
    if (typingIndicator) {
      typingIndicator.style.display = 'flex';
      scrollToBottom(true);
    }
  }

  function hideTypingIndicator() {
    if (typingIndicator) {
      typingIndicator.style.display = 'none';
    }
  }

  // ── WebSocket Connection ────────────────────────────────────────────────────
  function connect() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }

    updateStatus('connecting', 'Connecting...');

    try {
      ws = new WebSocket(getWebSocketUrl());
    } catch (err) {
      console.error('WebSocket connection initialization error:', err);
      scheduleReconnect();
      return;
    }

    ws.onopen = function () {
      reconnectDelay = 1000;
      updateStatus('online', 'Connected');
    };

    ws.onmessage = function (event) {
      hideTypingIndicator();
      if (!event.data) return;

      try {
        const data = JSON.parse(event.data);
        if (data && data.text) {
          if (data.type === 'error') {
            renderMessage('assistant', `⚠️ ${data.text}`);
          } else {
            renderMessage('assistant', data.text);
          }
        }
      } catch (err) {
        renderMessage('assistant', event.data);
      }
    };

    ws.onclose = function () {
      hideTypingIndicator();
      updateStatus('offline', 'Disconnected');
      scheduleReconnect();
    };

    ws.onerror = function (err) {
      console.error('WebSocket error:', err);
      ws.close();
    };
  }

  function scheduleReconnect() {
    if (reconnectTimer) return;
    const delaySec = Math.round(reconnectDelay / 1000);
    updateStatus('connecting', `Reconnecting in ${delaySec}s...`);

    reconnectTimer = setTimeout(function () {
      reconnectTimer = null;
      reconnectDelay = Math.min(reconnectDelay * 2, maxReconnectDelay);
      connect();
    }, reconnectDelay);
  }

  // ── Auto-Expanding Textarea & Character Count ───────────────────────────────
  function resizeTextarea() {
    if (!messageInput) return;
    messageInput.style.height = 'auto';
    const newHeight = Math.min(messageInput.scrollHeight, 160);
    messageInput.style.height = `${newHeight}px`;
  }

  function updateSendButtonState() {
    if (!messageInput || !sendButton) return;
    const text = messageInput.value.trim();
    sendButton.disabled = (!isConnected || text.length === 0 || text.length > 500);
  }

  function handleInputChange() {
    if (!messageInput) return;
    const len = messageInput.value.length;

    if (charCounter) {
      charCounter.textContent = `${len} / 500`;
      charCounter.classList.toggle('warning', len >= 400 && len < 480);
      charCounter.classList.toggle('danger', len >= 480);
    }

    resizeTextarea();
    updateSendButtonState();
  }

  // ── Message Sending ─────────────────────────────────────────────────────────
  function sendMessage(customText) {
    const text = (customText !== undefined ? customText : (messageInput ? messageInput.value : '')).trim();
    if (!text) return;

    if (!ws || ws.readyState !== WebSocket.OPEN) {
      console.warn('Cannot send: WebSocket is not open');
      return;
    }

    // Render user message immediately
    renderMessage('user', text);

    // Show AI typing indicator
    showTypingIndicator();

    // Send JSON payload
    ws.send(JSON.stringify({
      type: 'text',
      text: text
    }));

    // Reset input
    if (messageInput) {
      messageInput.value = '';
      resizeTextarea();
      handleInputChange();
      messageInput.focus();
    }
  }

  // ── Event Listeners ─────────────────────────────────────────────────────────
  if (messageInput) {
    messageInput.addEventListener('input', handleInputChange);

    messageInput.addEventListener('keydown', function (e) {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        sendMessage();
      }
    });
  }

  if (chatForm) {
    chatForm.addEventListener('submit', function (e) {
      e.preventDefault();
      sendMessage();
    });
  }

  // Quick suggestion cards
  suggestionCards.forEach(function (card) {
    card.addEventListener('click', function () {
      const prompt = card.getAttribute('data-prompt');
      if (prompt) {
        sendMessage(prompt);
      }
    });
  });

  // Clear chat
  if (clearChatBtn) {
    clearChatBtn.addEventListener('click', function () {
      if (!messagesContainer) return;
      messagesContainer.innerHTML = '';
      if (welcomeHero) {
        welcomeHero.style.display = 'flex';
      }
      renderMessage('assistant', 'Conversation cleared. How can I help you today?');
    });
  }

  // Initialize
  handleInputChange();
  connect();
})();
