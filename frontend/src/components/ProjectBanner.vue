<template>
  <div class="project-section">
    <h2 class="section-title">知识库项目</h2>
    <p class="section-desc">每个项目拥有独立的文档空间，点击进入项目内搜索问答。</p>
    <div class="project-grid" v-if="projects.length > 0">
      <div
        v-for="proj in projects"
        :key="proj.name"
        class="project-card"
        @click="$emit('select', proj.name)"
      >
        <div class="card-header">
          <span class="card-icon">{{ proj.name === '通用' ? '📋' : '📦' }}</span>
          <span class="card-name">{{ proj.name }}</span>
          <span class="card-badge">{{ proj.total_pages }} 页</span>
        </div>
        <div class="card-desc">{{ truncatedDesc(proj.description) }}</div>
        <div class="card-stats">
          <span class="stat-item" title="源文档">
            <span class="stat-val">{{ proj.source_pages }}</span>
            <span class="stat-lbl">源文档</span>
          </span>
          <span class="stat-item" title="实体">
            <span class="stat-val">{{ proj.entity_pages }}</span>
            <span class="stat-lbl">实体</span>
          </span>
          <span class="stat-item" title="概念">
            <span class="stat-val">{{ proj.concept_pages }}</span>
            <span class="stat-lbl">概念</span>
          </span>
          <span class="stat-item" title="综合提炼">
            <span class="stat-val">{{ proj.synthesis_pages }}</span>
            <span class="stat-lbl">综合</span>
          </span>
        </div>
      </div>
    </div>
    <div class="project-empty" v-else-if="loaded">
      暂无项目，在命令行创建：curl -X POST /api/projects -d '{"name":"项目名","description":"...","sub_projects":"..."}'
    </div>
  </div>
</template>

<script>
import { ref, onMounted } from 'vue'
import api from '../api.js'

export default {
  emits: ['select'],
  setup() {
    const projects = ref([])
    const loaded = ref(false)

    onMounted(async () => {
      try {
        projects.value = await api.listProjects()
      } catch (e) { /* ignore */ }
      loaded.value = true
    })

    function truncatedDesc(desc) {
      if (!desc) return ''
      return desc.length > 80 ? desc.slice(0, 80) + '...' : desc
    }

    return { projects, loaded, truncatedDesc }
  },
}
</script>

<style scoped>
.project-section {
  margin-top: 36px;
  padding-top: 28px;
  border-top: 1px solid var(--color-border);
}

.section-title {
  font-size: 18px;
  font-weight: 700;
  margin-bottom: 4px;
}

.section-desc {
  font-size: 13px;
  color: var(--color-text-muted);
  margin-bottom: 18px;
}

.project-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 14px;
}

.project-card {
  background: #fff;
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  padding: 20px;
  cursor: pointer;
  transition: all 0.15s;
}

.project-card:hover {
  border-color: var(--color-accent);
  box-shadow: 0 2px 12px rgba(79,108,247,0.08);
  transform: translateY(-1px);
}

.card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
}

.card-icon { font-size: 22px; }

.card-name {
  font-size: 17px;
  font-weight: 700;
  color: var(--color-text);
  flex: 1;
}

.card-badge {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  background: var(--color-accent-light);
  color: var(--color-accent);
  border-radius: 10px;
}

.card-desc {
  font-size: 13px;
  color: var(--color-text-secondary);
  line-height: 1.55;
  margin-bottom: 14px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.card-stats {
  display: flex;
  gap: 0;
  border-top: 1px solid var(--color-border);
  padding-top: 12px;
}

.stat-item {
  flex: 1;
  text-align: center;
}

.stat-val {
  display: block;
  font-size: 18px;
  font-weight: 700;
  color: var(--color-text);
  line-height: 1.1;
}

.stat-lbl {
  display: block;
  font-size: 11px;
  color: var(--color-text-muted);
  margin-top: 2px;
}

.project-empty {
  padding: 24px;
  background: #fafbfc;
  border: 1px dashed var(--color-border);
  border-radius: var(--radius);
  color: var(--color-text-muted);
  font-size: 13px;
  text-align: center;
}
</style>
