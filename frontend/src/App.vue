<template>
  <div class="app">
    <!-- 左侧导航 -->
    <aside class="sidebar">
      <div class="sidebar-brand">
        <span class="brand-title">RepoWiki</span>
        <span class="brand-sub">公司知识库</span>
      </div>

      <nav class="sidebar-nav">
        <button
          v-for="tab in tabs"
          :key="tab.id"
          :class="{ active: activeTab === tab.id }"
          @click="activeTab = tab.id"
        >
          <span class="nav-icon">{{ tab.icon }}</span>
          <span class="nav-label">{{ tab.label }}</span>
        </button>
      </nav>

      <!-- AI 问答切换按钮 -->
      <div class="sidebar-ai-toggle">
        <button :class="{ active: showAIPanel }" @click="showAIPanel = !showAIPanel">
          <span class="nav-icon">💬</span>
          <span class="nav-label">AI 问答</span>
          <span v-if="!showAIPanel" class="toggle-hint">打开</span>
          <span v-else class="toggle-hint active">已打开</span>
        </button>
      </div>

      <div class="sidebar-footer" v-if="stats">
        <div class="footer-stat">
          <span class="footer-number">{{ stats.total_pages }}</span>
          <span class="footer-unit">个页面已索引</span>
        </div>
        <div class="footer-meta">
          {{ stats.source_pages }} 源文档 · {{ stats.entity_pages }} 实体 · {{ stats.concept_pages }} 概念
          <span v-if="stats.synthesis_pages"> · {{ stats.synthesis_pages }} 综合</span>
        </div>
        <div class="footer-lint" v-if="stats.lint_time">
          <div class="lint-status" :class="lintStatusClass">
            <span class="lint-dot"></span>
            <span>{{ lintStatusText }}</span>
            <span class="lint-time">{{ stats.lint_time }}</span>
          </div>
          <div class="lint-detail" v-if="stats.lint_errors || stats.lint_warnings">
            <span v-if="stats.lint_errors" class="lint-badge error">{{ stats.lint_errors }} 严重</span>
            <span v-if="stats.lint_warnings" class="lint-badge warn">{{ stats.lint_warnings }} 警告</span>
            <span v-if="stats.lint_info" class="lint-badge info">{{ stats.lint_info }} 提示</span>
          </div>
          <div class="lint-detail" v-if="stats.stale_docs > 0" style="margin-top:4px">
            <span class="lint-badge stale">{{ stats.stale_docs }} 篇文档有新版本</span>
          </div>
        </div>
      </div>
    </aside>

    <!-- 中间主内容区：根据右侧 AI 面板状态自适应宽度 -->
    <main class="main-content" :class="{ 'ai-panel-open': showAIPanel }">
      <WikiBrowser v-if="activeTab === 'browse'" :key="'browse-' + refreshKey" @refresh="refreshKey++" />
      <ProjectBanner v-else-if="activeTab === 'explore'" key="explore" @select="onProjectSelect" />
      <ProjectDetail
        v-else-if="activeTab === 'project'"
        :key="'project-' + currentProject"
        :projectName="currentProject"
        @back="activeTab = 'explore'"
      />
      <SynthesizePanel v-else-if="activeTab === 'synthesize'" key="synthesize" @done="onSynthesizeDone" />
      <StatsPanel v-else-if="activeTab === 'stats'" key="stats" />
      <IngestPanel v-else-if="activeTab === 'ingest'" key="ingest" @done="onIngestDone" />
      <LintPanel v-else-if="activeTab === 'lint'" key="lint" :currentProject="currentProject" @done="onLintDone" />
    </main>

    <!-- 右侧 AI 问答面板（可滑入/滑出） -->
    <aside class="ai-panel" :class="{ open: showAIPanel }" v-show="showAIPanel">
      <QueryPanel @close="showAIPanel = false" />
    </aside>

    <!-- 面板关闭时的浮动入口按钮 -->
    <button v-if="!showAIPanel" class="floating-ai-btn" @click="showAIPanel = true" title="打开 AI 问答">
      <span class="float-icon">💬</span>
      <span class="float-label">AI 问答</span>
    </button>
  </div>
</template>

<script>
import { ref, computed, onMounted, watch } from 'vue'
import WikiBrowser from './components/WikiBrowser.vue'
import QueryPanel from './components/QueryPanel.vue'
import ProjectBanner from './components/ProjectBanner.vue'
import ProjectDetail from './components/ProjectDetail.vue'
import SynthesizePanel from './components/SynthesizePanel.vue'
import StatsPanel from './components/StatsPanel.vue'
import IngestPanel from './components/IngestPanel.vue'
import LintPanel from './components/LintPanel.vue'
import api from './api.js'

export default {
  components: { WikiBrowser, QueryPanel, ProjectBanner, ProjectDetail, SynthesizePanel, StatsPanel, IngestPanel, LintPanel },
  setup() {
    const activeTab = ref('explore')
    const stats = ref(null)
    const refreshKey = ref(0)
    const currentProject = ref('')
    const showAIPanel = ref(false)

    // 切换 tab 时自动关闭 AI 面板
    watch(activeTab, () => { showAIPanel.value = false })

    const tabs = [
      { id: 'explore', label: '知识库总览', icon: '🏠' },
      { id: 'browse', label: '浏览 Wiki', icon: '📚' },
      { id: 'synthesize', label: '综合提炼', icon: '🔬' },
      { id: 'stats', label: '知识库统计', icon: '📊' },
      { id: 'ingest', label: '导入文档', icon: '📥' },
      { id: 'lint', label: '质量检查', icon: '🔍' },
    ]

    onMounted(async () => {
      stats.value = await api.stats()
    })

    async function onSynthesizeDone() {
      stats.value = await api.stats()
      refreshKey.value++
    }

    async function onIngestDone() {
      stats.value = await api.stats()
      refreshKey.value++
    }

    async function onLintDone() {
      stats.value = await api.stats()
      refreshKey.value++
    }

    function onProjectSelect(name) {
      currentProject.value = name
      activeTab.value = 'project'
    }

    const lintStatusClass = computed(() => {
      if (!stats.value?.lint_time) return ''
      if (stats.value.lint_errors > 0) return 'bad'
      if (stats.value.lint_warnings > 100) return 'warn'
      return 'good'
    })

    const lintStatusText = computed(() => {
      if (!stats.value?.lint_time) return ''
      if (stats.value.lint_errors > 0) return `${stats.value.lint_errors} 个严重问题待处理`
      if (stats.value.lint_warnings > 100) return `${stats.value.lint_warnings} 个警告待关注`
      return '质量良好'
    })

    return { activeTab, tabs, stats, refreshKey, currentProject, showAIPanel, lintStatusClass, lintStatusText, onSynthesizeDone, onIngestDone, onLintDone, onProjectSelect }
  },
}
</script>

<style>
/* ===== Reset ===== */
*,
*::before,
*::after { margin: 0; padding: 0; box-sizing: border-box; }

:root {
  --color-bg: #ffffff;
  --color-sidebar: #f8f9fb;
  --color-text: #1d1d1f;
  --color-text-secondary: #6b6b7b;
  --color-text-muted: #9a9aad;
  --color-accent: #4f6cf7;
  --color-accent-light: #eef1ff;
  --color-border: #e8e8ef;
  --color-hover: #f3f4f9;
  --sidebar-width: 200px;
  --ai-panel-width: 420px;
  --radius: 8px;
}

html {
  font-size: 15px;
  -webkit-font-smoothing: antialiased;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif;
  background: var(--color-bg);
  color: var(--color-text);
  line-height: 1.6;
}

/* ===== Layout ===== */
.app { display: flex; min-height: 100vh; }

/* ===== Sidebar ===== */
.sidebar {
  width: var(--sidebar-width);
  background: var(--color-sidebar);
  display: flex;
  flex-direction: column;
  position: fixed;
  top: 0; left: 0; bottom: 0;
  z-index: 100;
  border-right: 1px solid var(--color-border);
}

.sidebar-brand {
  padding: 24px 20px 20px;
  border-bottom: 1px solid var(--color-border);
}

.brand-title {
  display: block;
  font-size: 18px;
  font-weight: 700;
  color: var(--color-text);
  margin-bottom: 2px;
}

.brand-sub {
  font-size: 12px;
  color: var(--color-text-muted);
}

/* Nav */
.sidebar-nav {
  flex: 0 0 auto;
  padding: 12px 10px;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.sidebar-nav button {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 9px 12px;
  border: none;
  background: transparent;
  color: var(--color-text-secondary);
  font-family: inherit;
  font-size: 13.5px;
  cursor: pointer;
  text-align: left;
  border-radius: 6px;
  transition: background 0.12s, color 0.12s;
}

.sidebar-nav button:hover {
  background: var(--color-hover);
  color: var(--color-text);
}

.sidebar-nav button.active {
  background: var(--color-accent-light);
  color: var(--color-accent);
  font-weight: 600;
}

.nav-icon { font-size: 15px; width: 20px; text-align: center; }

/* Sidebar AI toggle (底部区域) */
.sidebar-ai-toggle {
  flex: 1;
  display: flex;
  align-items: flex-end;
  padding: 12px 10px;
}

.sidebar-ai-toggle button {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 10px 12px;
  border: 1.5px solid var(--color-border);
  background: #fff;
  color: var(--color-text-secondary);
  font-family: inherit;
  font-size: 13.5px;
  cursor: pointer;
  text-align: left;
  border-radius: 8px;
  transition: all 0.15s;
  position: relative;
}

.sidebar-ai-toggle button:hover {
  border-color: var(--color-accent);
  color: var(--color-accent);
  background: var(--color-accent-light);
}

.sidebar-ai-toggle button.active {
  background: var(--color-accent-light);
  color: var(--color-accent);
  border-color: var(--color-accent);
  font-weight: 600;
}

.toggle-hint {
  margin-left: auto;
  font-size: 10px;
  opacity: 0.6;
  font-weight: 400;
}

.toggle-hint.active { opacity: 1; }

/* Sidebar footer */
.sidebar-footer {
  padding: 16px 20px;
  border-top: 1px solid var(--color-border);
}

.footer-stat {
  display: flex;
  align-items: baseline;
  gap: 6px;
  margin-bottom: 4px;
}

.footer-number {
  font-size: 28px;
  font-weight: 700;
  color: var(--color-text);
  line-height: 1;
}

.footer-unit {
  font-size: 12px;
  color: var(--color-text-muted);
}

.footer-meta {
  font-size: 11.5px;
  color: var(--color-text-muted);
  line-height: 1.5;
}

.footer-lint {
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--color-border);
}

.lint-status {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  margin-bottom: 4px;
}

.lint-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}

.lint-status.good .lint-dot { background: #16a34a; }
.lint-status.good { color: #16a34a; }
.lint-status.warn .lint-dot { background: #d97706; }
.lint-status.warn { color: #d97706; }
.lint-status.bad .lint-dot { background: #dc2626; }
.lint-status.bad { color: #dc2626; }

.lint-time {
  margin-left: auto;
  font-size: 10px;
  color: var(--color-text-muted);
}

.lint-detail {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}

.lint-badge {
  padding: 1px 5px;
  border-radius: 3px;
  font-size: 10.5px;
  font-weight: 600;
}

.lint-badge.error { background: #fef2f2; color: #dc2626; }
.lint-badge.warn { background: #fffbeb; color: #d97706; }
.lint-badge.info { background: var(--color-accent-light); color: var(--color-accent); }
.lint-badge.stale { background: #fefce8; color: #ca8a04; }

/* Main */
.main-content {
  margin-left: var(--sidebar-width);
  flex: 1;
  min-height: 100vh;
  padding: 36px 40px;
  transition: margin-right 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}

.main-content.ai-panel-open {
  margin-right: var(--ai-panel-width);
}

/* ===== Right AI Panel ===== */
.ai-panel {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  width: var(--ai-panel-width);
  background: #fafafa;
  border-left: 1px solid var(--color-border);
  z-index: 50;
  transform: translateX(100%);
  transition: transform 0.3s cubic-bezier(0.16, 1, 0.3, 1);
  overflow: hidden;
}

.ai-panel.open {
  transform: translateX(0);
}

/* Floating AI button when panel is closed */
.floating-ai-btn {
  position: fixed;
  bottom: 32px;
  right: 32px;
  z-index: 60;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 20px;
  background: #1d1d1f;
  color: #fff;
  border: none;
  border-radius: 100px;
  font-family: inherit;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  box-shadow: 0 4px 20px rgba(0,0,0,0.12), 0 1px 3px rgba(0,0,0,0.08);
  transition: all 0.2s;
}

.floating-ai-btn:hover {
  background: #3a3a3f;
  transform: translateY(-2px);
  box-shadow: 0 8px 30px rgba(0,0,0,0.15), 0 2px 6px rgba(0,0,0,0.1);
}

.float-icon { font-size: 18px; line-height: 1; }
.float-label { line-height: 1; }

/* ===== Shared classes ===== */
.page-title {
  font-size: 22px;
  font-weight: 700;
  color: var(--color-text);
  margin-bottom: 6px;
}

.page-desc {
  font-size: 14px;
  color: var(--color-text-secondary);
  margin-bottom: 28px;
}

.btn-primary {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 9px 22px;
  background: var(--color-accent);
  color: #fff;
  border: none;
  border-radius: 6px;
  font-family: inherit;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.12s;
  white-space: nowrap;
}

.btn-primary:hover { background: #3d5ae0; }
.btn-primary:disabled { background: #a3b1f7; cursor: not-allowed; }

.form-input {
  padding: 10px 14px;
  border: 1px solid var(--color-border);
  border-radius: 6px;
  background: #fff;
  font-family: inherit;
  font-size: 14px;
  color: var(--color-text);
  transition: border-color 0.12s;
}

.form-input:focus {
  outline: none;
  border-color: var(--color-accent);
  box-shadow: 0 0 0 3px rgba(79,108,247,0.08);
}

.form-input::placeholder { color: var(--color-text-muted); }

.loading-state {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--color-accent);
  font-size: 14px;
  padding: 16px 0;
}

.loading-state::before {
  content: '';
  width: 14px; height: 14px;
  border: 2px solid var(--color-accent-light);
  border-top-color: var(--color-accent);
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
}

@keyframes spin { to { transform: rotate(360deg); } }

.card {
  background: #fff;
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
}

/* Markdown body */
.markdown-body { font-size: 14.5px; line-height: 1.75; color: var(--color-text); }
.markdown-body :deep(h2) { font-size: 19px; margin: 24px 0 10px; font-weight: 700; }
.markdown-body :deep(h3) { font-size: 16px; margin: 20px 0 8px; font-weight: 600; }
.markdown-body :deep(h4) { font-size: 15px; margin: 16px 0 6px; font-weight: 600; }
.markdown-body :deep(h5) { font-size: 14px; margin: 14px 0 6px; font-weight: 600; }
.markdown-body :deep(p) { margin-bottom: 10px; }
.markdown-body :deep(ul), .markdown-body :deep(ol) { padding-left: 20px; margin-bottom: 10px; }
.markdown-body :deep(li) { margin-bottom: 4px; }
.markdown-body :deep(strong) { font-weight: 600; }
.markdown-body :deep(code) {
  padding: 2px 6px;
  background: #f3f4f9;
  border-radius: 3px;
  font-size: 13px;
  font-family: 'SFMono-Regular', 'Menlo', 'Monaco', monospace;
}
.markdown-body :deep(pre) {
  background: #f3f4f9;
  padding: 14px 18px;
  border-radius: 6px;
  overflow-x: auto;
  margin-bottom: 12px;
  font-size: 13px;
  line-height: 1.6;
}
.markdown-body :deep(pre code) {
  background: none;
  padding: 0;
  font-size: 13px;
}
.markdown-body :deep(pre code.hljs) {
  background: none;
  padding: 0;
}
.markdown-body :deep(table) {
  border-collapse: collapse;
  width: 100%;
  margin-bottom: 12px;
}
.markdown-body :deep(th),
.markdown-body :deep(td) {
  border: 1px solid var(--color-border);
  padding: 8px 12px;
  font-size: 13px;
  text-align: left;
}
.markdown-body :deep(th) {
  background: #f8f9fb;
  font-weight: 600;
}
.markdown-body :deep(blockquote) {
  border-left: 3px solid var(--color-accent);
  padding: 6px 14px;
  margin-bottom: 10px;
  color: var(--color-text-secondary);
  background: var(--color-accent-light);
  border-radius: 0 4px 4px 0;
}
.markdown-body :deep(hr) {
  border: none;
  border-top: 1px solid var(--color-border);
  margin: 16px 0;
}
.markdown-body :deep(a) { color: var(--color-accent); text-decoration: none; }
.markdown-body :deep(a:hover) { text-decoration: underline; }
</style>
