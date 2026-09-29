import { marked } from 'marked'
import hljs from 'highlight.js'
import 'highlight.js/styles/github.css'

// marked v18 使用 use() 扩展 API 接入代码高亮
// 注意：v18 的 code 渲染器接收 token 对象 {text, lang}，而非分开的参数
marked.use({
  renderer: {
    code(token) {
      const code = token.text || ''
      const lang = token.lang || ''
      if (lang && hljs.getLanguage(lang)) {
        try {
          return `<pre><code class="hljs language-${lang}">${hljs.highlight(code, { language: lang }).value}</code></pre>\n`
        } catch (_) { /* fall through */ }
      }
      try {
        return `<pre><code class="hljs">${hljs.highlightAuto(code).value}</code></pre>\n`
      } catch (_) {
        return `<pre><code>${code}</code></pre>\n`
      }
    },
  },
  breaks: false,
  gfm: true,
})

/**
 * 解析 Markdown 文本，自动处理 YAML frontmatter。
 * 返回 { meta: {...}, html: "..." }，meta 为 frontmatter 键值对，没有则为 null。
 */
export function parseMarkdown(text) {
  if (!text) return { meta: null, html: '' }

  let body = text
  let meta = null

  // 提取 YAML frontmatter (--- ... ---)
  const fmMatch = text.match(/^---\s*\n([\s\S]*?)\n---\s*\n/)
  if (fmMatch) {
    meta = parseFrontmatter(fmMatch[1])
    body = text.slice(fmMatch[0].length)
  }

  // 渲染正文
  const html = marked.parse(body)

  return { meta, html }
}

/**
 * 仅渲染 Markdown，不处理 frontmatter。用于问答回答等场景。
 */
export function renderMarkdown(text) {
  if (!text) return ''
  // 跳过可能的 frontmatter
  let body = text
  if (/^---\s*\n/.test(body)) {
    const end = body.indexOf('\n---', 4)
    if (end !== -1) {
      body = body.slice(end + 4)
    }
  }
  return marked.parse(body)
}

/**
 * 解析 YAML frontmatter 为键值对对象。
 * 支持简单字符串值、字符串数组（- item）。
 */
function parseFrontmatter(fm) {
  const result = {}
  const lines = fm.split('\n')
  let currentKey = null
  let currentArray = null
  let inArray = false

  for (const line of lines) {
    const trimmed = line.trim()
    if (!trimmed || trimmed.startsWith('#')) continue

    // 数组项：  - "value" 或  - value
    const arrMatch = line.match(/^(\s*)-\s+["']?(.+?)["']?\s*$/)
    if (arrMatch && inArray && currentKey) {
      const val = arrMatch[2].trim()
      if (val) currentArray.push(val)
      continue
    }

    // 键值对：key: "value" 或 key: value 或 key:
    const kvMatch = line.match(/^(\w[\w_-]*):\s*(.*)$/)
    if (kvMatch) {
      // 保存之前的数组
      if (currentKey && currentArray !== null) {
        result[currentKey] = currentArray
      }

      currentKey = kvMatch[1]
      const val = kvMatch[2].trim()

      // 判断是否为数组开始（值为空，下一行是 - 开头）
      if (val === '' || val === '[]') {
        currentArray = []
        inArray = true
        result[currentKey] = currentArray
      } else if (val.startsWith('[') && val.endsWith(']')) {
        // 单行数组格式：[a, b, c]
        const items = val.slice(1, -1).split(',').map(s => s.trim().replace(/^["']|["']$/g, '')).filter(Boolean)
        currentArray = items
        inArray = false
        result[currentKey] = currentArray
      } else {
        // 普通字符串值
        currentArray = null
        inArray = false
        result[currentKey] = val.replace(/^["']|["']$/g, '')
      }
      continue
    }
  }

  // 保存最后的数组
  if (currentKey && currentArray !== null && !(currentKey in result)) {
    result[currentKey] = currentArray
  }

  return result
}

export default { parseMarkdown, renderMarkdown }
