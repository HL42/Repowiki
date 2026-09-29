<template>
  <div class="ingest-panel">
    <h1 class="page-title">导入文档</h1>
    <p class="page-desc">选择目标项目和子项目，然后导入文档目录。</p>

    <!-- 阶段一：选项目 -->
    <div v-if="step === 1" class="project-grid">
      <div v-for="proj in projects" :key="proj.name" class="project-card"
        @click="selectProject(proj.name)">
        <div class="card-header">
          <span class="card-icon">{{ proj.name === '通用' ? '📋' : '📦' }}</span>
          <span class="card-name">{{ proj.name }}</span>
          <span class="card-badge">{{ proj.total_pages }} 页</span>
        </div>
        <div class="card-desc">{{ truncatedDesc(proj.description) }}</div>
        <div v-if="proj.sub_project_list?.length" class="card-sub-info">
          {{ proj.sub_project_list.length }} 个子项目
        </div>
      </div>
      <!-- 创建新项目 -->
      <div class="project-card card-new" @click="showNewProj = true">
        <div class="card-new-center">
          <span class="new-icon">＋</span>
          <span class="new-text">创建新项目</span>
        </div>
      </div>
    </div>

    <!-- 新建项目对话框 -->
    <div v-if="showNewProj" class="new-project-form card">
      <h3>创建新项目</h3>
      <div class="form-row"><label>项目名称</label><input v-model="newProjName" class="form-input" placeholder="例：用户中心" /></div>
      <div class="form-row"><label>项目简介</label><textarea v-model="newProjDesc" class="form-input" rows="2" placeholder="一句话描述..."></textarea></div>
      <div class="form-actions">
        <button class="btn-primary" @click="createProj" :disabled="creating || !newProjName.trim()">{{ creating ? '创建中...' : '创建' }}</button>
        <button class="btn-link" @click="showNewProj = false">取消</button>
      </div>
      <div v-if="createMsg" class="create-msg" :class="createOk ? 'ok' : 'err'">{{ createMsg }}</div>
    </div>

    <!-- 阶段二：选/建子项目 -->
    <div v-else-if="step === 2" class="step-two">
      <button class="btn-back" @click="step = 1; selectedSub = ''">← 选择项目</button>
      <span class="project-label">{{ selectedProject }} / 子项目</span>

      <div class="sub-grid">
        <div v-for="sub in subProjects" :key="sub" class="sub-card"
          @click="selectedSub = sub; step = 3">
          <span class="sub-icon">📂</span>
          <span class="sub-name">{{ sub }}</span>
        </div>
        <div class="sub-card card-new" @click="showNewSub = true">
          <div class="card-new-center">
            <span class="new-icon">＋</span>
            <span class="new-text">新建子项目</span>
          </div>
        </div>
        <div v-if="subProjects.length === 0 && loadedSubs && !showNewSub" class="empty-hint">
          暂无子项目，请先创建一个。
        </div>
      </div>

      <!-- 新建子项目 -->
      <div v-if="showNewSub" class="card new-sub-form">
        <h4>新建子项目</h4>
        <input v-model="newSubName" class="form-input" placeholder="例：yl-order-service"
          style="margin-bottom:10px" />
        <div class="form-actions">
          <button class="btn-primary" @click="createSub" :disabled="!newSubName.trim()">创建并继续</button>
          <button class="btn-link" @click="showNewSub = false">取消</button>
        </div>
      </div>
    </div>

    <!-- 阶段三：输入路径导入 -->
    <div v-else class="step-three">
      <button class="btn-back" @click="step = 2">← 选择子项目</button>
      <span class="path-breadcrumb">{{ selectedProject }} / {{ selectedSub }}</span>

      <div class="ingest-card card">
        <div class="ingest-form">
          <label class="form-label">文档目录路径</label>
          <div class="input-wrap">
            <input v-model="path" class="form-input ingest-input"
              placeholder="/Users/hl/yl-order-service 或代码目录" @keydown.enter="doIngest" />
            <button class="btn-primary ingest-btn" @click="doIngest" :disabled="loading">
              {{ loading ? '导入中...' : '开始导入' }}
            </button>
          </div>
          <p class="form-hint">文件将导入到「{{ selectedSub }}」的独立空间。该目录下 .md/.go/.py/.js/.ts/.vue/.java/.rs 文件将被扫描导入。</p>
        </div>

        <div v-if="result" class="ingest-result">
          <div v-if="result.validation && (result.validation.warnings?.length || result.validation.errors?.length)" class="validation-section">
            <div class="validation-title">预检查报告</div>
            <div v-if="result.validation.errors?.length" class="validation-errors">
              <div v-for="(e, i) in result.validation.errors" :key="'ve'+i" class="v-item v-err">{{ e }}</div>
            </div>
            <div v-if="result.validation.warnings?.length" class="validation-warnings">
              <div v-for="(w, i) in result.validation.warnings" :key="'vw'+i" class="v-item v-warn">{{ w }}</div>
            </div>
          </div>
          <div class="result-header" :class="result.errors?.length ? 'has-errors' : 'success'">
            <span v-if="!result.errors?.length">导入完成: {{ result.success }} 个文档处理成功</span>
            <span v-else>导入完成: {{ result.success }} 成功, {{ result.errors.length }} 失败</span>
          </div>
          <div v-if="result.errors?.length" class="error-list">
            <div v-for="(err, i) in result.errors" :key="i" class="error-item">{{ err }}</div>
          </div>
        </div>
        <div v-if="error" class="error-banner">{{ error }}</div>
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
    const step = ref(1)
    const projects = ref([])
    const selectedProject = ref('')
    const selectedSub = ref('')
    const path = ref('')
    const loading = ref(false)
    const result = ref(null)
    const error = ref('')
    const loaded = ref(false)

    // 项目
    const showNewProj = ref(false)
    const newProjName = ref('')
    const newProjDesc = ref('')
    const creating = ref(false)
    const createMsg = ref('')
    const createOk = ref(false)

    // 子项目
    const subProjects = ref([])
    const loadedSubs = ref(false)
    const showNewSub = ref(false)
    const newSubName = ref('')

    onMounted(async () => {
      try { projects.value = await api.listProjects() } catch (e) { /* */ }
    })

    async function selectProject(name) {
      selectedProject.value = name
      step.value = 2
      loadedSubs.value = false
      // 获取子项目列表（从 API 获取 + 从 projectInfo 的 sub_project_list）
      try { 
        subProjects.value = await api.listSubProjects(name) 
        // 同时检查 project info 中有没有已注册的子项目
        const projInfo = await api.getProject(name)
        if (projInfo.sub_project_list?.length) {
          for (const sp of projInfo.sub_project_list) {
            if (!subProjects.value.includes(sp.name)) {
              subProjects.value.push(sp.name)
            }
          }
        }
      } catch (e) { subProjects.value = [] }
      loadedSubs.value = true
    }

    async function createProj() {
      creating.value = true; createMsg.value = ''
      try {
        await api.createProject(newProjName.value.trim(), newProjDesc.value.trim(), '')
        createOk.value = true; createMsg.value = '项目创建成功！'
        projects.value = await api.listProjects()
        setTimeout(() => {
          selectProject(newProjName.value)
          showNewProj.value = false
          newProjName.value = ''; newProjDesc.value = ''; createMsg.value = ''
        }, 600)
      } catch (e) { createOk.value = false; createMsg.value = '创建失败: ' + e.message }
      creating.value = false
    }

    async function createSub() {
      if (!newSubName.value.trim()) return
      selectedSub.value = newSubName.value.trim()
      showNewSub.value = false
      newSubName.value = ''
      step.value = 3
      // 新子项目自动加入列表（后端在首次导入时创建目录）
      if (!subProjects.value.includes(selectedSub.value)) {
        subProjects.value.push(selectedSub.value)
      }
    }

    async function doIngest() {
      if (!path.value.trim()) return
      loading.value = true; error.value = ''; result.value = null
      try {
        result.value = await api.ingest(path.value.trim(), selectedProject.value, selectedSub.value)
        emit('done')
      } catch (e) { error.value = '请求失败: ' + e.message }
      loading.value = false
    }

    function truncatedDesc(desc) {
      if (!desc) return ''
      return desc.length > 60 ? desc.slice(0, 60) + '...' : desc
    }

    return { step, projects, selectedProject, selectedSub, path, loading, result, error, loaded,
             showNewProj, newProjName, newProjDesc, creating, createMsg, createOk,
             subProjects, loadedSubs, showNewSub, newSubName,
             selectProject, createProj, createSub, doIngest, truncatedDesc }
  },
}
</script>

<style scoped>
/* 项目格块 */
.project-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 14px; margin-bottom: 24px; }
.project-card { background: #fff; border: 1px solid var(--color-border); border-radius: var(--radius); padding: 16px; cursor: pointer; transition: all 0.15s; }
.project-card:hover { border-color: var(--color-accent); box-shadow: 0 2px 12px rgba(79,108,247,0.08); transform: translateY(-1px); }
.card-header { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; }
.card-icon { font-size: 20px; }
.card-name { font-size: 15px; font-weight: 700; flex: 1; }
.card-badge { font-size: 10px; padding: 1px 6px; background: var(--color-accent-light); color: var(--color-accent); border-radius: 8px; }
.card-desc { font-size: 12px; color: var(--color-text-muted); line-height: 1.4; margin-bottom: 6px; }
.card-sub-info { font-size: 11px; color: var(--color-text-secondary); }

/* 创建卡片 */
.card-new { border: 2px dashed var(--color-border); background: #fafbfc; }
.card-new:hover { border-color: var(--color-accent); background: var(--color-accent-light); }
.card-new-center { display: flex; flex-direction: column; align-items: center; justify-content: center; min-height: 100px; gap: 6px; }
.new-icon { font-size: 28px; color: var(--color-accent); font-weight: 300; }
.new-text { font-size: 14px; color: var(--color-text-secondary); font-weight: 500; }
.empty-hint { grid-column: 1/-1; padding: 24px; text-align: center; color: var(--color-text-muted); }

/* 表单 */
.new-project-form, .new-sub-form { padding: 24px; max-width: 500px; margin-bottom: 16px; }
.new-project-form h3 { font-size: 16px; margin-bottom: 16px; }
.new-sub-form h4 { font-size: 15px; margin-bottom: 12px; }
.form-row { margin-bottom: 14px; }
.form-row label { display: block; font-size: 13px; font-weight: 600; color: var(--color-text-secondary); margin-bottom: 4px; }
.form-row input, .form-row textarea { width: 100%; font-family: inherit; }
.form-actions { display: flex; gap: 10px; align-items: center; }
.btn-link { background: none; border: none; color: var(--color-text-muted); cursor: pointer; font-size: 13px; font-family: inherit; }
.create-msg { margin-top: 12px; font-size: 13px; }
.create-msg.ok { color: #166534; }
.create-msg.err { color: #dc2626; }

/* 阶段二 */
.step-two { max-width: 700px; }
.btn-back { padding: 6px 14px; background: none; border: 1px solid var(--color-border); border-radius: 4px; font-size: 13px; color: var(--color-text-secondary); cursor: pointer; font-family: inherit; margin-bottom: 16px; }
.btn-back:hover { background: var(--color-hover); }
.project-label { margin-left: 10px; font-size: 15px; font-weight: 600; color: var(--color-text); }
.sub-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 12px; }
.sub-card { background: #fff; border: 1px solid var(--color-border); border-radius: var(--radius); padding: 18px; cursor: pointer; transition: all 0.15s; display: flex; align-items: center; gap: 12px; }
.sub-card:hover { border-color: var(--color-accent); box-shadow: 0 2px 8px rgba(79,108,247,0.06); transform: translateY(-1px); }
.sub-icon { font-size: 22px; }
.sub-name { font-size: 14px; font-weight: 600; color: var(--color-text); }

/* 阶段三 */
.step-three { max-width: 700px; }
.path-breadcrumb { margin-left: 10px; font-size: 13px; color: var(--color-accent); font-weight: 600; margin-bottom: 16px; display: inline-block; padding: 3px 10px; background: var(--color-accent-light); border-radius: 4px; }

.ingest-card { padding: 24px; margin-bottom: 20px; }
.form-label { display: block; font-size: 13px; font-weight: 600; color: var(--color-text-secondary); margin-bottom: 8px; }
.input-wrap { display: flex; border: 1px solid var(--color-border); border-radius: 6px; overflow: hidden; }
.input-wrap:focus-within { border-color: var(--color-accent); box-shadow: 0 0 0 3px rgba(79,108,247,0.08); }
.ingest-input { flex: 1; padding: 11px 14px; border: none; border-radius: 0; font-size: 14px; }
.ingest-input:focus { outline: none; box-shadow: none; }
.ingest-btn { border-radius: 0; padding: 11px 24px; flex-shrink: 0; }
.form-hint { margin-top: 6px; font-size: 12px; color: var(--color-text-muted); line-height: 1.5; }

.ingest-result { margin-top: 20px; }
.result-header { padding: 12px 16px; font-size: 14px; font-weight: 600; border-radius: 6px; }
.result-header.success { background: #f0fdf4; color: #166534; }
.result-header.has-errors { background: #fffbeb; color: #92400e; }
.validation-section { margin-bottom: 16px; padding: 14px 16px; background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 8px; }
.validation-title { font-size: 13px; font-weight: 600; color: #475569; margin-bottom: 10px; }
.validation-errors { margin-bottom: 8px; }
.v-item { font-size: 12px; padding: 4px 0; line-height: 1.5; }
.v-err { color: #dc2626; }
.v-warn { color: #d97706; }
.error-list { margin-top: 10px; padding: 12px 16px; background: #fef2f2; border-radius: 6px; max-height: 200px; overflow-y: auto; }
.error-item { font-size: 12.5px; color: #991b1b; padding: 4px 0; border-bottom: 1px solid #fecaca; word-break: break-all; }
.error-banner { margin-top: 16px; padding: 12px 16px; background: #fef2f2; border: 1px solid #fecaca; border-radius: 6px; color: #991b1b; font-size: 14px; }
</style>
