<template>
  <div class="project-detail">
    <div class="back-bar">
      <button class="btn-back" @click="$emit('back')">← 返回首页</button>
    </div>

    <div v-if="loading" class="loading-state">加载中...</div>

    <div v-else-if="error" class="error-state">{{ error }}</div>

    <div v-else-if="project" class="project-body">
      <div class="project-hero">
        <h1>📦 {{ project.name }}</h1>
        <p class="hero-desc">{{ project.description }}</p>
        <div class="hero-stats">
          <div class="stat-block">
            <span class="stat-num">{{ project.total_pages }}</span>
            <span class="stat-label">总页面</span>
          </div>
          <div class="stat-block">
            <span class="stat-num">{{ project.source_pages }}</span>
            <span class="stat-label">源文档</span>
          </div>
          <div class="stat-block" v-if="project.entity_pages > 0">
            <span class="stat-num">{{ project.entity_pages }}</span>
            <span class="stat-label">实体</span>
          </div>
          <div class="stat-block" v-if="project.concept_pages > 0">
            <span class="stat-num">{{ project.concept_pages }}</span>
            <span class="stat-label">概念</span>
          </div>
        </div>
      </div>

      <div class="project-content" v-if="project.sub_projects">
        <h2>子项目结构</h2>
        <div class="sub-content" v-html="renderedSub"></div>
      </div>

      <!-- 问答区域 -->
      <div class="project-qa">
        <h2>项目内问答</h2>
        <div class="qa-input-row">
          <input
            v-model="question"
            class="form-input qa-input"
            placeholder="在此项目的文档内搜索提问..."
            @keydown.enter="askProject"
          />
          <button class="btn-primary" @click="askProject" :disabled="asking || !question.trim()">
            {{ asking ? '查询中...' : '提问' }}
          </button>
        </div>
        <div v-if="answer" class="qa-answer">
          <div class="markdown-body" v-html="renderedAnswer"></div>
          <div v-if="answerRefs.length" class="qa-refs">
            <strong>引用来源：</strong>
            <span v-for="(ref, i) in answerRefs" :key="i">{{ ref }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, onMounted, computed } from 'vue'
import api from '../api.js'
import { renderMarkdown } from './MarkdownRenderer.js'

export default {
  props: ['projectName'],
  emits: ['back'],
  setup(props) {
    const project = ref(null)
    const loading = ref(true)
    const error = ref('')
    const question = ref('')
    const answer = ref('')
    const answerRefs = ref([])
    const asking = ref(false)

    const renderedSub = computed(() => {
      if (!project.value?.sub_projects) return ''
      return renderMarkdown(project.value.sub_projects)
    })

    const renderedAnswer = computed(() => {
      if (!answer.value) return ''
      return renderMarkdown(answer.value)
    })

    onMounted(async () => {
      try {
        project.value = await api.getProject(props.projectName)
      } catch (e) {
        error.value = e.message
      }
      loading.value = false
    })

    async function askProject() {
      if (!question.value.trim() || asking.value) return
      asking.value = true
      answer.value = ''
      answerRefs.value = []
      try {
        const resp = await api.query(question.value.trim(), props.projectName)
        answer.value = resp.answer
        answerRefs.value = resp.source_pages || []
      } catch (e) {
        answer.value = '查询失败: ' + e.message
      }
      asking.value = false
    }

    return { project, loading, error, question, answer, answerRefs, asking, renderedSub, renderedAnswer, askProject }
  },
}
</script>

<style scoped>
.back-bar { margin-bottom: 20px; }
.btn-back {
  padding: 6px 14px;
  background: none;
  border: 1px solid var(--color-border);
  border-radius: 6px;
  font-size: 13px;
  color: var(--color-text-secondary);
  cursor: pointer;
  font-family: inherit;
}
.btn-back:hover { background: var(--color-hover); }

.project-hero {
  background: linear-gradient(135deg, var(--color-accent-light), #f8f9ff);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  padding: 28px;
  margin-bottom: 24px;
}

.project-hero h1 { font-size: 24px; margin-bottom: 8px; }

.hero-desc {
  font-size: 14px;
  color: var(--color-text-secondary);
  line-height: 1.6;
  margin-bottom: 18px;
}

.hero-stats { display: flex; gap: 24px; }

.stat-block { text-align: center; }

.stat-num {
  display: block;
  font-size: 26px;
  font-weight: 700;
  color: var(--color-accent);
  line-height: 1;
  margin-bottom: 2px;
}

.stat-label { font-size: 12px; color: var(--color-text-muted); }

.project-content {
  background: #fff;
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  padding: 20px 24px;
  margin-bottom: 24px;
}

.project-content h2 {
  font-size: 17px;
  margin-bottom: 12px;
}

.sub-content { font-size: 14px; color: var(--color-text-secondary); }

.sub-content :deep(ul) { padding-left: 20px; }
.sub-content :deep(li) { margin-bottom: 4px; }

.project-qa {
  background: #fff;
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  padding: 20px 24px;
}

.project-qa h2 { font-size: 17px; margin-bottom: 14px; }

.qa-input-row {
  display: flex;
  gap: 10px;
  margin-bottom: 16px;
}

.qa-input { flex: 1; }

.qa-answer {
  background: #fafbfc;
  border-radius: 6px;
  padding: 16px 20px;
}

.qa-refs {
  margin-top: 12px;
  padding-top: 10px;
  border-top: 1px solid var(--color-border);
  font-size: 12px;
  color: var(--color-text-muted);
}

.qa-refs span {
  display: inline-block;
  margin-left: 6px;
  padding: 1px 6px;
  background: var(--color-accent-light);
  border-radius: 3px;
  font-size: 11px;
}

.loading-state, .error-state {
  padding: 40px;
  text-align: center;
  color: var(--color-text-muted);
}

.error-state { color: #dc2626; }
</style>
