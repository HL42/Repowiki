package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileSystem 文件系统存储
type FileSystem struct {
	Root string
}

// Sub 返回项目级子存储实例（Root 指向 projects/{project}/）
func (fs *FileSystem) Sub(project string) (*FileSystem, error) {
	f, err := NewFileSystem(filepath.Join(fs.Root, "projects", project))
	if err != nil {
		return nil, fmt.Errorf("Sub(%q): %w", project, err)
	}
	return f, nil
}

// SubProject 返回子项目存储实例（Root 指向 {currentRoot}/sub-projects/{name}/）
// 用于同一大项目下隔离不同子项目的 raw/wiki 数据
func (fs *FileSystem) SubProject(name string) (*FileSystem, error) {
	f, err := NewFileSystem(filepath.Join(fs.Root, "sub-projects", name))
	if err != nil {
		return nil, fmt.Errorf("SubProject(%q): %w", name, err)
	}
	return f, nil
}

// ListSubProjects 列出当前项目下的所有子项目
func (fs *FileSystem) ListSubProjects() ([]string, error) {
	subDir := filepath.Join(fs.Root, "sub-projects")
	entries, err := os.ReadDir(subDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// ListProjects 列出所有已创建的项目
func (fs *FileSystem) ListProjects() ([]string, error) {
	projectsDir := filepath.Join(fs.Root, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

func NewFileSystem(root string) (*FileSystem, error) {
	dirs := []string{
		"raw",
		"wiki/sources",
		"wiki/entities",
		"wiki/concepts",
		"wiki/syntheses",
		"wiki/contradictions",
		"wiki/history/raw",
		"wiki/history/sources",
	}
	for _, dir := range dirs {

		path := filepath.Join(root, dir)
		if err := os.MkdirAll(path, 0755); err != nil {

			return nil, fmt.Errorf("创建目录失败 %s: %w", path, err)
		}
	}
	return &FileSystem{Root: root}, nil
}

// InitWiki 初始化 index.md 和 log.md（仅首次）
func (fs *FileSystem) InitWiki() error {
	indexPath := filepath.Join(fs.Root, "wiki", "index.md")
	if !fileExists(indexPath) {

		content := "# Wiki 索引导航\n\n" +
			"> 本文件由 RepoWiki 自动维护。\n\n" +
			"## 源文档摘要\n\n（导入文档后自动生成）\n\n" +
			"## 实体索引\n\n（识别实体后自动生成）\n\n" +
			"## 概念索引\n\n（识别概念后自动生成）\n\n" +
			"## 综合提炼\n\n（跨文档综合分析后自动生成）\n\n" +
			"## 矛盾记录\n\n（检测到信息冲突时自动生成）\n"
		if err := os.WriteFile(indexPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("创建 index.md 失败: %w", err)
		}
	}

	logPath := filepath.Join(fs.Root, "wiki", "log.md")
	if !fileExists(logPath) {

		content := "# 知识库演化日志\n\n记录每次导入、更新、综合操作。\n\n"
		if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("创建 log.md 失败: %w", err)
		}
	}

	return nil
}

// ArchiveRaw 将原始文档拷贝到 knowledge-base/raw/ 归档
func (fs *FileSystem) ArchiveRaw(srcPath string) (string, error) {
	return fs.ArchiveRawAs(srcPath, sanitizeFilename(filepath.Base(srcPath)))
}

// ArchiveRawAs 按指定名称归档，避免 index.md 等同名文件互相覆盖
func (fs *FileSystem) ArchiveRawAs(srcPath, destName string) (string, error) {
	destDir := filepath.Join(fs.Root, "raw")
	destPath := filepath.Join(destDir, sanitizeFilename(destName))

	content, err := os.ReadFile(srcPath)
	if err != nil {

		return "", fmt.Errorf("读取源文件失败: %w", err)
	}
	if err := os.WriteFile(destPath, content, 0644); err != nil {

		return "", fmt.Errorf("归档文件失败: %w", err)
	}
	return destPath, nil
}

// WritePage 写入 Wiki 页面（自动建目录）
func (fs *FileSystem) WritePage(subdir, filename, content string) error {
	fullPath := filepath.Join(fs.Root, "wiki", subdir, sanitizeFilename(filename)+".md")
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {

		return err
	}
	return os.WriteFile(fullPath, []byte(content), 0644)
}

// ReadPage 读取 Wiki 页面
func (fs *FileSystem) ReadPage(subdir, filename string) (string, error) {
	fullPath := filepath.Join(fs.Root, "wiki", subdir, sanitizeFilename(filename)+".md")
	data, err := os.ReadFile(fullPath)
	if err != nil {

		return "", err
	}
	return string(data), nil
}

// AppendLog 追加演化日志
func (fs *FileSystem) AppendLog(entry string) error {
	logPath := filepath.Join(fs.Root, "wiki", "log.md")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {

		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "\n---\n\n%s\n", entry)
	return err
}

// ListPages 列出某个子目录下所有 .md 页面
func (fs *FileSystem) ListPages(subdir string) ([]string, error) {
	dir := filepath.Join(fs.Root, "wiki", subdir)
	var pages []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {

			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {

			pages = append(pages, path)
		}
		return nil
	})
	return pages, err
}

// ---------- 哈希管理 ----------

// SaveHash 保存文件内容的 SHA256 哈希到 .hash 文件
func (fs *FileSystem) SaveHash(filename, content string) error {
	path := filepath.Join(fs.Root, "raw", sanitizeFilename(filename)+".hash")
	return os.WriteFile(path, []byte(content), 0644)
}

// ReadHash 读取已存档的哈希值
func (fs *FileSystem) ReadHash(filename string) (string, error) {
	path := filepath.Join(fs.Root, "raw", sanitizeFilename(filename)+".hash")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// ComputeHash 计算内容的 SHA256
func ComputeHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// ---------- 历史快照 ----------

// SaveHistoryRaw 保存归档文件的旧版本到 wiki/history/raw/
func (fs *FileSystem) SaveHistoryRaw(archivedName, content string, timestamp string) error {
	base := sanitizeFilename(archivedName)
	histName := base + "_" + timestamp + ".md"
	path := filepath.Join(fs.Root, "wiki", "history", "raw", histName)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// ReadArchivedRaw 读取已归档的原始文件
func (fs *FileSystem) ReadArchivedRaw(archivedName string) (string, error) {
	path := filepath.Join(fs.Root, "raw", sanitizeFilename(archivedName))
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ---------- 辅助函数 ----------

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// sanitizeFilename 替换文件名中的不安全字符
func sanitizeFilename(name string) string {
	repl := strings.NewReplacer(
		"/", "-", "\\", "-", ":", "-",
		"*", "-", "?", "-", "\"", "",
		"<", "-", ">", "-", "|", "-",
		" ", "-",
	)
	name = strings.TrimSuffix(name, ".md")
	return repl.Replace(name)
}
