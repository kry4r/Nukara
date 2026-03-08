# Nukara — 简易前端套壳指南

本文档介绍如何用最简单的方式为 Nukara 后端搭建一个 Web 前端，适合快速验证和演示。

## 方案一：纯 HTML + fetch（零依赖）

创建一个 `index.html`，直接用浏览器打开即可。

### 最小可用示例

```html
<!DOCTYPE html>
<html lang="zh">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Nukara Chat</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body { font-family: -apple-system, sans-serif; background: #f5f5f5; }
    #app { max-width: 480px; margin: 0 auto; height: 100vh; display: flex; flex-direction: column; }
    #login, #chat { padding: 20px; }
    #chat { display: none; flex: 1; flex-direction: column; }
    #messages { flex: 1; overflow-y: auto; padding: 10px 0; }
    .msg { margin: 8px 0; padding: 10px 14px; border-radius: 16px; max-width: 80%; word-break: break-word; }
    .msg.user { background: #007aff; color: #fff; margin-left: auto; }
    .msg.bot { background: #fff; color: #333; }
    #input-bar { display: flex; gap: 8px; padding: 10px 0; }
    #input-bar input { flex: 1; padding: 10px; border: 1px solid #ddd; border-radius: 20px; outline: none; }
    #input-bar button { padding: 10px 20px; background: #007aff; color: #fff; border: none; border-radius: 20px; cursor: pointer; }
    .btn { padding: 10px 20px; background: #007aff; color: #fff; border: none; border-radius: 8px; cursor: pointer; margin: 5px 0; }
    input[type=text] { padding: 10px; border: 1px solid #ddd; border-radius: 8px; width: 100%; margin: 5px 0; }
  </style>
</head>
<body>
<div id="app">
  <!-- 登录区 -->
  <div id="login">
    <h2>Nukara</h2>
    <input type="text" id="email" placeholder="邮箱地址">
    <button class="btn" onclick="sendEmailCode()">发送验证码</button>
    <input type="text" id="code" placeholder="验证码">
    <button class="btn" onclick="login()">登录</button>
    <button class="btn" onclick="register()" style="background:#34c759">注册</button>
    <p id="login-msg" style="color:#888;margin-top:10px"></p>
  </div>

  <!-- 聊天区 -->
  <div id="chat">
    <h3 id="bot-name">Chat</h3>
    <div id="messages"></div>
    <div id="input-bar">
      <input type="text" id="msg-input" placeholder="说点什么..." onkeydown="if(event.key==='Enter')sendMsg()">
      <button onclick="sendMsg()">发送</button>
    </div>
  </div>
</div>

<script>
const API = 'http://localhost:8080';
let token = '';
let botId = '';
let convId = '';

async function api(path, opts = {}) {
  const headers = { 'Content-Type': 'application/json', ...opts.headers };
  if (token) headers['Authorization'] = 'Bearer ' + token;
  const res = await fetch(API + path, { ...opts, headers });
  return res.json();
}

async function sendEmailCode() {
  const email = document.getElementById('email').value;
  const data = await api('/api/v1/auth/email/send', {
    method: 'POST',
    body: JSON.stringify({ email, purpose: 'login' })
  });
  document.getElementById('login-msg').textContent = '验证码已发送，请检查收件箱';
}

async function login() {
  const email = document.getElementById('email').value;
  const email_code = document.getElementById('code').value;
  const data = await api('/api/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, email_code })
  });
  if (data.access_token) {
    token = data.access_token;
    await enterChat();
  } else {
    document.getElementById('login-msg').textContent = data.error || '登录失败';
  }
}

async function register() {
  const email = document.getElementById('email').value;
  const email_code = document.getElementById('code').value;
  const nicknameSeed = (email.split('@')[0] || 'user').slice(-6);
  const data = await api('/api/v1/auth/register', {
    method: 'POST',
    body: JSON.stringify({ email, email_code, nickname: '用户' + nicknameSeed })
  });
  if (data.access_token) {
    token = data.access_token;
    await enterChat();
  } else {
    document.getElementById('login-msg').textContent = data.error || '注册失败';
  }
}

async function enterChat() {
  document.getElementById('login').style.display = 'none';
  document.getElementById('chat').style.display = 'flex';

  // 获取或创建 bot
  let bots = await api('/api/v1/bots');
  if (!bots || bots.length === 0) {
    const newBot = await api('/api/v1/bots', {
      method: 'POST',
      body: JSON.stringify({ name: '小夜', summary: '温柔的陪伴AI', traits: ['温柔'] })
    });
    botId = newBot.id;
  } else {
    botId = bots[0].id;
    document.getElementById('bot-name').textContent = bots[0].name;
  }

  // 获取会话
  const convs = await api('/api/v1/conversations');
  if (convs && convs.length > 0) {
    convId = convs[0].id;
    document.getElementById('bot-name').textContent = convs[0].bot_name;
    // 加载历史消息
    const msgs = await api('/api/v1/conversations/' + convId + '/messages?limit=50');
    if (msgs) msgs.forEach(m => appendMsg(m.content.text, m.sender_type));
  }
}

async function sendMsg() {
  const input = document.getElementById('msg-input');
  const text = input.value.trim();
  if (!text) return;
  input.value = '';
  appendMsg(text, 'user');

  const data = await api('/api/v1/conversations/' + convId + '/send', {
    method: 'POST',
    body: JSON.stringify({ content: { type: 'text', text } })
  });
  if (data.bot_message) {
    appendMsg(data.bot_message.content.text, 'bot');
  }
}

function appendMsg(text, type) {
  const div = document.createElement('div');
  div.className = 'msg ' + (type === 'user' ? 'user' : 'bot');
  div.textContent = text;
  const container = document.getElementById('messages');
  container.appendChild(div);
  container.scrollTop = container.scrollHeight;
}
</script>
</body>
</html>
```

### 使用方式

1. 将上面的 HTML 保存为 `index.html`
2. 启动后端：`cd Nukara_Backend && ./build/gateway`
3. 后端需要允许跨域（或直接用同端口代理）
4. 浏览器打开 `index.html`

### 跨域问题

后端默认没有 CORS 头。最简单的解决方式是用一个反向代理，比如：

```bash
# 安装 caddy: brew install caddy
# 创建 Caddyfile:
cat > Caddyfile << 'EOF'
:3000 {
    handle /api/* {
        reverse_proxy localhost:8080
    }
    handle /ws/* {
        reverse_proxy localhost:8080
    }
    handle {
        root * .
        file_server
    }
}
EOF

caddy run
```

然后访问 `http://localhost:3000`，把 HTML 中的 `API` 变量改为空字符串 `''`。

---

## 方案二：WebSocket 实时聊天

在方案一基础上加入 WebSocket 支持，获得打字机效果：

```javascript
// 在 enterChat() 末尾添加：
const ws = new WebSocket('ws://localhost:8080/ws/chat?token=' + token);

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  switch (data.type) {
    case 'stream_start':
      // 创建一个空的 bot 消息气泡
      appendMsg('', 'bot');
      break;
    case 'stream_chunk':
      // 追加文字到最后一个 bot 气泡
      const msgs = document.querySelectorAll('.msg.bot');
      const last = msgs[msgs.length - 1];
      if (last) last.textContent += data.text;
      break;
    case 'proactive_message':
      // 主动消息
      appendMsg(data.content.text, 'bot');
      break;
  }
};
```

---

## 方案三：用 React/Vue 快速搭建

如果需要更完整的前端，推荐：

```bash
# React
npx create-react-app nukara-web
# 或 Vue
npm create vue@latest nukara-web
```

核心只需封装一个 API 客户端：

```javascript
// api.js
const BASE = 'http://localhost:8080';

export async function request(path, options = {}) {
  const token = localStorage.getItem('nukara_token');
  const res = await fetch(BASE + path, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...options.headers,
    },
  });
  return res.json();
}
```

然后按需调用各 API 端点即可。完整 API 列表参见 [CURL_TESTING.md](./CURL_TESTING.md)。
