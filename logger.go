package logger

import (
	"fmt"
	"github.com/lmittmann/tint"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"
)

// Mod Enum-like types for Mod and LoggerType
type Mod string
type Type string
type Path string

const (
	ModProd Mod = "prod"
	ModDev  Mod = "dev"

	TypeService Type = "service"
	TypeRequest Type = "request"

	DefaultMod         = ModDev
	DefaultType        = TypeService
	DefaultServicePath = "./logs/service_logs/"
	DefaultRequestPath = "./logs/request_logs/"
)

var (
	instance *Logger
	once     sync.Once
)

type Logger struct {
	sync.RWMutex
	*slog.Logger
	logFile    *os.File
	logsDict   Path
	mod        Mod
	LoggerType Type
	today      time.Time
}

// Get returns the singleton instance of Logger
func Get() *Logger {
	return instance
}

// InitLogger initializes the global logger based on environment variables
func init() {
	mod := Mod(os.Getenv("LOGGER_MOD"))
	lType := Type(os.Getenv("LOGGER_TYPE"))
	sPath := Path(os.Getenv("LOGGER_SERVICE_PATH"))

	if mod.Empty() {
		mod = DefaultMod
	}
	if lType.Empty() {
		lType = DefaultType
	}
	if sPath.Empty() {
		sPath = DefaultServicePath
	}

	instance = build(sPath, mod, lType)
	handler := selectHandler(mod, instance.logsDict)
	instance.setLogger(slog.New(handler))
	if mod == ModProd {
		instance.watcher()
	}
	slog.SetDefault(instance.Logger)
}

// build creates a new Logger instance
func build(logsDict Path, mod Mod, lType Type) *Logger {
	return &Logger{
		RWMutex:    sync.RWMutex{},
		logsDict:   logsDict,
		mod:        mod,
		LoggerType: lType,
		today:      time.Now(),
	}
}

// setLogger sets the logger instance
func (ll *Logger) setLogger(logger *slog.Logger) {
	ll.Lock()
	defer ll.Unlock()
	ll.Logger = logger
}

// watcher starts a background goroutine to handle daily log rotation
func (ll *Logger) watcher() {
	go func() {
		for {
			time.Sleep(1 * time.Hour)
			ll.RLock()
			shouldRotate := ll.today.Day() != time.Now().Day()
			ll.RUnlock()
			if shouldRotate {
				ll.setLogger(slog.New(prodHandler(ll.logsDict)))
				ll.Lock()
				ll.today = time.Now()
				ll.Unlock()
			}
		}
	}()
}

// selectHandler returns the appropriate slog.Handler based on the mode
func selectHandler(mod Mod, path Path) slog.Handler {
	if mod == ModProd {
		return prodHandler(path)
	}
	return devHandler()
}

// prodHandler creates a handler for production logging
func prodHandler(path Path) slog.Handler {
	file, err := openLogFile(path)
	if err != nil {
		panic(err)
	}
	return slog.NewJSONHandler(io.MultiWriter(os.Stdout, file), &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	})
}

// devHandler creates a handler for development logging
func devHandler() slog.Handler {
	return tint.NewHandler(os.Stderr, &tint.Options{
		Level:      slog.LevelDebug,
		TimeFormat: "3:04:05PM",
		AddSource:  true,
	})
}

// openLogFile opens or creates a log file at the specified path
func openLogFile(path Path) (*os.File, error) {
	p := path.String() + fmt.Sprintf("%s.txt", time.Now().Format(time.DateOnly))
	err := os.MkdirAll(path.String(), os.ModePerm)
	if err != nil {
		return nil, err
	}
	return os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
}

// Empty Utility methods to check for empty values
func (m Mod) Empty() bool     { return m == "" }
func (t Type) Empty() bool    { return t == "" }
func (p Path) Empty() bool    { return p == "" }
func (p Path) String() string { return string(p) }

func CreateProdLogger(logsDict Path) {
	if logsDict.Empty() {
		logsDict = DefaultServicePath
	}
	logger := build(logsDict, ModProd, DefaultType)
	handler := prodHandler(logsDict)
	logger.setLogger(slog.New(handler))
	logger.watcher()
	instance = logger
	slog.SetDefault(logger.Logger)
}

func CreateDevLogger() {
	logger := build("", ModDev, DefaultType)
	handler := devHandler()
	logger.setLogger(slog.New(handler))
	instance = logger
	slog.SetDefault(logger.Logger)
}

func CreateRequestLogger(mod Mod, logsDict Path) *Logger {
	if logsDict.Empty() {
		logsDict = DefaultRequestPath
	}
	logger := build(logsDict, mod, TypeRequest)
	var handler slog.Handler
	if mod == ModProd {
		handler = prodHandler(logsDict)
		logger.watcher()
	} else {
		handler = devHandler()
	}
	logger.setLogger(slog.New(handler))
	return logger
}
