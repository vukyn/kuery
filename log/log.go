package log

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
)

const (
	LOG_MODE_PRETTY = "pretty"
	LOG_MODE_JSON   = "json"

	LOG_LEVEL_DEBUG    = "debug"
	LOG_LEVEL_INFO     = "info"
	LOG_LEVEL_WARN     = "warn"
	LOG_LEVEL_ERROR    = "error"
	LOG_LEVEL_PANIC    = "panic"
	LOG_LEVEL_DISABLED = "disabled"
)

type Config struct {
	Mode  string // pretty, json
	Level string // debug, info, warn, error, panic, disabled
}

func Init(cfg Config) error {
	zerolog.CallerMarshalFunc = formatCaller // format caller (log.go:77)
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	switch cfg.Mode {
	case LOG_MODE_PRETTY:
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	case LOG_MODE_JSON:
		log.Logger = log.Output(os.Stderr)
	default:
		return fmt.Errorf("invalid log mode: %s", cfg.Mode)
	}

	switch cfg.Level {
	case LOG_LEVEL_DEBUG:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case LOG_LEVEL_INFO:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case LOG_LEVEL_WARN:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case LOG_LEVEL_ERROR:
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case LOG_LEVEL_PANIC:
		zerolog.SetGlobalLevel(zerolog.PanicLevel)
	case LOG_LEVEL_DISABLED:
		zerolog.SetGlobalLevel(zerolog.Disabled)
	default:
		return fmt.Errorf("invalid log level: %s", cfg.Level)
	}

	return nil
}

type Logger struct {
	zerolog.Logger
}

func New(pkg, funcName string) *Logger {
	log := log.With().
		Caller().
		CallerWithSkipFrameCount(3).
		Str("pkg", pkg).
		Str("func", funcName).
		Logger()
	return &Logger{log}
}

func (l *Logger) Info(msg string, args ...any) {
	l.Logger.Info().Msgf(msg, args...)
}

func (l *Logger) Error(msg string, err error) {
	l.Logger.Error().Stack().Err(err).Msg(msg)
}

func (l *Logger) Debug(msg string, args ...any) {
	l.Logger.Debug().Msgf(msg, args...)
}

func (l *Logger) Panic(msg string, args ...any) {
	l.Logger.Panic().Msgf(msg, args...)
}

func formatCaller(pc uintptr, file string, line int) string {
	return filepath.Base(file) + ":" + strconv.Itoa(line)
}
