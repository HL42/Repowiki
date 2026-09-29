<template>
  <div class="lint-panel">
    <div class="lint-header">
      <div>
        <h1 class="page-title">质量检查</h1>
        <p class="page-desc">扫描知识库中的所有页面，发现空壳页面、LLM 幻觉产物、低质量内容等问题。</p>
      </div>
      <button class="btn-primary" @click="runLint" :disabled="scanning">
        {{ scanning ? '扫描中...' : '开始检查' }}
      </button>
    </div>

    <!-- 概览 -->
    <div v-if="report" class="lint-summary">
      <div class="summary-card total">
        <div class="summary-num">{{ report.total_pages }}</div>
        <div class="summary-label">总页面</div>
      </div>
      <div class="summary-card error">
        <div class="summary-num">{{ report.error_count }}</div>
        <div class="summary-label">严重问题</div>
      </div>
      <div class="summary-card warn">
        <div class="summary-num">{{ report.warn_count }}</div>
        <div class="summary-label">警告</div>
      </div>
      <div class="summary-card info">
        <div class="summary-num">{{ report.info_count }}</div>
        <div class="summary-label">提示</div>
      </div>
      <div class="summary-card clean">
        <div class="summary-num">{{ report.clean_count }}</div>
        <div class="summary-label">正常</div>
      </div>
    </div>

    <!-- 筛选栏 -->
    <div v-if="report && report.issues.length > 0" class="lint-filters">
      <button :class="{ active: filter === 'all' }" @click="filter = 'all'">全部 ({{ report.issues.length }})</button>
      <button :class="{ active: filter === 'error' }" @click="filter = 'error'">严重 ({{ report.error_count }})</button>
      <button :class="{ active: filter === 'warning' }" @click="filter = 'warning'">警告 ({{ report.warn_count }})</button>
      <button :class="{ active: filter === 'info' }" @click="filter = 'info'">提示 ({{ report.info_count }})</button>
      <div class="filter-right">
        <button v-if="filteredIssues.length > 0" class="btn-warn-sm" @click="selectAllVisible">全选</button>
        <span v-if="selectedIssues.length > 0" class="batch-info">
          已选 {{ selectedIssues.length }} 项
          <button class="btn-danger-sm" @click="batchDelete">批量删除</button>
          <button class="btn-link-sm" @click="selectedIssues = []">取消选择</button>
        </span>
        <button v-if="filteredIssues.length > 0 && filter !== 'all'" class="btn-danger-sm" @click="batchDeleteAllVisible" style="margin-left:8px">一键删除全部</button>
      </div>
    </div>

    <!-- 问题列表 -->
    <div v-if="report && filteredIssues.length > 0" class="issue-list">
      <div
        v-for="(issue, idx) in filteredIssues"
        :key="idx"
        :class="['issue-item', issue.severity]"
      >
        <div class="issue-select">
          <input
            type="checkbox"
            :checked="selectedIssues.includes(idx)"
            @change="toggleSelect(idx)"
          />
        </div>
        <div class="issue-body">
          <div class="issue-location">
            <span class="issue-cat">{{ issue.category }}</span>
            <span class="issue-name">{{ issue.name }}</span>
            <span class="issue-size">{{ (issue.size / 1024).toFixed(2) }} KB</span>
          </div>
          <div class="issue-desc">{{ issue.issue }}</div>
        </div>
        <div class="issue-actions">
          <span class="severity-tag" :class="issue.severity">
            {{ issue.severity === 'error' ? '严重' : issue.severity === 'warning' ? '警告' : '提示' }}
          </span>
          <button class="btn-delete-sm" @click="deleteOne(issue, idx)" :disabled="deleting === idx">
            {{ deleting === idx ? '删除中' : '删除' }}
          </button>
        </div>
      </div>
    </div>

    <!-- 无问题 -->
    <div v-if="report && report.issues.length === 0" class="clean-banner">
      知识库质量良好，未发现问题。
    </div>

    <div v-if="!report && !scanning" class="empty-hint">
      点击「开始检查」扫描知识库质量
    </div>

    <!-- 操作反馈 -->
    <div v-if="feedback" class="feedback" :class="feedbackType">{{ feedback }}</div>
  </div>
</template>

<script>
import { ref, computed } from 'vue'
import api from '../api.js'

export default {
  props: {
    currentProject: { type: String, default: '' }
  },
  emits: ['done'],
  setup(props, { emit }) {
    const report = ref(null)
    const scanning = ref(false)
    const filter = ref('all')
    const selectedIssues = ref([])
    const deleting = ref(-1)
    const feedback = ref('')
    const feedbackType = ref('')

    const filteredIssues = computed(() => {
      if (!report.value) return []
      if (filter.value === 'all') return report.value.issues
      return report.value.issues.filter(i => i.severity === filter.value)
    })

    async function runLint() {
      scanning.value = true
      report.value = null
      selectedIssues.value = []
      feedback.value = ''
      try {
        report.value = await api.lint()
      } catch (e) {
        feedback.value = '扫描失败: ' + e.message
        feedbackType.value = 'error'
      }
      scanning.value = false
    }

    function toggleSelect(idx) {
      const pos = selectedIssues.value.indexOf(idx)
      if (pos >= 0) {
        selectedIssues.value.splice(pos, 1)
      } else {
        selectedIssues.value.push(idx)
      }
    }

    async function deleteOne(issue, idx) {
      if (!confirm(`确认删除 ${issue.category}/${issue.name}？`)) return
      deleting.value = idx
      try {
        // 优先使用 issue 自带的项目信息（lint 报告精确标记了来源）
        const project = issue.project || props.currentProject
        const subProject = issue.sub_project || ''
        const resp = await api.deletePage(issue.category, issue.name, project, subProject)
        if (resp.deleted) {
          showFeedback(`已删除 ${issue.name}`, 'success')
          // 从报告中移除
          const globalIdx = report.value.issues.indexOf(issue)
          if (globalIdx >= 0) {
            report.value.issues.splice(globalIdx, 1)
            // 更新统计
            if (issue.severity === 'error') report.value.error_count--
            if (issue.severity === 'warning') report.value.warn_count--
            if (issue.severity === 'info') report.value.info_count--
            report.value.total_pages--
          }
          emit('done')
        } else if (resp.error) {
          showFeedback(resp.error, 'error')
        }
      } catch (e) {
        showFeedback('删除失败: ' + e.message, 'error')
      }
      deleting.value = -1
    }

    async function batchDelete() {
      if (!confirm(`确认批量删除 ${selectedIssues.value.length} 个页面？此操作不可撤销。`)) return
      const toDelete = [...selectedIssues.value].sort((a, b) => b - a)
      for (const idx of toDelete) {
        const issue = report.value.issues[idx]
        if (!issue) continue
        try {
          const project = issue.project || props.currentProject
          const subProject = issue.sub_project || ''
          const resp = await api.deletePage(issue.category, issue.name, project, subProject)
          if (resp.deleted) {
            report.value.issues.splice(idx, 1)
            if (issue.severity === 'error') report.value.error_count--
            if (issue.severity === 'warning') report.value.warn_count--
            if (issue.severity === 'info') report.value.info_count--
            report.value.total_pages--
          }
        } catch (e) { /* 继续 */ }
      }
      selectedIssues.value = []
      showFeedback(`批量删除完成`, 'success')
      emit('done')
    }

    function selectAllVisible() {
      if (!report.value) return
      const visibleIdx = report.value.issues
        .map((issue, idx) => (filter.value === 'all' || issue.severity === filter.value) ? idx : -1)
        .filter(idx => idx >= 0)
      selectedIssues.value = visibleIdx
    }

    async function batchDeleteAllVisible() {
      if (!report.value) return
      const targetSev = filter.value
      if (targetSev === 'all') return

      const visibleCount = report.value.issues.filter(i => i.severity === targetSev).length
      if (visibleCount === 0) return

      const sevLabel = targetSev === 'error' ? '严重' : targetSev === 'warning' ? '警告' : '提示'
      if (!confirm(`确认删除全部 ${visibleCount} 个${sevLabel}问题页面？此操作不可撤销。`)) return

      const toDelete = report.value.issues
        .map((issue, idx) => issue.severity === targetSev ? idx : -1)
        .filter(idx => idx >= 0)
        .sort((a, b) => b - a)

      for (const idx of toDelete) {
        const issue = report.value.issues[idx]
        if (!issue || issue.severity !== targetSev) continue
        try {
          const project = issue.project || props.currentProject
          const subProject = issue.sub_project || ''
          const resp = await api.deletePage(issue.category, issue.name, project, subProject)
          if (resp.deleted) {
            report.value.issues.splice(idx, 1)
            if (targetSev === 'error') report.value.error_count--
            if (targetSev === 'warning') report.value.warn_count--
            if (targetSev === 'info') report.value.info_count--
            report.value.total_pages--
            if (report.value.clean_count !== undefined) report.value.clean_count++
          }
        } catch (e) { /* 继续删除下一个 */ }
      }
      selectedIssues.value = []
      showFeedback(`已删除全部${sevLabel}问题`, 'success')
      emit('done')
    }

    function showFeedback(msg, type) {
      feedback.value = msg
      feedbackType.value = type
      setTimeout(() => { feedback.value = '' }, 3000)
    }

    return { report, scanning, filter, filteredIssues, selectedIssues, deleting, feedback, feedbackType, runLint, toggleSelect, deleteOne, batchDelete, selectAllVisible, batchDeleteAllVisible }
  },
}
</script>

<style scoped>
.lint-panel { max-width: 900px; }

.lint-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 20px;
}

.lint-header .page-desc { margin-bottom: 0; }

/* Summary */
.lint-summary {
  display: flex;
  gap: 10px;
  margin-bottom: 20px;
}

.summary-card {
  flex: 1;
  text-align: center;
  padding: 18px 12px;
  border-radius: var(--radius);
  background: #fafbfc;
  border: 1px solid var(--color-border);
}

.summary-num {
  font-size: 28px;
  font-weight: 700;
  line-height: 1.1;
  margin-bottom: 4px;
}

.summary-label {
  font-size: 12px;
  color: var(--color-text-muted);
}

.summary-card.total .summary-num { color: var(--color-text); }
.summary-card.error .summary-num { color: #dc2626; }
.summary-card.warn .summary-num { color: #d97706; }
.summary-card.info .summary-num { color: var(--color-accent); }
.summary-card.clean .summary-num { color: #16a34a; }

/* Filters */
.lint-filters {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 14px;
}

.lint-filters button {
  padding: 5px 14px;
  border: 1px solid var(--color-border);
  background: #fff;
  border-radius: 4px;
  font-size: 13px;
  cursor: pointer;
  color: var(--color-text-secondary);
  font-family: inherit;
  transition: all 0.1s;
}

.lint-filters button:hover { background: var(--color-hover); }

.lint-filters button.active {
  background: var(--color-accent);
  color: #fff;
  border-color: var(--color-accent);
}

.filter-right { margin-left: auto; }

.batch-info {
  font-size: 13px;
  color: var(--color-text-secondary);
}

.btn-danger-sm {
  margin-left: 10px;
  padding: 4px 12px;
  background: #dc2626;
  color: #fff;
  border: none;
  border-radius: 3px;
  font-size: 12px;
  cursor: pointer;
  font-family: inherit;
}

.btn-danger-sm:hover { background: #b91c1c; }

.btn-warn-sm {
  padding: 4px 12px;
  background: #fef3c7;
  color: #d97706;
  border: 1px solid #fcd34d;
  border-radius: 3px;
  font-size: 12px;
  cursor: pointer;
  font-family: inherit;
  transition: all 0.1s;
}

.btn-warn-sm:hover { background: #fcd34d; }

.btn-link-sm {
  margin-left: 6px;
  padding: 4px 8px;
  background: none;
  border: none;
  color: var(--color-text-muted);
  font-size: 12px;
  cursor: pointer;
  font-family: inherit;
}

/* Issue list */
.issue-list {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.issue-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  background: #fff;
  border: 1px solid var(--color-border);
  border-radius: 4px;
}

.issue-item.error { border-left: 3px solid #dc2626; }
.issue-item.warning { border-left: 3px solid #d97706; }
.issue-item.info { border-left: 3px solid var(--color-accent); }

.issue-select { flex-shrink: 0; }

.issue-body { flex: 1; min-width: 0; }

.issue-location {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 3px;
}

.issue-cat {
  font-size: 11px;
  font-weight: 600;
  padding: 1px 6px;
  background: var(--color-hover);
  border-radius: 3px;
  color: var(--color-text-muted);
}

.issue-name {
  font-size: 13.5px;
  font-weight: 600;
  color: var(--color-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.issue-size {
  font-size: 11px;
  color: var(--color-text-muted);
}

.issue-desc {
  font-size: 12.5px;
  color: var(--color-text-secondary);
}

.issue-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.severity-tag {
  padding: 2px 8px;
  border-radius: 3px;
  font-size: 11px;
  font-weight: 600;
}

.severity-tag.error { background: #fef2f2; color: #dc2626; }
.severity-tag.warning { background: #fffbeb; color: #d97706; }
.severity-tag.info { background: var(--color-accent-light); color: var(--color-accent); }

.btn-delete-sm {
  padding: 3px 10px;
  background: #fef2f2;
  color: #dc2626;
  border: 1px solid #fecaca;
  border-radius: 3px;
  font-size: 12px;
  cursor: pointer;
  font-family: inherit;
  transition: all 0.1s;
}

.btn-delete-sm:hover { background: #dc2626; color: #fff; }
.btn-delete-sm:disabled { opacity: 0.5; cursor: not-allowed; }

/* Clean / Empty */
.clean-banner {
  padding: 24px;
  background: #f0fdf4;
  border: 1px solid #bbf7d0;
  border-radius: var(--radius);
  color: #166534;
  font-size: 15px;
  text-align: center;
}

.empty-hint {
  padding: 40px;
  text-align: center;
  color: var(--color-text-muted);
  font-size: 14px;
}

.feedback {
  position: fixed;
  bottom: 24px;
  right: 24px;
  padding: 12px 20px;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 500;
  z-index: 200;
}

.feedback.success { background: #166534; color: #fff; }
.feedback.error { background: #dc2626; color: #fff; }
</style>
