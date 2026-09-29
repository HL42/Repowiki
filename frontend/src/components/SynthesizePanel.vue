<template>
  <div class="synthesize-panel">
    <h1 class="page-title">综合提炼</h1>
    <p class="page-desc">选择项目后，指定主题让 LLM 跨文档综合分析，发现矛盾与知识空白。</p>

    <!-- 阶段一：选项目 -->
    <div v-if="!selectedProject" class="project-grid">
      <div v-for="proj in projects" :key="proj.name" class="project-card"
        @click="selectedProject = proj.name">
        <div class="card-header">
          <span class="card-icon">{{ proj.name === '通用' ? '📋' : '📦' }}</span>
          <span class="card-name">{{ proj.name }}</span>
          <span class="card-badge">{{ proj.total_pages }} 页</span>
        </div>
        <div class="card-desc">{{ truncatedDesc(proj.description) }}</div>
        <div class="card-stats">
          <span class="stat-item"><span class="stat-val">{{ proj.source_pages }}</span><span class="stat-lbl">源文档</span></span>
          <span class="stat-item"><span class="stat-val">{{ proj.synthesis_pages }}</span><span class="stat-lbl">综合</span></span>
        </div>
      </div>
      <div v-if="projects.length === 0 && loaded" class="empty-hint">暂无项目</div>
    </div>

    <!-- 阶段二：选子项目（有子项目时显示） -->
    <div v-else-if="selectedProject && !skipSub">
      <button class="btn-back" @click="selectedProject = ''">← 选择项目</button>
      <span class="project-label">{{ selectedProject }}</span>

      <div class="sub-select-area" v-if="subProjects.length > 0">
        <p class="sub-hint">选择要提炼的子项目范围：</p>
        <div class="sub-options">
          <label class="sub-option" :class="{active: selectedSub === ''}">
            <input type="radio" value="" v-model="selectedSub"> 全部子项目
          </label>
          <label v-for="sp in subProjects" :key="sp" class="sub-option"
            :class="{active: selectedSub === sp}">
            <input type="radio" :value="sp" v-model="selectedSub"> {{ sp }}
          </label>
        </div>
        <button class="btn-primary" style="margin-top:14px" @click="skipSub = true">继续</button>
      </div>
      <div v-else style="margin-top:16px">
        <p style="color:var(--color-text-muted);font-size:13px;">该项目暂无子项目，将直接在项目范围内提炼。</p>
        <button class="btn-primary" style="margin-top:10px" @click="skipSub = true">继续</button>
      </div>
    </div>

    <!-- 阶段三：提炼 -->
    <div v-else>
      <button class="btn-back" @click="selectedProject = ''">← 选择项目</button>
      <span class="project-label">{{ selectedProject }}</span>

      <div class="synthesize-form">
        <div class="input-wrap">
          <input v-model="topic" class="form-input synth-input"
            placeholder="输入主题，比如：错误处理" @keydown.enter="synthesize" />
          <button class="btn-primary" @click="synthesize" :disabled="loading">
            {{ loading ? '分析中...' : '开始提炼' }}
          </button>
        </div>
      </div>

      <div v-if="loading" class="loading-state">正在综合分析相关文档...</div>

      <div v-if="result" class="result-card card">
        <div class="result-body">{{ result.result }}</div>
        <div v-if="result.page_path" class="result-path">
          <span>新页面: </span><code>{{ result.page_path }}</code>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, onMounted, watch } from 'vue'
import api from '../api.js'

export default {
  emits: ['done'],
  setup(props, { emit }) {
    const projects = ref([])
    const loaded = ref(false)
    const selectedProject = ref('')
    const selectedSub = ref('')
    const skipSub = ref(false)
    const topic = ref('')
    const result = ref(null)
    const loading = ref(false)
    const subProjects = ref([])

    async function loadSubProjects() {
      try {
        subProjects.value = await api.listSubProjects(selectedProject.value)
        const projInfo = await api.getProject(selectedProject.value)
        if (projInfo.sub_project_list?.length) {
          for (const sp of projInfo.sub_project_list) {
            if (!subProjects.value.includes(sp.name)) subProjects.value.push(sp.name)
          }
        }
      } catch (e) { subProjects.value = [] }
    }

    onMounted(async () => {
      try { projects.value = await api.listProjects() } catch (e) { /* */ }
      loaded.value = true
    })

    watch(selectedProject, async (newVal) => {
      if (newVal) {
        skipSub.value = false
        selectedSub.value = ''
        await loadSubProjects()
      }
    })

    async function synthesize() {
      if (!topic.value.trim()) return
      loading.value = true; result.value = null
      try {
        result.value = await api.synthesize(topic.value, selectedProject.value, selectedSub.value || undefined)
        emit('done')
      } catch (e) {
        result.value = { result: '请求失败: ' + e.message }
      }
      loading.value = false
    }

    function truncatedDesc(desc) {
      if (!desc) return ''
      return desc.length > 60 ? desc.slice(0, 60) + '...' : desc
    }

    return { projects, loaded, selectedProject, selectedSub, skipSub, topic, result, loading, subProjects, synthesize, truncatedDesc, loadSubProjects }
  },
}
</script>

<style scoped>
.project-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 14px; margin-bottom: 24px; }
.project-card { background: #fff; border: 1px solid var(--color-border); border-radius: var(--radius); padding: 16px; cursor: pointer; transition: all 0.15s; }
.project-card:hover { border-color: var(--color-accent); box-shadow: 0 2px 12px rgba(79,108,247,0.08); transform: translateY(-1px); }
.card-header { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; }
.card-icon { font-size: 20px; }
.card-name { font-size: 15px; font-weight: 700; flex: 1; }
.card-badge { font-size: 10px; padding: 1px 6px; background: var(--color-accent-light); color: var(--color-accent); border-radius: 8px; }
.card-desc { font-size: 12px; color: var(--color-text-muted); line-height: 1.4; margin-bottom: 8px; }
.card-stats { display: flex; gap: 0; border-top: 1px solid var(--color-border); padding-top: 8px; }
.stat-item { flex: 1; text-align: center; }
.stat-val { display: block; font-size: 16px; font-weight: 700; }
.stat-lbl { display: block; font-size: 10px; color: var(--color-text-muted); margin-top: 1px; }
.empty-hint { grid-column: 1/-1; padding: 24px; text-align: center; color: var(--color-text-muted); }

.btn-back { padding: 4px 12px; background: none; border: 1px solid var(--color-border); border-radius: 4px; font-size: 13px; color: var(--color-text-secondary); cursor: pointer; font-family: inherit; margin-bottom: 16px; }
.btn-back:hover { background: var(--color-hover); }
.project-label { margin-left: 10px; font-size: 14px; font-weight: 600; color: var(--color-text); }

.synthesize-panel { max-width: 700px; }
.synthesize-form { margin-top: 20px; }
.input-wrap { display: flex; border: 1px solid var(--color-border); border-radius: 6px; overflow: hidden; }
.input-wrap:focus-within { border-color: var(--color-accent); box-shadow: 0 0 0 3px rgba(79,108,247,0.08); }
.synth-input { flex: 1; padding: 11px 14px; border: none; border-radius: 0; font-size: 14px; }
.synth-input:focus { outline: none; box-shadow: none; }

.result-card { padding: 24px; margin-top: 20px; }
.result-body { font-size: 14px; line-height: 1.7; white-space: pre-wrap; }
.result-path { margin-top: 16px; font-size: 13px; color: var(--color-text-muted); }
.result-path code { padding: 2px 6px; background: var(--color-hover); border-radius: 3px; font-size: 12px; }

.sub-select-area { margin-top: 16px; }
.sub-hint { font-size: 13px; color: var(--color-text-secondary); margin-bottom: 10px; }
.sub-options { display: flex; flex-wrap: wrap; gap: 8px; }
.sub-option { display: flex; align-items: center; gap: 6px; padding: 8px 14px; border: 1px solid var(--color-border); border-radius: 6px; cursor: pointer; font-size: 13px; transition: all 0.1s; }
.sub-option:hover { border-color: var(--color-accent); }
.sub-option.active { border-color: var(--color-accent); background: var(--color-accent-light); color: var(--color-accent); font-weight: 600; }
.sub-option input { accent-color: var(--color-accent); }
</style>
