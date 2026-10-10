package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// BrowseEntry 目录浏览中的单个条目
type BrowseEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// BrowseDirResponse 目录浏览响应
type BrowseDirResponse struct {
	Path      string        `json:"path"`
	Parent    string        `json:"parent"`
	Dirs      []BrowseEntry `json:"dirs"`
	Files     []BrowseEntry `json:"files"`
	IsHome    bool          `json:"is_home"`
	Truncated bool          `json:"truncated"`
}

// 单次返回的条目上限，避免选中 node_modules 这类超大目录时返回巨量数据
const (
	maxBrowseDirs  = 500
	maxBrowseFiles = 200
)

// handleBrowseDir 列出本机某目录的子目录与文件，供「导入文档」的文件夹选择器使用。
//
// 注意：本接口会列出本机磁盘内容，安全性依赖服务仅监听 127.0.0.1
// （见 cmd/main.go 中的 host 常量）。若将来改为对外提供服务，必须先加鉴权。
func (s *Server) handleBrowseDir(w http.ResponseWriter, r *http.Request) {
	reqPath := strings.TrimSpace(r.URL.Query().Get("path"))

	home, homeErr := os.UserHomeDir()

	// 未指定路径时从用户主目录开始，省去用户手输路径
	if reqPath == "" {
		if homeErr != nil {
			writeJSONError(w, http.StatusInternalServerError, "无法确定用户主目录")
			return
		}
		reqPath = home
	}

	abs, err := filepath.Abs(reqPath)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "路径无效")
		return
	}

	info, err := os.Stat(abs)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "路径不存在或不可读")
		return
	}
	if !info.IsDir() {
		writeJSONError(w, http.StatusBadRequest, "该路径不是目录")
		return
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		writeJSONError(w, http.StatusForbidden, "无权限读取该目录")
		return
	}

	// os.ReadDir 已按文件名排序
	resp := BrowseDirResponse{
		Path:   abs,
		Dirs:   []BrowseEntry{},
		Files:  []BrowseEntry{},
		IsHome: homeErr == nil && abs == home,
	}
	// 已到文件系统根目录时不再提供「上一级」
	if parent := filepath.Dir(abs); parent != abs {
		resp.Parent = parent
	}

	for _, e := range entries {
		name := e.Name()
		// 跳过隐藏项，避免列表被 .git / .DS_Store 等淹没
		if strings.HasPrefix(name, ".") {
			continue
		}
		entry := BrowseEntry{Name: name, Path: filepath.Join(abs, name)}
		if isDirEntry(entry.Path, e) {
			if len(resp.Dirs) >= maxBrowseDirs {
				resp.Truncated = true
				continue
			}
			resp.Dirs = append(resp.Dirs, entry)
			continue
		}
		if len(resp.Files) >= maxBrowseFiles {
			resp.Truncated = true
			continue
		}
		resp.Files = append(resp.Files, entry)
	}

	writeJSON(w, resp)
}

// isDirEntry 判断条目是否为可进入的目录。
//
// DirEntry.IsDir() 对符号链接返回 false，而 macOS 上 iCloud / OneDrive
// 这类常用目录恰恰是链接；不跟随一次就会被列成文件，用户点不进去。
func isDirEntry(path string, e os.DirEntry) bool {
	if e.IsDir() {
		return true
	}
	if e.Type()&os.ModeSymlink == 0 {
		return false
	}
	target, err := os.Stat(path)
	return err == nil && target.IsDir()
}
