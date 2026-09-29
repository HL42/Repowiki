<template>
  <div class="stats-panel">
    <h1 class="page-title">知识库统计</h1>
    <p class="page-desc">当前知识库索引概况。每次导入或综合提炼后数据会自动更新。</p>

    <div v-if="loadError" class="stats-error">{{ loadError }}</div>

    <div class="stats-grid" v-if="stats">
      <div class="stat-item">
        <div class="stat-num">{{ stats.total_pages.toLocaleString() }}</div>
        <div class="stat-label">总页数</div>
      </div>
      <div class="stat-item">
        <div class="stat-num">{{ stats.source_pages.toLocaleString() }}</div>
        <div class="stat-label">源文档摘要</div>
      </div>
      <div class="stat-item">
        <div class="stat-num">{{ stats.entity_pages.toLocaleString() }}</div>
        <div class="stat-label">实体页</div>
      </div>
      <div class="stat-item">
        <div class="stat-num">{{ stats.concept_pages.toLocaleString() }}</div>
        <div class="stat-label">概念页</div>
      </div>
      <div class="stat-item">
        <div class="stat-num">{{ stats.synthesis_pages.toLocaleString() }}</div>
        <div class="stat-label">综合提炼</div>
      </div>
      <div v-if="stats.stale_docs" class="stat-item stat-stale">
        <div class="stat-num">{{ stats.stale_docs.toLocaleString() }}</div>
        <div class="stat-label">过期综合页</div>
      </div>
    </div>

    <div class="stats-action">
      <button class="btn-primary" @click="loadStats">刷新统计</button>
      <span v-if="lastUpdated" class="update-time">更新于 {{ lastUpdated }}</span>
    </div>
  </div>
</template>

<script>
import { ref, onMounted } from 'vue'
import api from '../api.js'

export default {
  setup() {
    const stats = ref(null)
    const lastUpdated = ref('')
    const loadError = ref('')

    async function loadStats() {
      loadError.value = ''
      try {
        stats.value = await api.stats()
        lastUpdated.value = new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
      } catch (e) {
        loadError.value = '加载统计失败: ' + (e.message || '网络错误，请确保后端已启动或刷新页面重试')
      }
    }

    onMounted(loadStats)
    return { stats, loadStats, lastUpdated, loadError }
  },
}
</script>

<style scoped>
.stats-panel { max-width: 820px; }

.stats-error {
  padding: 14px 18px;
  background: #fef2f2;
  border: 1px solid #fecaca;
  border-radius: 6px;
  color: #991b1b;
  font-size: 13px;
  margin-bottom: 18px;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
  gap: 14px;
  margin-bottom: 24px;
}

.stat-item {
  background: #fff;
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  padding: 22px 20px;
  text-align: center;
}

.stat-num {
  font-size: 32px;
  font-weight: 700;
  color: var(--color-accent);
  line-height: 1.1;
  margin-bottom: 6px;
}

.stat-label {
  font-size: 13px;
  color: var(--color-text-secondary);
}

.stat-stale .stat-num {
  color: #d97706;
}
.stat-stale {
  border-color: #fde68a;
  background: #fffbeb;
}

.stats-action {
  display: flex;
  align-items: center;
  gap: 14px;
}

.update-time {
  font-size: 12.5px;
  color: var(--color-text-muted);
}
</style>
