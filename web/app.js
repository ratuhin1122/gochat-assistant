(function () {
  'use strict';

  // Elements
  const messagesContainer = document.getElementById('chat-messages');
  const chatForm = document.getElementById('chat-form');
  const messageInput = document.getElementById('message-input');
  const sendButton = document.getElementById('send-button');
  const statusDot = document.querySelector('.status-dot');
  const statusText = document.querySelector('.status-text');

  let ws = null;
  let reconnectDelay = 1000;
  const maxReconnectDelay = 30000;
  let reconnectTimer = null;

  // Determine WebSocket URL based on current page location
  function getWebSocketUrl() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    return `${protocol}//${window.location.host}/ws`;
  }

  // Update visual connection status
  function updateStatus(state, text) {
    if (statusText) {
      statusText.textContent = text;
    }
    if (statusDot) {
      statusDot.classList.remove('connecting', 'offline');
      if (state === 'connecting') {
        statusDot.classList.add('connecting');
      } else if (state === 'offline') {
        statusDot.classList.add('offline');
      }
    }
    if (sendButton) {
      sendButton.disabled = (state !== 'online');
    }
  }

  // Format current local time HH:MM
  function formatTime() {
    const now = new Date();
    return now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  }

  // Render a message safely using textContent (never innerHTML)
  function renderMessage(role, text) {
    if (!messagesContainer) return;

    const messageArticle = document.createElement('article');
    messageArticle.className = `message message-${role}`;

    // For assistant messages, add avatar SVG safely using DOM elements
    if (role === 'assistant') {
      const avatarDiv = document.createElement('div');
      avatarDiv.className = 'message-avatar';
      avatarDiv.setAttribute('aria-hidden', 'true');

      // Create SVG node with XML namespace
      const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
      svg.setAttribute('width', '18');
      svg.setAttribute('height', '18');
      svg.setAttribute('viewBox', '0 0 24 24');
      svg.setAttribute('fill', 'none');
      svg.setAttribute('stroke', 'currentColor');
      svg.setAttribute('stroke-width', '2');
      svg.setAttribute('stroke-linecap', 'round');
      svg.setAttribute('stroke-linejoin', 'round');

      const path1 = document.createElementNS('http://www.w3.org/2000/svg', 'path');
      path1.setAttribute('d', 'M12 8V4H8');
      const rect = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
      rect.setAttribute('width', '16');
      rect.setAttribute('height', '12');
      rect.setAttribute('x', '4');
      rect.setAttribute('y', '8');
      rect.setAttribute('rx', '2');
      const path2 = document.createElementNS('http://www.w3.org/2000/svg', 'path');
      path2.setAttribute('d', 'M2 14h2');
      const path3 = document.createElementNS('http://www.w3.org/2000/svg', 'path');
      path3.setAttribute('d', 'M20 14h2');

      svg.appendChild(path1);
      svg.appendChild(rect);
      svg.appendChild(path2);
      svg.appendChild(path3);
      avatarDiv.appendChild(svg);
      messageArticle.appendChild(avatarDiv);
    }

    const bodyDiv = document.createElement('div');
    bodyDiv.className = 'message-body';

    const bubbleDiv = document.createElement('div');
    bubbleDiv.className = 'message-bubble';

    const p = document.createElement('p');
    p.textContent = text; // SAFE: textContent only, never innerHTML

    bubbleDiv.appendChild(p);
    bodyDiv.appendChild(bubbleDiv);

    const timeSpan = document.createElement('span');
    timeSpan.className = 'message-time';
    timeSpan.textContent = formatTime();
    bodyDiv.appendChild(timeSpan);

    messageArticle.appendChild(bodyDiv);
    messagesContainer.appendChild(messageArticle);

    // Auto-scroll to latest message
    messagesContainer.scrollTop = messagesContainer.scrollHeight;
  }

  // Connect to WebSocket with exponential backoff reconnect
  function connect() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }

    updateStatus('connecting', 'Connecting...');

    try {
      ws = new WebSocket(getWebSocketUrl());
    } catch (err) {
      console.error('WebSocket connection error:', err);
      scheduleReconnect();
      return;
    }

    ws.onopen = function () {
      console.log('WebSocket connected');
      reconnectDelay = 1000; // Reset backoff on success
      updateStatus('online', 'Online');
    };

    ws.onmessage = function (event) {
      const msg = typeof event.data === 'string' ? event.data : '';
      if (msg) {
        renderMessage('assistant', msg);
      }
    };

    ws.onclose = function () {
      console.log('WebSocket closed');
      updateStatus('offline', 'Offline');
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

  // Send message handler
  function sendMessage() {
    if (!messageInput) return;
    const text = messageInput.value.trim();
    if (!text) return;

    if (!ws || ws.readyState !== WebSocket.OPEN) {
      console.warn('Cannot send: WebSocket is not open');
      return;
    }

    // Render user message immediately
    renderMessage('user', text);

    // Send via WebSocket
    ws.send(text);

    // Clear and refocus input
    messageInput.value = '';
    messageInput.focus();
  }

  // Event Listeners
  if (chatForm) {
    chatForm.addEventListener('submit', function (e) {
      e.preventDefault();
      sendMessage();
    });
  }

  if (messageInput) {
    messageInput.addEventListener('keydown', function (e) {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        sendMessage();
      }
    });
  }

  // Initialize connection
  connect();
})();
