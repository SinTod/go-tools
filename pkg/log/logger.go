package log

import (
	"fmt"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"path"
	"strings"
	"sync"
)

const (
	MaxUint = ^uint(0)
	MinUint = 0
	MaxInt  = int(MaxUint >> 1)
	MinInt  = -MaxInt - 1
)

var (
	DefaultLogger = NewHelper(With(NewLogger(""),
		"timestamp", DefaultTimestamp,
		"caller", DefaultCaller,
		"service.id", "",
		"app_name", "",
		"service.version", "",
	))
	once sync.Once
)

// CustomDefaultLogger 初始化一个自定义的 logger,只有第一次执行有效
func CustomDefaultLogger(logPath string, id, appName, version, module string) {
	once.Do(func() {
		DefaultLogger = NewHelper(With(NewLogger(logPath),
			"timestamp", DefaultTimestamp,
			"caller", DefaultCaller,
			"service.id", id,
			"app_name", appName,
			"service.version", version,
			"module", module,
		))
	})
}

// ZapLogger 结构体
type ZapLogger struct {
	log  *zap.Logger
	Sync func() error
}

// NewZapLogger 创建一个 ZapLogger 实例
func NewZapLogger(encoder zapcore.EncoderConfig, level zap.AtomicLevel, logPath string, opts ...zap.Option) *ZapLogger {

	writeSyncer, l := getLogWriter(LevelInfo, logPath)
	errWriteSyncer, lErr := getLogWriter(LevelError, logPath)
	//启动定时任务 切割
	c := cron.New(cron.WithSeconds())
	spec := "0 0 0 * * ?" //每天凌晨执行
	//spec := "*/3 * * * * ?" //每三秒执行一次
	c.AddFunc(spec, func() {
		l.Rotate()
		lErr.Rotate()
	})
	c.Start()

	encConsole := zapcore.NewConsoleEncoder(encoder)

	encFile := zapcore.NewJSONEncoder(encoder)

	// 设置 console 默认日志
	coreConsole := zapcore.NewCore(
		encConsole,
		zapcore.NewMultiWriteSyncer(
			zapcore.AddSync(os.Stdout),
		), level)
	// 设置写文件日志
	coreFile := zapcore.NewCore(
		encFile,
		zapcore.NewMultiWriteSyncer(
			zapcore.AddSync(writeSyncer),
		), level)
	// 设置 zapcore 错误日志
	coreErr := zapcore.NewCore(
		encFile,
		zapcore.NewMultiWriteSyncer(
			zapcore.AddSync(errWriteSyncer),
		), zap.NewAtomicLevelAt(zapcore.ErrorLevel))
	//  new 一个 *zap.Logger
	//zapLogger := zap.New(core, opts...)
	// 通过环境变量关闭格式化输出
	var zapLogger *zap.Logger
	if os.Getenv("LOG_STDOUT_OFF") != "" {
		zapLogger = zap.New(zapcore.NewTee(coreFile, coreErr), opts...)
	} else {
		zapLogger = zap.New(zapcore.NewTee(coreConsole, coreFile, coreErr), opts...)
	}
	return &ZapLogger{log: zapLogger, Sync: zapLogger.Sync}
}

// Log 方法实现了 kratos/log/log.go 中的 Logger interface
func (l *ZapLogger) Log(level Level, keyvals ...interface{}) error {
	if len(keyvals) == 0 || len(keyvals)%2 != 0 {
		l.log.Warn(fmt.Sprint("Keyvalues must appear in pairs: ", keyvals))
		return nil
	}
	// 按照 KV 传入的时候,使用的 zap.Field
	var data []zap.Field
	for i := 0; i < len(keyvals); i += 2 {
		data = append(data, zap.Any(fmt.Sprint(keyvals[i]), keyvals[i+1]))
	}
	switch level {
	case LevelDebug:
		l.log.Debug("", data...)
	case LevelInfo:
		l.log.Info("", data...)
	case LevelWarn:
		l.log.Warn("", data...)
	case LevelError:
		l.log.Error("", data...)
	case LevelFatal:
		l.log.Fatal("", data...)
	}
	return nil
}

// 日志自动切割，采用 lumberjack 实现的
func getLogWriter(level Level, logPath string) (zapcore.WriteSyncer, *lumberjack.Logger) {
	var filename string
	defaultPath := "./logs/"

	if logPath != "" {
		defaultPath = logPath
	}
	if level == LevelError {
		filename = path.Join(defaultPath, fmt.Sprintf("%s_json_err.log", "log"))
	} else {
		filename = path.Join(defaultPath, fmt.Sprintf("%s_json.log", "log"))
	}

	lumberJackLogger := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    100,
		MaxBackups: MaxInt,
		MaxAge:     3,
		LocalTime:  true,
		Compress:   true,
	}

	return zapcore.AddSync(lumberJackLogger), lumberJackLogger
}

func NewLogger(logPath string) *ZapLogger {
	encoder := zapcore.EncoderConfig{
		//TimeKey:        "t",
		LevelKey: "log_level",

		//NameKey:        "logger",
		//CallerKey:      "caller",
		//MessageKey:     "msg",
		//StacktraceKey:  "stack",
		//EncodeTime:     zapcore.ISO8601TimeEncoder,
		LineEnding:  zapcore.DefaultLineEnding,
		EncodeLevel: zapcore.CapitalLevelEncoder, //将级别转换成大写
		//EncodeDuration: zapcore.SecondsDurationEncoder,
		//EncodeCaller:   zapcore.FullCallerEncoder,
	}

	l := NewZapLogger(
		encoder,
		getLogLevel(),
		logPath,
		zap.AddStacktrace(
			zap.NewAtomicLevelAt(zapcore.FatalLevel)),
		//zap.AddCallerSkip(2),
		//zap.AddCaller(),
	)

	return l
}

func getLogLevel() zap.AtomicLevel {
	level := os.Getenv("LOG_LEVEL")
	level = strings.ToUpper(level)
	switch level {
	case "DEBUG":
		return zap.NewAtomicLevelAt(zapcore.DebugLevel)
	case "WARN":
		return zap.NewAtomicLevelAt(zapcore.WarnLevel)
	case "ERROR":
		return zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	case "INFO":
		return zap.NewAtomicLevelAt(zapcore.InfoLevel)
	default:
		return zap.NewAtomicLevelAt(zapcore.DebugLevel)
	}
}
