async function request(url, body) {
  const opts = {
    headers: { 'Content-Type': 'application/json' },
  }
  if (body) {
    opts.method = 'POST'
    opts.body = JSON.stringify(body)
  }
  const resp = await fetch(url, opts)
  const data = await resp.json().catch(() => ({}))
  if (!resp.ok) {
    throw new Error(data.error || `HTTP ${resp.status}`)
  }
  return data
}

export default {
  query(question, project, subProject, history) {
    const body = { question }
    if (project) body.project = project
    if (subProject) body.sub_project = subProject
    if (history && history.length) body.history = history
    return request('/api/query', body)
  },
  synthesize(topic, project, subProject) {
    const body = { topic }
    if (project) body.project = project
    if (subProject) body.sub_project = subProject
    return request('/api/synthesize', body)
  },
  ingest(path, project, subProject) {
    const body = { raw_path: path }
    if (project) body.project = project
    if (subProject) body.sub_project = subProject
    return request('/api/ingest', body)
  },
  validate(path, project, subProject) {
    const body = { raw_path: path }
    if (project) body.project = project
    if (subProject) body.sub_project = subProject
    return request('/api/ingest/validate', body)
  },
  stats(project) {
    const url = project ? `/api/stats?project=${encodeURIComponent(project)}` : '/api/stats'
    return request(url)
  },
  async lint() {
    const resp = await fetch('/api/lint')
    const data = await resp.json().catch(() => ({}))
    if (!resp.ok) {
      throw new Error(data.error || `HTTP ${resp.status}`)
    }
    return data
  },
  async deletePage(category, name, project, subProject) {
    let url = `/api/wiki/page/${category}/${name}`
    const params = []
    if (project) params.push(`project=${encodeURIComponent(project)}`)
    if (subProject) params.push(`sub_project=${encodeURIComponent(subProject)}`)
    if (params.length > 0) url += '?' + params.join('&')
    
    const resp = await fetch(url, { method: 'DELETE' })
    const data = await resp.json().catch(() => ({}))
    if (!resp.ok) {
      throw new Error(data.error || `HTTP ${resp.status}`)
    }
    return data
  },
  // 项目管理
  async listProjects() {
    const resp = await fetch('/api/projects')
    const data = await resp.json().catch(() => ({}))
    if (!resp.ok) throw new Error(data.error || `HTTP ${resp.status}`)
    return data.projects || []
  },
  async createProject(name, description, subProjects) {
    return request('/api/projects', { name, description, sub_projects: subProjects })
  },
  async getProject(name) {
    const resp = await fetch(`/api/project/${encodeURIComponent(name)}`)
    const data = await resp.json().catch(() => ({}))
    if (!resp.ok) throw new Error(data.error || `HTTP ${resp.status}`)
    return data
  },
  async listSubProjects(project) {
    const resp = await fetch(`/api/sub-projects?project=${encodeURIComponent(project)}`)
    const data = await resp.json().catch(() => ({}))
    if (!resp.ok) throw new Error(data.error || `HTTP ${resp.status}`)
    return data.sub_projects || []
  },
}
