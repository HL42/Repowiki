<template>
  <div class="query-panel">
    <!-- 头部：标题 + 操作 -->
    <header class="panel-head">
      <div class="head-left">
        <span class="head-title">AI 问答</span>
        <span v-if="messages.length" class="conv-count">{{ Math.ceil(messages.length / 2) }} 轮</span>
      </div>
      <div class="head-right">
        <button v-if="messages.length" class="head-btn" @click="newChat" title="新对话">+</button>
        <button class="head-btn close-btn" @click="$emit('close')" title="关闭">✕</button>
      </div>
    </header>

    <!-- 范围选择 -->
    <div class="scope-strip">
      <select v-model="selectedProject" class="scope-sel" @change="onScopeChange">
        <option value="">全部项目</option>
        <option v-for="proj in projects" :key="proj.name" :value="proj.name">{{ proj.name }}</option>
      </select>
      <select v-if="subProjects.length" v-model="selectedSubProject" class="scope-sel" @change="onScopeChange">
        <option value="">全部子项目</option>
        <option v-for="sub in subProjects" :key="sub" :value="sub">{{ sub }}</option>
      </select>
    </div>

    <!-- 消息区 -->
    <div ref="chatArea" class="chat-body">
      <div v-if="!messages.length && !loading" class="empty-chat">
        <p class="empty-title">搜索知识库</p>
        <p class="empty-desc">在下方输入问题，AI 将从 Wiki 中检索回答</p>
      </div>

      <TransitionGroup name="msg" tag="div">
        <div v-for="(msg, idx) in messages" :key="msg.id" class="chat-msg" :class="msg.role">
          <div class="msg-bubble" :class="msg.role">
            <div class="bubble-label">{{ msg.role === 'assistant' ? 'AI' : '我' }}</div>
            <div v-if="msg.role === 'assistant'" class="bubble-body markdown-body" v-html="renderMarkdown(msg.content)"></div>
            <div v-else class="bubble-body">{{ msg.content }}</div>

            <footer v-if="msg.role === 'assistant' && msg.meta" class="bubble-foot">
              <span :class="['conf', msg.meta.confidence]">{{ confText(msg.meta.confidence) }}</span>
              <span class="foot-num">{{ msg.meta.matchedChunks || 0 }}段 · {{ msg.meta.searchScope || scopeLabel }}</span>
              <div v-if="msg.meta.citations?.length || msg.meta.sourcePages?.length" class="ref-row">
                <span v-for="(c, ci) in (msg.meta.citations || msg.meta.sourcePages?.map(s => ({ page: s })))" :key="ci" class="ref-tag">
                  {{ c.page }}{{ c.section ? ' · ' + c.section : '' }}
                </span>
              </div>
            </footer>
          </div>

          <button
            v-if="msg.role === 'user' && idx === messages.length - 2"
            class="retry-mini" @click="retryLast" :disabled="loading"
          >重发</button>
        </div>
      </TransitionGroup>

      <div v-if="loading" class="chat-msg assistant">
        <div class="msg-bubble assistant thinking-state">
          <span class="dot-pulse"><i></i><i></i><i></i></span>
          <span class="thinking-label">检索中...</span>
        </div>
      </div>
    </div>

    <div v-if="error" class="err-strip">{{ error }}<button @click="error = ''" class="err-close">&times;</button></div>

    <!-- 输入区 -->
    <div class="input-strip">
      <textarea
        ref="inputEl"
        v-model="question"
        class="chat-input"
        placeholder="输入问题，支持追问..."
        @keydown.ctrl.enter="ask"
        @keydown.meta.enter="ask"
        @keydown.enter.exact.prevent="quickSubmit"
        rows="1"
        :disabled="loading"
      ></textarea>
      <button class="send" @click="ask" :disabled="loading || !question.trim()" :aria-label="loading ? '等待' : '发送'">
        <span v-if="!loading" class="send-arrow">↑</span>
        <span v-else class="send-dot"></span>
      </button>
    </div>
  </div>
</template>

<script>
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { renderMarkdown } from './MarkdownRenderer.js'
import api from '../api.js'

const MAX_HISTORY_TURNS = 6

let msgIdCounter = 0
function nextMsgId() { return 'm' + (++msgIdCounter) }

export default {
  emits: ['close'],
  setup() {
    const question = ref('')
    const messages = ref([])
    const loading = ref(false)
    const error = ref('')
    const projects = ref([])
    const subProjects = ref([])
    const selectedProject = ref('')
    const selectedSubProject = ref('')
    const projectInfo = ref(null)
    const chatArea = ref(null)
    const inputEl = ref(null)

    onMounted(async () => {
      try { projects.value = await api.listProjects() } catch (e) { /* silent */ }
    })

    let projectWatchId = 0
    watch(selectedProject, async (p) => {
      const currentId = ++projectWatchId
      selectedSubProject.value = ''; projectInfo.value = null; subProjects.value = []
      if (!p) return
      try {
        const [subs, info] = await Promise.all([api.listSubProjects(p), api.getProject(p)])
        if (currentId !== projectWatchId) return // 忽略过期回调
        const names = new Set(subs)
        for (const s of info.sub_project_list || []) names.add(s.name)
        subProjects.value = Array.from(names)
        projectInfo.value = info
      } catch (e) { /* silent */ }
    })

    watch(() => messages.value.length, () => nextTick(scrollBottom))

    function buildHistory() {
      return messages.value.slice(-MAX_HISTORY_TURNS * 2).map(m => ({ role: m.role, content: m.content }))
    }

    async function ask() {
      if (!question.value.trim() || loading.value) return
      const q = question.value.trim()
      error.value = ''
      // 不清空输入框，直到请求成功后再清空，这样失败时用户仍可看到刚才输入的内容
      question.value = ''

      messages.value.push({ id: nextMsgId(), role: 'user', content: q })
      nextTick(scrollBottom)

      const history = buildHistory().slice(0, -1)
      loading.value = true
      try {
        const data = await api.query(q, selectedProject.value || undefined, selectedSubProject.value || undefined, history)
        if (data.error) { error.value = data.error } else {
          messages.value.push({
            id: nextMsgId(), role: 'assistant', content: data.answer,
            meta: {
              confidence: data.confidence, matchedChunks: data.matched_chunks,
              searchScope: data.search_scope, citations: data.citations, sourcePages: data.source_pages,
            },
          })
        }
      } catch (e) {
        error.value = '请求失败: ' + e.message
        // 恢复问题到输入框，方便重试
        question.value = q
      }
      loading.value = false
      nextTick(() => { scrollBottom(); inputEl.value?.focus() })
    }

    function retryLast() {
      if (loading.value) return
      const idx = messages.value.map(m => m.role).lastIndexOf('user')
      if (idx === -1) return
      const q = messages.value[idx].content
      if (messages.value[idx + 1]?.role === 'assistant') messages.value.splice(idx, 2)
      else messages.value.splice(idx, 1)
      question.value = q
      nextTick(ask)
    }

    function newChat() { messages.value = []; error.value = ''; question.value = ''; nextTick(() => inputEl.value?.focus()) }
    function onScopeChange() {}
    function quickSubmit() { if (question.value.trim()) ask() }
    function scrollBottom() { const el = chatArea.value; if (el) el.scrollTop = el.scrollHeight }
    function confText(c) { if (c === 'high') return '高'; if (c === 'medium') return '中'; return '低' }

    const scopeLabel = computed(() => {
      if (!selectedProject.value) return '全部'
      if (!selectedSubProject.value) return selectedProject.value
      return selectedProject.value + '/' + selectedSubProject.value
    })

    return {
      question, messages, loading, error, ask, retryLast, newChat, onScopeChange, quickSubmit,
      renderMarkdown, projects, subProjects, selectedProject, selectedSubProject,
      chatArea, inputEl, confText, scopeLabel,
    }
  },
}
</script>

<style scoped>
.query-panel {
  height: 100vh;
  display: flex;
  flex-direction: column;
  font-family: -apple-system, BlinkMacSystemFont, 'PingFang SC', 'Microsoft YaHei', sans-serif;
}

/* === Head === */
.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 18px 12px;
  border-bottom: 1px solid var(--color-border);
  flex-shrink: 0;
}

.head-left { display: flex; align-items: baseline; gap: 8px; }

.head-title { font-size: 16px; font-weight: 700; color: var(--color-text); }

.conv-count {
  font-size: 11px; color: var(--color-text-muted);
  background: #eee; padding: 1px 8px; border-radius: 100px;
}

.head-right { display: flex; gap: 4px; }

.head-btn {
  width: 30px; height: 30px;
  border: none; background: transparent;
  border-radius: 8px; font-size: 16px; cursor: pointer;
  display: flex; align-items: center; justify-content: center;
  color: var(--color-text-muted); transition: all 0.15s;
  line-height: 1;
}

.head-btn:hover { background: #eee; color: var(--color-text); }
.close-btn:hover { background: #fee; color: #c00; }

/* === Scope === */
.scope-strip {
  display: flex; gap: 6px; padding: 10px 18px; flex-shrink: 0;
}

.scope-sel {
  flex: 1; min-width: 0;
  border: 1px solid var(--color-border); border-radius: 6px;
  background: #fff; font-family: inherit; font-size: 12px;
  color: var(--color-text-secondary); padding: 5px 8px; cursor: pointer; outline: none;
}

/* === Chat Body === */
.chat-body {
  flex: 1; overflow-y: auto; padding: 8px 18px; min-height: 0;
}

.empty-chat {
  display: flex; flex-direction: column; align-items: center;
  padding: 60px 16px; text-align: center;
}

.empty-title { font-size: 15px; font-weight: 600; color: var(--color-text); margin-bottom: 4px; }
.empty-desc { font-size: 13px; color: var(--color-text-muted); line-height: 1.6; }

/* === Messages === */
.chat-msg { margin-bottom: 20px; }

.chat-msg.user { display: flex; flex-direction: column; align-items: flex-end; }
.chat-msg.assistant { display: flex; flex-direction: column; align-items: flex-start; }

.msg-bubble { max-width: 92%; }

.msg-bubble.user {
  background: #1d1d1f; color: #f0f0f3;
  padding: 10px 14px; border-radius: 14px 14px 3px 14px;
  font-size: 13.5px; line-height: 1.55; white-space: pre-wrap;
}

.msg-bubble.assistant {
  padding: 0; font-size: 13.5px; line-height: 1.62;
  color: var(--color-text); border-left: 2.5px solid #e0e0e6; padding-left: 12px;
  transition: border-color 0.15s;
}

.msg-bubble.assistant:hover { border-left-color: #1d1d1f; }

.bubble-label {
  font-size: 10px; font-weight: 700; text-transform: uppercase;
  letter-spacing: 0.05em; margin-bottom: 4px; color: var(--color-text-muted);
}

.msg-bubble.user .bubble-label { color: rgba(255,255,255,0.45); }

.bubble-body { word-break: break-word; }

/* Footer */
.bubble-foot {
  margin-top: 10px; padding-top: 8px;
  border-top: 1px solid #eee; display: flex; flex-wrap: wrap;
  gap: 6px; align-items: center;
}

.conf {
  font-size: 10px; font-weight: 600; padding: 1px 7px;
  border-radius: 100px; line-height: 1.6;
}

.conf.high   { background: #ecfdf5; color: #065f46; }
.conf.medium { background: #fffbeb; color: #92400e; }
.conf.low    { background: #fef2f2; color: #991b1b; }

.foot-num { font-size: 10.5px; color: var(--color-text-muted); }

.ref-row { width: 100%; display: flex; flex-wrap: wrap; gap: 4px; }

.ref-tag {
  font-size: 10.5px; padding: 1px 8px; background: #f3f3f6;
  color: var(--color-text-secondary); border-radius: 100px; cursor: default;
  transition: background 0.12s;
}

.ref-tag:hover { background: #e0e0e5; }

/* Thinking */
.thinking-state {
  display: flex; align-items: center; gap: 8px;
  padding: 8px 0; border-left: 2.5px solid #e0e0e6; padding-left: 12px;
}

.dot-pulse { display: flex; gap: 3px; }

.dot-pulse i {
  display: block; width: 4px; height: 4px; border-radius: 50%;
  background: #b0b0ba; animation: bounce-dot 1.4s infinite ease-in-out;
}

.dot-pulse i:nth-child(2) { animation-delay: 0.2s; }
.dot-pulse i:nth-child(3) { animation-delay: 0.4s; }

@keyframes bounce-dot {
  0%, 80%, 100% { opacity: 0.3; transform: translateY(0); }
  40% { opacity: 1; transform: translateY(-3px); }
}

.thinking-label { font-size: 12px; color: var(--color-text-muted); }

/* Retry */
.retry-mini {
  font-size: 10.5px; align-self: flex-end; margin-top: 2px;
  padding: 2px 10px; border: 1px solid var(--color-border);
  border-radius: 100px; background: #fff; color: var(--color-text-muted);
  cursor: pointer; font-family: inherit; transition: all 0.15s;
}

.retry-mini:hover { color: #1d1d1f; border-color: #1d1d1f; }

/* Error */
.err-strip {
  padding: 8px 14px; background: #fef2f2; border-top: 1px solid #fecaca;
  color: #991b1b; font-size: 12px; display: flex; align-items: center;
  justify-content: space-between; flex-shrink: 0;
}

.err-close { border: none; background: none; font-size: 15px; cursor: pointer; color: #991b1b; opacity: 0.6; }
.err-close:hover { opacity: 1; }

/* === Input === */
.input-strip {
  padding: 10px 18px; border-top: 1px solid var(--color-border);
  background: #fafafa; flex-shrink: 0;
  display: flex; align-items: flex-end; gap: 8px;
}

.chat-input {
  flex: 1; border: none; resize: none;
  background: #fff; border-radius: 10px; padding: 9px 12px;
  font-family: inherit; font-size: 13.5px; line-height: 1.5;
  color: var(--color-text); outline: none;
  border: 1px solid var(--color-border); min-height: 38px; max-height: 100px;
  transition: border-color 0.15s;
}

.chat-input:focus { border-color: #1d1d1f; }
.chat-input::placeholder { color: #b0b0ba; }

.send {
  width: 38px; height: 38px; border: none; border-radius: 10px;
  background: #1d1d1f; color: #fff; font-size: 16px; font-weight: 700;
  cursor: pointer; display: flex; align-items: center; justify-content: center;
  flex-shrink: 0; transition: all 0.15s;
}

.send:hover:not(:disabled) { background: #3a3a3f; transform: scale(1.04); }
.send:active:not(:disabled) { transform: scale(0.95); }
.send:disabled { background: #d1d1d6; cursor: not-allowed; }

.send-arrow { line-height: 1; }

.send-dot {
  width: 14px; height: 14px;
  border: 2px solid rgba(255,255,255,0.3);
  border-top-color: #fff; border-radius: 50%;
  animation: spi 0.7s linear infinite;
}

@keyframes spi { to { transform: rotate(360deg); } }

/* Transitions */
.msg-enter-active { transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1); }
.msg-leave-active { transition: all 0.15s ease-in; }
.msg-enter-from { opacity: 0; transform: translateY(12px); }
.msg-leave-to { opacity: 0; }

/* Markdown inside AI */
.msg-bubble.assistant .markdown-body :deep(h2) { font-size: 15px; margin: 12px 0 5px; font-weight: 700; }
.msg-bubble.assistant .markdown-body :deep(h3) { font-size: 14px; margin: 10px 0 4px; font-weight: 600; }
.msg-bubble.assistant .markdown-body :deep(p) { margin-bottom: 6px; }
.msg-bubble.assistant .markdown-body :deep(ul), .msg-bubble.assistant .markdown-body :deep(ol) { padding-left: 18px; margin-bottom: 6px; }
.msg-bubble.assistant .markdown-body :deep(code) {
  padding: 1px 4px; background: #f0f0f3; border-radius: 3px; font-size: 12px;
  font-family: 'SF Mono', 'Menlo', monospace;
}
.msg-bubble.assistant .markdown-body :deep(pre) {
  background: #f6f6f8; padding: 10px 12px; border-radius: 6px;
  overflow-x: auto; margin-bottom: 8px; font-size: 12px; line-height: 1.5;
}
.msg-bubble.assistant .markdown-body :deep(pre code) { background: none; padding: 0; font-size: 12px; }
.msg-bubble.assistant .markdown-body :deep(blockquote) {
  border-left: 2px solid #d0d0d8; padding: 4px 10px; margin-bottom: 6px;
  color: var(--color-text-secondary); font-style: italic;
}
.msg-bubble.assistant .markdown-body :deep(a) { color: var(--color-accent); text-decoration: none; }
.msg-bubble.assistant .markdown-body :deep(table) { border-collapse: collapse; width: 100%; margin-bottom: 8px; font-size: 12px; }
.msg-bubble.assistant .markdown-body :deep(th), .msg-bubble.assistant .markdown-body :deep(td) {
  border: 1px solid #e0e0e6; padding: 5px 8px; text-align: left;
}
.msg-bubble.assistant .markdown-body :deep(th) { background: #f6f6f8; font-weight: 600; }
</style>
