<template>
  <div class="wiki-browser">
    <h1 class="page-title">浏览 Wiki</h1>
    <p class="page-desc">选择项目后按分类浏览知识图谱，点击页面查看完整内容。</p>

    <!-- 阶段一：项目列表 -->
    <div v-if="!selectedProject" class="project-grid">
      <div
        v-for="proj in projects"
        :key="proj.name"
        class="project-card"
        @click="selectProject(proj.name)"
      >
        <div class="card-header">
          <span class="card-icon">{{ proj.name === '通用' ? '📋' : '📦' }}</span>
          <span class="card-name">{{ proj.name }}</span>
          <span class="card-badge">{{ proj.total_pages }} 页</span>
        </div>
        <div class="card-desc">{{ truncatedDesc(proj.description) }}</div>
        <div class="card-stats">
          <span class="stat-item"><span class="stat-val">{{ proj.source_pages }}</span><span class="stat-lbl">源文档</span></span>
          <span class="stat-item"><span class="stat-val">{{ proj.entity_pages }}</span><span class="stat-lbl">实体</span></span>
          <span class="stat-item"><span class="stat-val">{{ proj.concept_pages }}</span><span class="stat-lbl">概念</span></span>
          <span class="stat-item"><span class="stat-val">{{ proj.synthesis_pages }}</span><span class="stat-lbl">综合</span></span>
        </div>
      </div>
      <div v-if="projects.length === 0 && loaded" class="empty-hint">暂无项目</div>
    </div>

    <!-- 阶段二：选子项目 -->
    <div v-else-if="selectedProject && !skipSub" class="step-two">
      <button class="btn-back" @click="selectedProject = ''">← 所有项目</button>
      <span class="project-label">{{ selectedProject }} / 子项目</span>

      <div class="sub-grid" v-if="subProjects.length > 0">
        <div v-for="sp in subProjects" :key="sp" class="sub-card"
          @click="selectSub(sp)">
          <span class="sub-icon">📂</span>
          <span class="sub-name">{{ sp }}</span>
        </div>
      </div>
      <div v-else style="margin-top:16px;color:var(--color-text-muted);font-size:13px;">
        该项目暂无子项目数据，请先导入文档。
        <br><br>
        <button class="btn-primary" @click="selectedProject = ''" style="padding:8px 20px">返回项目列表</button>
      </div>
    </div>

    <!-- 阶段三：三栏浏览 -->
    <div v-else class="browser-shell">
      <div class="toolbar">
        <button class="btn-back" @click="goBackSub">← {{ selectedSub ? '选择子项目' : '所有项目' }}</button>
        <span class="toolbar-title">{{ selectedProject }}<template v-if="selectedSub"> / {{ selectedSub }}</template></span>
      </div>
      <div class="shell-body">
        <!-- 分类列 -->
        <div class="col col-cat">
          <div class="col-header">分类</div>
          <div
            v-for="cat in categories"
            :key="cat.id"
            :class="['cat-item', { active: activeCat === cat.id }]"
            @click="selectCategory(cat.id)"
          >
            <span class="cat-name">{{ cat.label }}</span>
            <span class="cat-count">{{ cat.count }}</span>
          </div>
        </div>

        <!-- 页面列表列 -->
        <div class="col col-list">
          <div class="col-header">页面列表</div>
          <div class="search-box">
            <input v-model="search" class="form-input" placeholder="搜索页面..." />
          </div>
          <div class="list-scroll">
            <div
              v-for="page in filteredPages"
              :key="page.name"
              :class="['list-item', { active: activePage === page.name }]"
              @click="loadPage(page)"
            >
              {{ page.name.replace(/-/g, ' ').replace(/\.md$/, '') }}
            </div>
            <div v-if="filteredPages.length === 0" class="empty-hint">暂无页面</div>
          </div>
        </div>

        <!-- 内容列 -->
        <div class="col col-content">
          <div class="col-header">{{ activePage ? activePage.replace(/-/g, ' ').replace(/\.md$/, '') : '内容预览' }}</div>
          <div class="content-scroll">
            <div v-if="loading" class="loading-state">加载中...</div>
            <div v-else-if="pageMeta || pageHtml" class="page-view">
              <!-- frontmatter 元数据卡片 -->
              <div v-if="pageMeta" class="meta-card">
                <div class="meta-row" v-if="pageMeta.title">
                  <span class="meta-label">标题</span>
                  <span class="meta-value meta-title">{{ pageMeta.title }}</span>
                </div>
                <div class="meta-tags">
                  <span v-if="pageMeta.type" :class="['meta-badge', 'badge-type']">{{ typeLabel(pageMeta.type) }}</span>
                  <span v-if="pageMeta.status" :class="['meta-badge', 'badge-status', pageMeta.status]">{{ statusLabel(pageMeta.status) }}</span>
                  <span v-if="pageMeta.confidence" :class="['meta-badge', 'badge-confidence', pageMeta.confidence]">{{ confidenceLabel(pageMeta.confidence) }}</span>
                  <span v-if="pageMeta.created" class="meta-date">{{ formatDate(pageMeta.created) }}</span>
                </div>
                <div v-if="pageMeta.sources && pageMeta.sources.length" class="meta-row">
                  <span class="meta-label">来源</span>
                  <span class="meta-sources">
                    <span v-for="s in pageMeta.sources" :key="s" class="source-tag">{{ s }}</span>
                  </span>
                </div>
                <div v-if="pageMeta.tags && pageMeta.tags.length" class="meta-row">
                  <span class="meta-label">标签</span>
                  <span class="meta-tags-list">
                    <span v-for="t in pageMeta.tags" :key="t" class="tag-chip">{{ t }}</span>
                  </span>
                </div>
              </div>
              <!-- 正文 -->
              <div class="markdown-body" v-html="pageHtml"></div>
            </div>
            <div v-else class="empty-state">选择左侧页面查看内容</div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, computed, onMounted } from 'vue'
import { parseMarkdown } from './MarkdownRenderer.js'
import api from '../api.js'

export default {
  setup() {
    const projects = ref([])
    const loaded = ref(false)
    const selectedProject = ref('')
    const selectedSub = ref('')
    const skipSub = ref(false)

    const categories = [
      { id: 'sources', label: '源文档', count: 0 },
      { id: 'entities', label: '实体', count: 0 },
      { id: 'concepts', label: '概念', count: 0 },
      { id: 'syntheses', label: '综合', count: 0 },
      { id: 'contradictions', label: '矛盾', count: 0 },
    ]
    const activeCat = ref('sources')
    const activePage = ref('')
    const search = ref('')
    const pageContent = ref('')
    const pageMeta = ref(null)       // frontmatter 元数据
    const pageHtml = ref('')         // 渲染后的 HTML（不含 frontmatter）
    const loading = ref(false)
    const allPages = ref({ sources: [], entities: [], concepts: [], syntheses: [], contradictions: [] })
    const subProjects = ref([])

    onMounted(async () => {
      try { projects.value = await api.listProjects() } catch (e) { /* */ }
      loaded.value = true
    })

    async function selectProject(name) {
      selectedProject.value = name
      selectedSub.value = ''
      skipSub.value = false
      activeCat.value = 'sources'; activePage.value = ''; pageContent.value = ''; search.value = ''
      // 获取项目统计和子项目列表
      try {
        const info = await api.getProject(name)
        categories[0].count = info.source_pages
        categories[1].count = info.entity_pages
        categories[2].count = info.concept_pages
        categories[3].count = info.synthesis_pages
        categories[4].count = 0
        subProjects.value = (info.sub_project_list || []).map(s => s.name)
        if (!subProjects.value.length) subProjects.value = []
        // 同时从 sub-projects API 获取（可能有未在 project.md 中注册的）
        const apiSubs = await api.listSubProjects(name)
        for (const s of apiSubs) {
          if (!subProjects.value.includes(s)) subProjects.value.push(s)
        }
      } catch (e) { /* */ }
      allPages.value = { sources: [], entities: [], concepts: [], syntheses: [], contradictions: [] }
      // 如果没有子项目，直接进入浏览（兼容通用项目等无子项目的旧数据）
      if (subProjects.value.length === 0) skipSub.value = true
      else if (subProjects.value.length === 1) selectSub(subProjects.value[0]) // 单个子项目自动进入
    }

    function selectSub(sp) {
      selectedSub.value = sp
      skipSub.value = true
      activeCat.value = 'sources'; activePage.value = ''; pageContent.value = ''; search.value = ''
      allPages.value = { sources: [], entities: [], concepts: [], syntheses: [], contradictions: [] }
      loadPageList('sources')
    }

    function goBackSub() {
      if (selectedSub.value) {
        selectedSub.value = ''; skipSub.value = false
      } else {
        selectedProject.value = ''
      }
    }

    async function selectCategory(id) {
      activeCat.value = id; activePage.value = ''; pageContent.value = ''; search.value = ''
      await loadPageList(id)
    }

    async function loadPageList(cat) {
      if (!selectedProject.value) return
      if (allPages.value[cat].length > 0) return
      try {
        let url = `/api/wiki/list/${cat}?project=${encodeURIComponent(selectedProject.value)}`
        if (selectedSub.value) url += `&sub_project=${encodeURIComponent(selectedSub.value)}`
        const resp = await fetch(url)
        const data = await resp.json()
        allPages.value[cat] = data.pages || []
      } catch (e) { console.error(e) }
    }

    async function loadPage(page) {
      activePage.value = page.name; loading.value = true; pageMeta.value = null; pageHtml.value = ''
      try {
        let url = `/api/wiki/page/${activeCat.value}/${page.name}?project=${encodeURIComponent(selectedProject.value)}`
        if (selectedSub.value) url += `&sub_project=${encodeURIComponent(selectedSub.value)}`
        const resp = await fetch(url)
        const raw = await resp.text()
        const parsed = parseMarkdown(raw)
        pageMeta.value = parsed.meta
        pageHtml.value = parsed.html
        pageContent.value = raw // 保留原始文本供兼容
      } catch (e) {
        pageHtml.value = '<p class="error-msg">加载失败: ' + e.message + '</p>'
      }
      loading.value = false
    }

    const filteredPages = computed(() => {
      const pages = allPages.value[activeCat.value] || []
      if (!search.value) return pages
      const q = search.value.toLowerCase()
      return pages.filter(p => p.name.toLowerCase().includes(q))
    })

    function truncatedDesc(desc) {
      if (!desc) return ''
      return desc.length > 80 ? desc.slice(0, 80) + '...' : desc
    }

    function typeLabel(type) {
      const map = { source: '源文档', entity: '实体', concept: '概念', synthesis: '综合', contradiction: '矛盾' }
      return map[type] || type
    }

    function statusLabel(status) {
      const map = { published: '已发布', draft: '草稿', archived: '已归档' }
      return map[status] || status
    }

    function confidenceLabel(conf) {
      const map = { high: '高可信', medium: '中可信', low: '低可信' }
      return map[conf] || conf
    }

    function formatDate(d) {
      if (!d) return ''
      const s = String(d)
      // 支持 "2026-06-18" 或 "2026-06-18T10:00:00Z" 格式
      return s.slice(0, 10)
    }

    return { projects, loaded, selectedProject, selectedSub, skipSub, categories, activeCat, activePage, search, pageContent, pageMeta, pageHtml, loading, filteredPages, subProjects, selectProject, selectSub, goBackSub, selectCategory, loadPage, truncatedDesc, typeLabel, statusLabel, confidenceLabel, formatDate }
  },
}
</script>

<style scoped>
/* ---------- 阶段一：项目格块（与首页一致） ---------- */
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

.card-header { display: flex; align-items: center; gap: 8px; margin-bottom: 10px; }

.card-icon { font-size: 22px; }

.card-name { font-size: 17px; font-weight: 700; color: var(--color-text); flex: 1; }

.card-badge {
  font-size: 11px; font-weight: 600; padding: 2px 8px;
  background: var(--color-accent-light); color: var(--color-accent); border-radius: 10px;
}

.card-desc {
  font-size: 13px; color: var(--color-text-secondary); line-height: 1.55; margin-bottom: 14px;
  display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden;
}

.card-stats {
  display: flex; gap: 0; border-top: 1px solid var(--color-border); padding-top: 12px;
}

.stat-item { flex: 1; text-align: center; }

.stat-val { display: block; font-size: 18px; font-weight: 700; color: var(--color-text); line-height: 1.1; }

.stat-lbl { display: block; font-size: 11px; color: var(--color-text-muted); margin-top: 2px; }

/* ---------- 阶段二：三栏浏览器 ---------- */
.toolbar {
  display: flex; align-items: center; gap: 12px;
  padding: 10px 16px; background: #fafbfc; border-bottom: 1px solid var(--color-border);
  border-radius: var(--radius) var(--radius) 0 0;
}

.btn-back {
  padding: 4px 12px; background: none; border: 1px solid var(--color-border); border-radius: 4px;
  font-size: 13px; color: var(--color-text-secondary); cursor: pointer; font-family: inherit;
}

.btn-back:hover { background: var(--color-hover); }

.toolbar-title { font-size: 14px; font-weight: 600; color: var(--color-text); }

.browser-shell {
  border: 1px solid var(--color-border); border-radius: var(--radius);
  overflow: hidden; background: #fff; margin-top: 0;
}

.shell-body {
  display: flex; height: calc(100vh - 230px); min-height: 450px;
}

.col { display: flex; flex-direction: column; }

.col-header {
  padding: 10px 14px; font-size: 12px; font-weight: 600;
  color: var(--color-text-muted); background: #fafbfc;
  border-bottom: 1px solid var(--color-border); flex-shrink: 0;
}

.col-cat { width: 120px; border-right: 1px solid var(--color-border); flex-shrink: 0; }

.cat-item {
  display: flex; align-items: center; justify-content: space-between;
  padding: 9px 14px; cursor: pointer; font-size: 13px;
  color: var(--color-text-secondary); border-bottom: 1px solid var(--color-border);
  transition: background 0.1s;
}

.cat-item:hover { background: var(--color-hover); }

.cat-item.active { background: var(--color-accent-light); color: var(--color-accent); font-weight: 600; }

.cat-count { font-size: 11px; color: var(--color-text-muted); }

.cat-item.active .cat-count { color: var(--color-accent); }

.col-list { width: 210px; border-right: 1px solid var(--color-border); flex-shrink: 0; }

.search-box { padding: 8px; border-bottom: 1px solid var(--color-border); }

.search-box input {
  width: 100%; padding: 6px 10px; font-size: 13px;
  border: 1px solid var(--color-border); border-radius: 4px; background: var(--color-hover);
}

.search-box input:focus { background: #fff; }

.list-scroll { flex: 1; overflow-y: auto; }

.list-item {
  padding: 8px 14px; cursor: pointer; font-size: 13px;
  color: var(--color-text-secondary); border-bottom: 1px solid var(--color-border);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  transition: background 0.1s;
}

.list-item:hover { background: var(--color-hover); }

.list-item.active { background: var(--color-accent-light); color: var(--color-accent); font-weight: 500; }

.col-content { flex: 1; min-width: 0; }

.content-scroll { flex: 1; overflow-y: auto; padding: 24px 30px; }

.page-view { max-width: 780px; }

/* ---------- 元数据卡片 ---------- */
.meta-card {
  background: linear-gradient(135deg, #f8f9ff 0%, #f0f3ff 100%);
  border: 1px solid #e0e4f7;
  border-radius: 10px;
  padding: 18px 22px;
  margin-bottom: 24px;
}

.meta-row {
  display: flex; align-items: flex-start; gap: 12px;
  margin-bottom: 8px; font-size: 13px;
}

.meta-row:last-child { margin-bottom: 0; }

.meta-label {
  flex-shrink: 0; color: var(--color-text-muted);
  font-weight: 600; min-width: 36px; padding-top: 1px;
}

.meta-value { color: var(--color-text); }

.meta-title { font-size: 16px; font-weight: 700; }

.meta-tags {
  display: flex; flex-wrap: wrap; align-items: center; gap: 8px;
  margin: 10px 0 12px 48px;
}

.meta-badge {
  display: inline-block; padding: 2px 10px; border-radius: 4px;
  font-size: 11px; font-weight: 600;
}

.badge-type { background: #eef0ff; color: var(--color-accent); }
.badge-status.published { background: #ecfdf5; color: #047857; }
.badge-status.draft { background: #fffbeb; color: #8a5a00; }
.badge-status.archived { background: #f3f4f6; color: #6b7280; }
.badge-confidence.high { background: #ecfdf5; color: #047857; }
.badge-confidence.medium { background: #fffbeb; color: #8a5a00; }
.badge-confidence.low { background: #fef3f2; color: #b91c1c; }

.meta-date {
  font-size: 11px; color: var(--color-text-muted);
  margin-left: auto;
}

.meta-sources {
  display: flex; flex-wrap: wrap; gap: 4px;
}

.source-tag {
  display: inline-block; padding: 1px 8px; border-radius: 3px;
  font-size: 11px; background: #e8eaed; color: var(--color-text-secondary);
}

.tag-chip {
  display: inline-block; padding: 1px 8px; border-radius: 3px;
  font-size: 11px; background: var(--color-accent-light); color: var(--color-accent);
}

.error-msg { color: #b91c1c; }

.empty-state, .empty-hint {
  display: flex; align-items: center; justify-content: center;
  height: 100%; color: var(--color-text-muted); font-size: 14px;
}

.empty-hint { padding: 24px 14px; text-align: center; }

/* 阶段二：子项目选择 */
.step-two { max-width: 700px; margin-top: 0; }
.project-label { margin-left: 10px; font-size: 15px; font-weight: 600; color: var(--color-text); display: inline-block; padding: 4px 12px; background: var(--color-accent-light); border-radius: 4px; margin-bottom: 16px; }
.sub-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 12px; margin-top: 14px; }
.sub-card { background: #fff; border: 1px solid var(--color-border); border-radius: var(--radius); padding: 18px; cursor: pointer; transition: all 0.15s; display: flex; align-items: center; gap: 12px; }
.sub-card:hover { border-color: var(--color-accent); box-shadow: 0 2px 8px rgba(79,108,247,0.06); transform: translateY(-1px); }
.sub-icon { font-size: 22px; }
.sub-name { font-size: 14px; font-weight: 600; color: var(--color-text); }
</style>
