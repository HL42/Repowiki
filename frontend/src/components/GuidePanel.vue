<template>
  <div class="guide-panel">
    <h1 class="page-title">快速开始</h1>
    <p class="page-desc">三步把 RepoWiki 用起来：配置密钥 → 导入文档 → 提问。</p>

    <div class="status-banner" :class="isEmpty ? 'empty' : 'ready'">
      <span class="status-icon">{{ isEmpty ? '📭' : '✅' }}</span>
      <div class="status-text">
        <div class="status-title">{{ isEmpty ? '知识库还是空的' : '知识库已有内容' }}</div>
        <div class="status-sub">
          <template v-if="isEmpty">按下面三步导入第一批文档后即可提问。</template>
          <template v-else>已索引 {{ stats.total_pages }} 个页面（{{ stats.source_pages }} 源文档 · {{ stats.entity_pages }} 实体 · {{ stats.concept_pages }} 概念）。</template>
        </div>
      </div>
    </div>

    <!-- 第 1 步 -->
    <section class="step-card card">
      <div class="step-head">
        <span class="step-no">1</span>
        <h3 class="step-title">填写 DeepSeek API Key</h3>
        <span class="step-badge" :class="{ done: cfg.api_key_set }">{{ cfg.api_key_set ? '已配置' : '必做' }}</span>
      </div>

      <!-- 未配置：直接在这里填，不用碰终端 -->
      <div v-if="!cfg.api_key_set" class="key-form">
        <p class="step-body">
          到 <a href="https://platform.deepseek.com/api_keys" target="_blank" rel="noreferrer">platform.deepseek.com</a>
          创建一个 API Key（<strong>sk-</strong> 开头），粘贴到下面即可。密钥只保存在本机，不会上传。
        </p>
        <div class="key-input-row">
          <input
            v-model="apiKeyInput"
            type="password"
            class="form-input key-input"
            placeholder="sk-…"
            autocomplete="off"
            @keydown.enter="saveKey"
          />
          <button class="btn-primary" @click="saveKey" :disabled="saving || !apiKeyInput.trim()">
            {{ saving ? '校验并保存中…' : '保存' }}
          </button>
        </div>
        <div v-if="saveError" class="save-msg err">{{ saveError }}</div>
        <div v-if="saveOk" class="save-msg ok">已保存并验证通过，可以开始导入了。</div>
      </div>

      <!-- 已配置：显示脱敏后的 key -->
      <div v-else class="key-done">
        <p class="step-body">
          当前已配置 <code>{{ cfg.model }}</code>，密钥 <code>{{ cfg.api_key_masked }}</code>。
          如需更换，在下面输入新密钥覆盖即可。
        </p>
        <div class="key-input-row">
          <input
            v-model="apiKeyInput"
            type="password"
            class="form-input key-input"
            placeholder="输入新密钥可覆盖"
            autocomplete="off"
            @keydown.enter="saveKey"
          />
          <button class="btn-link" @click="saveKey" :disabled="saving || !apiKeyInput.trim()">
            {{ saving ? '保存中…' : '更换密钥' }}
          </button>
        </div>
        <div v-if="saveError" class="save-msg err">{{ saveError }}</div>
        <div v-if="saveOk" class="save-msg ok">已更新。</div>
      </div>

      <ul class="tips">
        <li>密钥保存在本机 <code>.env</code>（或系统用户数据目录），只有本程序能读到。</li>
        <li>保存时会即时校验有效性，填错的密钥会直接提示。</li>
        <li>如果你更习惯命令行：也可以手动编辑 <code>.env</code> 后重启。</li>
      </ul>
    </section>

    <!-- 第 2 步 -->
    <section class="step-card card">
      <div class="step-head">
        <span class="step-no">2</span>
        <h3 class="step-title">导入你的文档或代码</h3>
      </div>
      <p class="step-body">
        切到「导入文档」，先选目标项目与子项目，再在「文档目录路径」旁点
        <strong>浏览文件夹…</strong>，从弹出的目录树里选中要导入的目录。
      </p>
      <ul class="tips">
        <li>支持 <code>.md</code> / <code>.go</code> / <code>.py</code> / <code>.js</code> / <code>.ts</code> / <code>.vue</code> / <code>.java</code> / <code>.rs</code> 等文件，按目录递归扫描。</li>
        <li>包内自带示例项目 <code>examples/order-service</code>，可以直接用它练手，不用担心污染自己的仓库。</li>
        <li>导入会调用 DeepSeek 解析文档，消耗少量额度；内容未变的文件会按哈希跳过，重复导入不重复计费。</li>
      </ul>
      <button class="btn-primary step-btn" @click="$emit('go', 'ingest')">去导入文档 →</button>
    </section>

    <!-- 第 3 步 -->
    <section class="step-card card">
      <div class="step-head">
        <span class="step-no">3</span>
        <h3 class="step-title">开始提问</h3>
      </div>
      <p class="step-body">
        点左下角的 <strong>AI 问答</strong>（或右下角悬浮按钮）打开问答面板，直接用自然语言提问，回答会带上出处链接。
      </p>
      <ul class="tips">
        <li>试试：「订单的状态流转规则是什么？」「支付回调如何保证幂等？」</li>
        <li>提问时可以指定项目 / 子项目，把检索范围缩小到某个模块。</li>
        <li>想要一份跨文档的综述，用「综合提炼」按主题生成。</li>
      </ul>
      <button class="btn-primary step-btn" @click="$emit('ask')">打开 AI 问答 →</button>
    </section>
  </div>
</template>

<script>
import { ref, computed, onMounted } from 'vue'
import api from '../api.js'

export default {
  emits: ['go', 'ask', 'configured'],
  props: {
    stats: { type: Object, default: null },
  },
  setup(props, { emit }) {
    const isEmpty = computed(() => !props.stats || !props.stats.total_pages)

    const cfg = ref({ api_key_set: false, api_key_masked: '', model: '' })
    const apiKeyInput = ref('')
    const saving = ref(false)
    const saveError = ref('')
    const saveOk = ref(false)

    async function loadConfig() {
      try {
        cfg.value = await api.getConfig()
      } catch (e) { /* 后端未就绪时保持默认 */ }
    }

    async function saveKey() {
      const key = apiKeyInput.value.trim()
      if (!key) return
      saving.value = true
      saveError.value = ''
      saveOk.value = false
      try {
        cfg.value = await api.setConfig(key)
        apiKeyInput.value = ''
        saveOk.value = true
        emit('configured')
      } catch (e) {
        saveError.value = e.message || '保存失败'
      }
      saving.value = false
    }

    onMounted(loadConfig)

    return { isEmpty, cfg, apiKeyInput, saving, saveError, saveOk, saveKey }
  },
}
</script>

<style scoped>
.guide-panel { max-width: 760px; }
.page-title { font-size: 24px; font-weight: 700; margin-bottom: 6px; }
.page-desc { font-size: 14px; color: var(--color-text-secondary); margin-bottom: 20px; }

.status-banner { display: flex; align-items: center; gap: 14px; padding: 16px 18px; border-radius: 8px; margin-bottom: 20px; }
.status-banner.empty { background: #fffbeb; border: 1px solid #fde68a; }
.status-banner.ready { background: #f0fdf4; border: 1px solid #bbf7d0; }
.status-icon { font-size: 26px; }
.status-title { font-size: 15px; font-weight: 700; }
.status-sub { font-size: 13px; color: var(--color-text-secondary); margin-top: 2px; }

.step-card { padding: 22px 24px; margin-bottom: 16px; }
.step-head { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; }
.step-no { width: 26px; height: 26px; border-radius: 50%; background: var(--color-accent); color: #fff; font-size: 14px; font-weight: 700; display: flex; align-items: center; justify-content: center; flex-shrink: 0; }
.step-title { font-size: 16px; font-weight: 700; flex: 1; }
.step-badge { font-size: 11px; padding: 2px 8px; border-radius: 10px; background: #fef3c7; color: #92400e; }
.step-badge.done { background: #dcfce7; color: #166534; }
.step-body { font-size: 14px; color: var(--color-text-secondary); line-height: 1.7; margin-bottom: 12px; }
.step-body strong { color: var(--color-text); }

.key-form, .key-done { margin-bottom: 10px; }
.key-input-row { display: flex; gap: 10px; align-items: center; margin-bottom: 8px; }
.key-input { flex: 1; padding: 10px 14px; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 13px; }
.save-msg { font-size: 13px; margin-bottom: 8px; }
.save-msg.ok { color: #166534; }
.save-msg.err { color: #dc2626; }

.code-block { background: #1e293b; color: #e2e8f0; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 13px; padding: 12px 16px; border-radius: 6px; overflow-x: auto; margin-bottom: 12px; }

.tips { list-style: none; padding: 0; margin: 0 0 4px; }
.tips li { position: relative; padding-left: 18px; font-size: 13px; color: var(--color-text-muted); line-height: 1.8; }
.tips li::before { content: '·'; position: absolute; left: 6px; font-weight: 700; color: var(--color-accent); }

code { background: var(--color-hover); padding: 1px 5px; border-radius: 3px; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12.5px; color: var(--color-text); }
.step-btn { margin-top: 12px; }
</style>
