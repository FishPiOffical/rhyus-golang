package common

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

const (
	ExitCodeReadOnlyDatabase = 20 // 数据库文件被锁
	ExitCodeUnavailablePort  = 21 // 端口不可用
	ExitCodeWorkspaceLocked  = 24 // 工作空间已被锁定
	ExitCodeInitWorkspaceErr = 25 // 初始化工作空间失败
	ExitCodeFileSysErr       = 26 // 文件系统错误
	ExitCodeCertificateErr   = 26 // 生成证书错误
	ExitCodeUnsafe           = 26 // 由于不安全退出
	ExitCodeOk               = 0  // 正常退出
	ExitCodeFatal            = 1  // 致命错误
)

var (
	level   int
	logger  *log.Logger
	logFile *os.File
	LogPath string
)

// init 初始化日志输出位置及文件名
func init() {
	dir, err := os.Getwd()
	if nil != err {
		log.Printf("get current dir failed: %s", err)
	}
	LogPath = filepath.Join(dir, "log", "logging.log")
}

func (g *GuLog) SetLogPath(path string) {
	LogPath = path
}

func (g *GuLog) Trace(format string, v ...any) {
	defer g.closeLogger()
	g.openLogger()

	if !g.isTraceEnabled() {
		return
	}
	g.logTrace(format, v...)
}

func (g *GuLog) Debug(format string, v ...any) {
	defer g.closeLogger()
	g.openLogger()

	if !g.isDebugEnabled() {
		return
	}
	g.logDebug(format, v...)
}

func (g *GuLog) Info(format string, v ...any) {
	defer g.closeLogger()
	g.openLogger()
	g.logInfo(format, v...)
}

func (g *GuLog) Error(format string, v ...any) {
	defer g.closeLogger()
	g.openLogger()
	g.logError(format, v...)
}

func (g *GuLog) Warn(format string, v ...any) {
	defer g.closeLogger()
	g.openLogger()

	if !g.isWarnEnabled() {
		return
	}
	g.logWarn(format, v...)
}

func (g *GuLog) Fatal(exitCode int, format string, v ...any) {
	g.openLogger()
	g.logFatal(exitCode, format, v...)
}

var logLock = sync.Mutex{}

func (g *GuLog) openLogger() {
	logLock.Lock()

	// Todo 临时解决日志文件过大的问题
	if File.IsExist(LogPath) {
		if size := File.GetFileSize(LogPath); 1024*1024*32 <= size {
			// 日志文件大于 32M 的话重命名
			timestamp := time.Now().Format("2006-01-02_15-04-05") // 获取当前时间戳格式化为字符串
			newLogPath := fmt.Sprintf("%s.%s", LogPath, timestamp)

			if err := os.Rename(LogPath, newLogPath); err != nil {
				log.Printf("rename log file from [%s] to [%s] failed: %s", LogPath, newLogPath, err)
			} else {
				log.Printf("save log file to [%s] success", newLogPath)
			}
		}
	}

	dir, _ := filepath.Split(LogPath)
	if !File.IsExist(dir) {
		if err := os.MkdirAll(dir, 0755); nil != err {
			log.Printf("create log dir [%s] failed: %s", dir, err)
		}
	}

	var err error
	logFile, err = os.OpenFile(LogPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if nil != err {
		log.Printf("create log file [%s] failed: %s", LogPath, err)
	}
	g.initLogger(io.MultiWriter(os.Stdout, logFile))
}

func (g *GuLog) closeLogger() {
	_ = logFile.Close()
	logLock.Unlock()
}

func (g *GuLog) Recover() {
	if e := recover(); nil != e {
		stack := g.stack()
		msg := fmt.Sprintf("PANIC RECOVERED: %v\n%s\n", e, stack)
		g.Error(msg)
	}
}

func (g *GuLog) RecoverError(e any) {
	if nil != e {
		stack := g.stack()
		msg := fmt.Sprintf("PANIC RECOVERED: %v\n%s\n", e, stack)
		g.Error(msg)
	}
}

var (
	dunno     = []byte("???")
	centerDot = []byte("·")
	dot       = []byte(".")
	slash     = []byte("/")
)

// stack implements Stack, skipping 2 frames.
func (g *GuLog) stack() []byte {
	buf := &bytes.Buffer{} // the returned data
	// As we loop, we open files and read them. These variables record the currently
	// loaded file.
	var lines [][]byte
	var lastFile string
	for i := 2; ; i++ { // Caller we care about is the user, 2 frames up
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		// Print this much at least.  If we can't find the source, it won't show.
		_, _ = fmt.Fprintf(buf, "%s:%d (0x%x)\n", file, line, pc)
		if file != lastFile {
			data, err := os.ReadFile(file)
			if err != nil {
				continue
			}
			lines = bytes.Split(data, []byte{'\n'})
			lastFile = file
		}
		line-- // in stack trace, lines are 1-indexed but our array is 0-indexed
		_, _ = fmt.Fprintf(buf, "\t%s: %s\n", g.function(pc), g.source(lines, line))
	}
	return buf.Bytes()
}

// source returns a space-trimmed slice of the n'th line.
func (g *GuLog) source(lines [][]byte, n int) []byte {
	if n < 0 || n >= len(lines) {
		return dunno
	}
	return bytes.Trim(lines[n], " \t")
}

// function returns, if possible, the name of the function containing the PC.
func (g *GuLog) function(pc uintptr) []byte {
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return dunno
	}
	name := []byte(fn.Name())
	// The name includes the path name to the package, which is unnecessary
	// since the file name is already included.  Plus, it has center dots.
	// That is, we see
	//	runtime/debug.*T·ptrmethod
	// and want
	//	*T.ptrmethod
	// Since the package path might contains dots (e.g. code.google.com/...),
	// we first remove the path prefix if there is one.
	if lastslash := bytes.LastIndex(name, slash); lastslash >= 0 {
		name = name[lastslash+1:]
	}
	if period := bytes.Index(name, dot); period >= 0 {
		name = name[period+1:]
	}
	name = bytes.Replace(name, centerDot, dot, -1)
	return name
}

// Logging level.
const (
	OFF = iota
	TRACE
	DEBUG
	INFO
	WARN
	ERROR
	FATAL
)

// initLogger initializes the logger.
func (g *GuLog) initLogger(out io.Writer) {
	logger = log.New(out, "", log.Ldate|log.Ltime|log.Lshortfile)
}

// SetLogLevel sets the logging level of all loggers.
func (g *GuLog) SetLogLevel(logLevel string) {
	level = g.GetLevel(logLevel)
}

// GetLevel gets logging level int value corresponding to the specified level.
func (g *GuLog) GetLevel(logLevel string) int {
	logLevel = strings.ToLower(logLevel)

	switch logLevel {
	case "off":
		return OFF
	case "trace":
		return TRACE
	case "debug":
		return DEBUG
	case "info":
		return INFO
	case "warn":
		return WARN
	case "error":
		return ERROR
	case "fatal":
		return FATAL
	default:
		return INFO
	}
}

// isTraceEnabled determines whether the trace level is enabled.
func (g *GuLog) isTraceEnabled() bool {
	return level <= TRACE
}

// isDebugEnabled determines whether the debug level is enabled.
func (g *GuLog) isDebugEnabled() bool {
	return level <= DEBUG
}

// isWarnEnabled determines whether the debug level is enabled.
func (g *GuLog) isWarnEnabled() bool {
	return level <= WARN
}

// logTrace prints trace level message with format.
func (g *GuLog) logTrace(format string, v ...any) {
	if TRACE < level {
		return
	}

	logger.SetPrefix("TRACE ")
	_ = logger.Output(3, fmt.Sprintf(format, v...))
}

// logDebug prints debug level message with format.
func (g *GuLog) logDebug(format string, v ...any) {
	if DEBUG < level {
		return
	}

	logger.SetPrefix("DEBUG ")
	_ = logger.Output(3, fmt.Sprintf(format, v...))
}

// logInfo prints info level message with format.
func (g *GuLog) logInfo(format string, v ...any) {
	if INFO < level {
		return
	}

	logger.SetPrefix("INFO  ")
	_ = logger.Output(3, fmt.Sprintf(format, v...))
}

// logWarn prints warning level message with format.
func (g *GuLog) logWarn(format string, v ...any) {
	if WARN < level {
		return
	}

	logger.SetPrefix("WARN  ")
	msg := fmt.Sprintf(format, v...)
	_ = logger.Output(3, msg)
}

// logError prints error level message with format.
func (g *GuLog) logError(format string, v ...any) {
	if ERROR < level {
		return
	}

	logger.SetPrefix("ERROR ")
	format += "\n%s"
	v = append(v, g.ShortStack())
	msg := fmt.Sprintf(format, v...)
	_ = logger.Output(3, msg)
}

// logFatal prints fatal level message with format and exit process with code 1.
func (g *GuLog) logFatal(exitCode int, format string, v ...any) {
	if FATAL < level {
		return
	}

	logger.SetPrefix("FATAL ")
	format += "\n%s"
	v = append(v, g.ShortStack())
	msg := fmt.Sprintf(format, v...)
	_ = logger.Output(3, msg)
	g.closeLogger()
	os.Exit(exitCode)
}

func (g *GuLog) ShortStack() string {
	output := string(debug.Stack())
	lines := strings.Split(output, "\n")
	if 32 < len(lines) {
		lines = lines[32:]
	}
	buf := bytes.Buffer{}
	for _, l := range lines {
		if strings.Contains(l, "gin-gonic") {
			break
		}
		buf.WriteString("    ")
		buf.WriteString(l)
		buf.WriteByte('\n')
	}
	return buf.String()
}
