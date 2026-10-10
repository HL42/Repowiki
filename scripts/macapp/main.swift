// RepoWiki macOS 原生壳：启动 Resources 里的 repowiki 后端，然后用 WKWebView
// 把 Web UI 内嵌进一个独立窗口（不再借道系统默认浏览器）。
//
// 与后端的契约：
//   - 后端以 --no-open 启动（不自己开浏览器）；
//   - 后端通过 --port-file <path> 把实际监听端口写进文件，本壳读取后拼出 URL。
import Cocoa
import WebKit

final class AppDelegate: NSObject, NSApplicationDelegate {
    private var window: NSWindow!
    private var server: Process?

    func applicationDidFinishLaunching(_ notification: Notification) {
        buildMenu()
        startServerAndLoadUI()
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool { true }

    func applicationWillTerminate(_ notification: Notification) {
        server?.terminate()
    }

    // ---------- 启动 ----------

    private func startServerAndLoadUI() {
        guard let binPath = locateServerBinary() else {
            showError("未找到 repowiki 服务程序，请确认安装包完整。")
            NSApp.terminate(nil)
            return
        }

        let portFile = NSTemporaryDirectory() + "repowiki-port-\(ProcessInfo.processInfo.processIdentifier).txt"
        try? FileManager.default.removeItem(atPath: portFile)

        let p = Process()
        p.executableURL = URL(fileURLWithPath: binPath)
        p.arguments = ["--no-open", "--port-file", portFile]

        // 把后端输出写进日志，便于排查（找不到数据目录时静默丢弃）
        redirectToLog(p)

        do {
            try p.run()
        } catch {
            showError("启动服务失败: \(error.localizedDescription)")
            NSApp.terminate(nil)
            return
        }
        server = p

        let port = waitForPort(file: portFile, timeout: 8.0)
        guard let url = URL(string: "http://127.0.0.1:\(port)/") else {
            NSApp.terminate(nil)
            return
        }

        let rect = NSRect(x: 0, y: 0, width: 1200, height: 800)
        window = NSWindow(
            contentRect: rect,
            styleMask: [.titled, .closable, .miniaturizable, .resizable],
            backing: .buffered,
            defer: false
        )
        window.title = "RepoWiki"
        window.minSize = NSSize(width: 900, height: 600)
        window.center()

        let webView = WKWebView(frame: rect, configuration: WKWebViewConfiguration())
        window.contentView = webView
        webView.load(URLRequest(url: url))

        window.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }

    // ---------- 辅助 ----------

    private func locateServerBinary() -> String? {
        let exeURL = Bundle.main.executableURL ?? URL(fileURLWithPath: CommandLine.arguments[0])
        let resourcesURL = exeURL
            .deletingLastPathComponent()        // MacOS/
            .deletingLastPathComponent()        // Contents/
            .appendingPathComponent("Resources")

        let candidates: [URL?] = [
            resourcesURL.appendingPathComponent("repowiki"),
            Bundle.main.resourceURL?.appendingPathComponent("repowiki"),
        ]
        for c in candidates {
            if let c = c, FileManager.default.isExecutableFile(atPath: c.path) {
                return c.path
            }
        }
        return nil
    }

    private func dataDir() -> URL {
        let base = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask).first
            ?? FileManager.default.homeDirectoryForCurrentUser
        return base.appendingPathComponent("RepoWiki", isDirectory: true)
    }

    private func redirectToLog(_ p: Process) {
        let logURL = dataDir().appendingPathComponent("app.log")
        do {
            try FileManager.default.createDirectory(at: dataDir(), withIntermediateDirectories: true)
            if FileManager.default.createFile(atPath: logURL.path, contents: nil),
               let fh = FileHandle(forWritingAtPath: logURL.path) {
                p.standardOutput = fh
                p.standardError = fh
            }
        } catch { /* 日志不可写不致命 */ }
    }

    private func waitForPort(file: String, timeout: TimeInterval) -> String {
        let deadline = Date().addingTimeInterval(timeout)
        while Date() < deadline {
            if let s = try? String(contentsOfFile: file, encoding: .utf8) {
                let t = s.trimmingCharacters(in: .whitespacesAndNewlines)
                if let port = Int(t), port > 0 {
                    return String(port)
                }
            }
            Thread.sleep(forTimeInterval: 0.05)
        }
        return "8080" // 兜底：正常情况下 8 秒内必写端口
    }

    private func buildMenu() {
        let mainMenu = NSMenu()

        let appMenuItem = NSMenuItem()
        mainMenu.addItem(appMenuItem)
        let appMenu = NSMenu()
        appMenu.addItem(withTitle: "退出 RepoWiki", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")
        appMenuItem.submenu = appMenu

        let editMenuItem = NSMenuItem()
        mainMenu.addItem(editMenuItem)
        let editMenu = NSMenu(title: "编辑")
        editMenu.addItem(withTitle: "拷贝", action: #selector(NSText.copy(_:)), keyEquivalent: "c")
        editMenu.addItem(withTitle: "粘贴", action: #selector(NSText.paste(_:)), keyEquivalent: "v")
        editMenu.addItem(withTitle: "全选", action: #selector(NSText.selectAll(_:)), keyEquivalent: "a")
        editMenuItem.submenu = editMenu

        NSApp.mainMenu = mainMenu
    }

    private func showError(_ msg: String) {
        let alert = NSAlert()
        alert.messageText = "RepoWiki"
        alert.informativeText = msg
        alert.alertStyle = .critical
        alert.runModal()
    }
}

// 入口：main.swift 的顶层代码即程序入口（无需 @main）
let app = NSApplication.shared
let delegate = AppDelegate()
app.delegate = delegate
app.setActivationPolicy(.regular)
app.run()
