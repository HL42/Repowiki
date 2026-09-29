package wiki

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"repowiki/internal/storage"
)

// ValidationReport 导入前置检查报告
type ValidationReport struct {
	Passed            bool     `json:"passed"`
	TotalFiles        int      `json:"total_files"`
	NewFiles          int      `json:"new_files"`
	ChangedFiles      int      `json:"changed_files"`
	UnchangedFiles    int      `json:"unchanged_files"`
	CrossProjectDupes []string `json:"cross_project_dupes"`
	Warnings          []string `json:"warnings"`
	Errors            []string `json:"errors"`
}

// ValidateImport 执行三重导入前置检查（只读，不修改任何文件）
// Check A: 哈希匹配 — 检测文件是否全未变化，避免无效导入
// Check B: 跨项目碰撞 — 检测特征文件是否已在其他子项目中归档
// Check C: 路径合理性 — 路径存在、非噪声目录、文件数量、项目结构
func (e *Engine) ValidateImport(rawPath string) *ValidationReport {
	report := &ValidationReport{Passed: true}

	// ---- Check C: 路径合理性 ----
	info, err := os.Stat(rawPath)
	if err != nil {
		report.Passed = false
		report.Errors = append(report.Errors, fmt.Sprintf("路径不存在或不可读: %v", err))
		return report
	}
	if !info.IsDir() {
		report.Passed = false
		report.Errors = append(report.Errors, "路径不是目录")
		return report
	}

	base := filepath.Base(rawPath)
	if isNoiseDirName(base) {
		report.Warnings = append(report.Warnings,
			fmt.Sprintf("目标目录名 %q 可能是构建产物或依赖目录，请确认路径正确", base))
	}

	if !hasRecognizableProjectFile(rawPath) {
		report.Warnings = append(report.Warnings,
			"未识别到标准项目结构文件（go.mod/package.json/pom.xml/requirements.txt 等），请确认路径正确")
	}

	// 收集所有支持的文件
	files := collectImportableFiles(rawPath)
	report.TotalFiles = len(files)
	if len(files) == 0 {
		report.Warnings = append(report.Warnings,
			"目录中没有支持的文件类型（.md/.go/.py/.js/.ts/.vue/.java/.rs）")
		return report
	}
	if len(files) > 5000 {
		report.Warnings = append(report.Warnings,
			fmt.Sprintf("文件数量过多（%d 个），建议用 wiki-import-auditor 筛选核心文件后再导入", len(files)))
	}

	// ---- Check A: 哈希匹配检测 ----
	rawDir := filepath.Join(e.Root, "raw")
	for _, f := range files {
		content, readErr := os.ReadFile(f)
		if readErr != nil {
			continue
		}
		newHash := storage.ComputeHash(string(content))

		archivedName := archiveSlugForValidate(f)
		archivedPath := filepath.Join(rawDir, archivedName)

		if _, statErr := os.Stat(archivedPath); os.IsNotExist(statErr) {
			report.NewFiles++
			continue
		}

		hashPath := filepath.Join(rawDir, archivedName+".hash")
		oldHashData, hashErr := os.ReadFile(hashPath)
		if hashErr != nil {
			// 有归档但无哈希 → 迁移场景，视为新文件
			report.NewFiles++
			continue
		}
		oldHash := strings.TrimSpace(string(oldHashData))

		if oldHash == newHash {
			report.UnchangedFiles++
		} else {
			report.ChangedFiles++
		}
	}

	if report.UnchangedFiles == report.TotalFiles && report.TotalFiles > 0 {
		report.Warnings = append(report.Warnings,
			fmt.Sprintf("所有 %d 个文件均未变化，重复导入不会产生新数据", report.TotalFiles))
	}

	// ---- Check B: 跨项目碰撞检测 ----
	crossDupes := detectCrossProjectDupes(rawPath, e.Root)
	report.CrossProjectDupes = crossDupes
	for _, d := range crossDupes {
		report.Warnings = append(report.Warnings, d)
	}

	if len(report.Errors) > 0 {
		report.Passed = false
	}

	return report
}

// ---------- Check A 辅助 ----------

// collectImportableFiles 遍历目录收集所有支持的文件，保持与 IngestAll 过滤规则一致
func collectImportableFiles(dirPath string) []string {
	var files []string
	filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") && name != "." {
				return filepath.SkipDir
			}
			if isNoiseDirName(name) {
				return filepath.SkipDir
			}
			return nil
		}
		// 跳过符号链接（避免无限递归或遍历到预期外路径）
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		name := info.Name()
		ext := strings.ToLower(filepath.Ext(name))
		supported := map[string]bool{
			".md": true, ".go": true, ".py": true,
			".js": true, ".ts": true, ".vue": true,
			".java": true, ".rs": true,
		}
		if !supported[ext] {
			return nil
		}
		// 保持与 IngestAll 一致：跳过测试/生成/样式/mock 文件
		baseName := strings.TrimSuffix(name, filepath.Ext(name))
		if isNoiseCodeFile(name, baseName) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	return files
}

// archiveSlugForValidate 复用 ingest.go 中的 archiveSlug 逻辑，确保验证与导入的归档名一致
func archiveSlugForValidate(rawPath string) string {
	return archiveSlug(rawPath)
}

// ---------- Check C 辅助 ----------

// noiseDirNames 统一噪声目录列表，供 IngestAll 和 ValidateImport 共用
var noiseDirNames = map[string]bool{
	"node_modules": true, "vendor": true, ".git": true,
	"__pycache__": true, "dist": true, "build": true,
	".next": true, ".nuxt": true, "target": true, "bin": true, "obj": true,
	"mock": true, "mocks": true, "__snapshots__": true,
	"fixtures": true, "testdata": true,
	".idea": true, ".vscode": true, ".svn": true,
}

func isNoiseDirName(name string) bool {
	return noiseDirNames[name]
}

func hasRecognizableProjectFile(dirPath string) bool {
	markers := []string{
		"go.mod", "package.json", "pom.xml", "requirements.txt",
		"Cargo.toml", "CMakeLists.txt", "Makefile", "README.md",
	}
	// 检查根目录
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(dirPath, m)); err == nil {
			return true
		}
	}
	// 检查一级子目录（monorepo 场景，如 packages/a/go.mod）
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		for _, m := range markers {
			if _, err := os.Stat(filepath.Join(dirPath, entry.Name(), m)); err == nil {
				return true
			}
		}
	}
	return false
}

// ---------- Check B 辅助 ----------

// signatureFileNames 入口/配置文件，用于跨项目碰撞检测
var signatureFileNames = []string{
	"go.mod", "package.json", "main.go", "App.vue", "pages.json",
	"app.json", "main.js", "index.js", "index.ts", "main.ts",
	"pom.xml", "build.gradle", "Cargo.toml", "requirements.txt",
}

// detectCrossProjectDupes 检查目标目录的特征文件是否已在兄弟子项目中归档
func detectCrossProjectDupes(rawPath, engineRoot string) []string {
	var dupes []string

	sigFiles := findSignatureFiles(rawPath)
	if len(sigFiles) == 0 {
		return nil
	}

	// 确定扫描范围：找 engineRoot 的父目录下是否有其他 sub-projects 目录
	// engineRoot 可能是 .../sub-projects/{name} 或 .../projects/{name}
	parentDir := filepath.Dir(engineRoot)
	parentName := filepath.Base(parentDir)

	var scanDirs []string

	if parentName == "sub-projects" {
		// 引擎在子项目层级 → 扫描兄弟子项目
		entries, err := os.ReadDir(parentDir)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				siblingRaw := filepath.Join(parentDir, entry.Name(), "raw")
				if _, err := os.Stat(siblingRaw); err == nil {
					scanDirs = append(scanDirs, siblingRaw)
				}
			}
		}
	} else {
		// 引擎可能在项目层级 → 扫描 sub-projects/ 下的子项目
		subProjDir := filepath.Join(engineRoot, "sub-projects")
		entries, err := os.ReadDir(subProjDir)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				siblingRaw := filepath.Join(subProjDir, entry.Name(), "raw")
				if _, err := os.Stat(siblingRaw); err == nil {
					scanDirs = append(scanDirs, siblingRaw)
				}
			}
		}
	}

	for _, sig := range sigFiles {
		sigName := filepath.Base(sig)
		sigArchiveName := archiveSlugForValidate(sig)
		// 归一化 rawPath 为绝对路径（用于路径比较）
		absRawPath, _ := filepath.Abs(rawPath)
		for _, scanDir := range scanDirs {
			// 跳过自身：用绝对路径判断 rawPath 是否属于当前扫描项目
			projDir := filepath.Dir(scanDir) // scanDir = .../project/raw，往上即为项目目录
			if strings.HasPrefix(absRawPath, projDir+string(os.PathSeparator)) || absRawPath == projDir {
				continue
			}
			if _, err := os.Stat(filepath.Join(scanDir, sigArchiveName)); err == nil {
				siblingName := filepath.Base(filepath.Dir(scanDir))
				dupes = append(dupes, fmt.Sprintf(
					"特征文件 %q 也存在于子项目 %q 的归档中，可能是同一代码库的重复导入",
					sigName, siblingName,
				))
			}
		}
	}

	return dupes
}

// findSignatureFiles 在目录中查找特征文件（含一级子目录，适配 monorepo）
func findSignatureFiles(dirPath string) []string {
	var found []string
	for _, name := range signatureFileNames {
		p := filepath.Join(dirPath, name)
		if _, err := os.Stat(p); err == nil {
			found = append(found, p)
		}
	}
	// 也检查一级子目录（monorepo 场景）
	entries, err := os.ReadDir(dirPath)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			for _, name := range signatureFileNames {
				p := filepath.Join(dirPath, e.Name(), name)
				if _, err := os.Stat(p); err == nil {
					found = append(found, p)
					break
				}
			}
		}
	}
	return found
}
