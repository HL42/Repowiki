// Package config 负责 RepoWiki 的配置加载、数据目录与打包资源定位。
//
// 安装包与源码两种运行形态的关键差异都收敛在这里：
//   - 打包后（.app / .exe）资源在可执行文件附近，数据在用户数据目录；
//   - 源码运行时资源在 CWD，数据默认也在 CWD（方便开发）。
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AppName 数据目录与 .app 包的显示名
const AppName = "RepoWiki"

// Settings 启动期确定的配置（APIKey/Model 运行期可被网页设置改写）
type Settings struct {
	APIKey   string
	Model    string
	Port     string
	WikiRoot string
}

// DataDir 返回用户数据目录（macOS: ~/Library/Application Support/RepoWiki，
// Windows: %APPDATA%\RepoWiki，Linux: ~/.local/share/repowiki）。
// 不创建目录；写 .env / knowledge-base 时由调用方 MkdirAll。
func DataDir() string {
	if d, err := os.UserConfigDir(); err == nil && d != "" {
		return filepath.Join(d, AppName)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".repowiki")
}

// ExeDir 返回可执行文件所在目录；失败时返回空串。
func ExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		exe, _ = os.Executable()
	}
	return filepath.Dir(exe)
}

// ResourceDir 返回「包含 frontend/ 资源目录」的根目录，搜索顺序：
//  1. 当前工作目录（源码：go run / 从仓库根启动）
//  2. 可执行文件所在目录（扁平 zip / Windows exe）
//  3. 可执行文件上一级的 Resources（macOS .app: Contents/Resources）
//
// 返回第一个存在 frontend/dist 的目录；都没有时回退到 CWD。
func ResourceDir() string {
	cwd, _ := os.Getwd()
	candidates := []string{cwd}
	if exeDir := ExeDir(); exeDir != "" {
		candidates = append(candidates,
			exeDir,
			filepath.Join(exeDir, "..", "Resources"),
		)
	}

	seen := map[string]bool{}
	for _, d := range candidates {
		if d == "" {
			continue
		}
		if abs, err := filepath.Abs(d); err == nil {
			d = abs
		}
		if seen[d] {
			continue
		}
		seen[d] = true
		if info, err := os.Stat(filepath.Join(d, "frontend", "dist")); err == nil && info.IsDir() {
			return d
		}
	}
	return cwd
}

// IsPackaged 资源是否位于可执行文件附近（即安装包形态），而非 CWD。
func IsPackaged() bool {
	cwd, _ := os.Getwd()
	rd := ResourceDir()
	return filepath.Clean(rd) != filepath.Clean(cwd)
}

// Load 按优先级读取配置：环境变量 > .env 文件（先 CWD 后数据目录）> 默认值。
func Load() Settings {
	dotEnv := readDotEnv()

	apiKey := getConfig("DEEPSEEK_API_KEY", dotEnv, "")
	model := getConfig("DEEPSEEK_MODEL", dotEnv, "deepseek-chat")
	port := getConfig("PORT", dotEnv, "8080")

	defaultWikiRoot := "./knowledge-base"
	if IsPackaged() {
		defaultWikiRoot = filepath.Join(DataDir(), "knowledge-base")
	}
	wikiRoot := getConfig("WIKI_ROOT", dotEnv, defaultWikiRoot)

	return Settings{APIKey: apiKey, Model: model, Port: port, WikiRoot: wikiRoot}
}

// EnvPaths 返回 .env 的读取候选路径（按优先级）。
func EnvPaths() []string {
	var paths []string
	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(cwd, ".env"))
	}
	if d := DataDir(); d != "" {
		paths = append(paths, filepath.Join(d, ".env"))
	}
	return paths
}

// EnvWritePath 返回 .env 的写入路径：打包 → 数据目录，源码 → CWD。
func EnvWritePath() string {
	if IsPackaged() {
		return filepath.Join(DataDir(), ".env")
	}
	if cwd, err := os.Getwd(); err == nil {
		return filepath.Join(cwd, ".env")
	}
	return filepath.Join(DataDir(), ".env")
}

// SaveEnv 把配置写回 .env（文件权限 0600，含密钥）。
func SaveEnv(s Settings) error {
	if s.Model == "" {
		s.Model = "deepseek-chat"
	}
	if s.Port == "" {
		s.Port = "8080"
	}
	if s.WikiRoot == "" {
		s.WikiRoot = "./knowledge-base"
	}

	path := EnvWritePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	content := fmt.Sprintf(
		"DEEPSEEK_API_KEY=%s\nDEEPSEEK_MODEL=%s\nPORT=%s\nWIKI_ROOT=%s\n",
		s.APIKey, s.Model, s.Port, s.WikiRoot,
	)
	return os.WriteFile(path, []byte(content), 0600)
}

// MaskKey 隐藏密钥中间部分，用于展示（如 sk-****abcd）。
func MaskKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return "已设置"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

func readDotEnv() string {
	for _, p := range EnvPaths() {
		if data, err := os.ReadFile(p); err == nil {
			return string(data)
		}
	}
	return ""
}

func getConfig(key, dotEnv, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	if v := parseEnvValue(dotEnv, key); v != "" {
		return v
	}
	return def
}

func parseEnvValue(content, key string) string {
	prefix := key + "="
	for _, line := range strings.Split(content, "\n") {
		line = trimLine(line)
		if strings.HasPrefix(line, prefix) {
			val := line[len(prefix):]
			if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') ||
				(val[0] == '\'' && val[len(val)-1] == '\'')) {
				val = val[1 : len(val)-1]
			}
			return val
		}
	}
	return ""
}

func trimLine(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "#") {
		return ""
	}
	return s
}
