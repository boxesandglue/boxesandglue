package bag

import (
	"os"

	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/sirupsen/logrus"
)

// Logging
// Several things are copied from logrus. One day I might drop the package, but the API
// should be stable.
var log *logrus.Logger

func init() {
	log = logrus.New()
	log.SetFormatter(&nested.Formatter{
		HideKeys:    true,
		FieldsOrder: []string{"component", "category"},
	})

	log.SetOutput(os.Stdout)
}

// Level type
type Level uint32

// SetLogLevel sets the logging level
func SetLogLevel(level Level) {
	switch level {
	case PanicLevel:
		log.SetLevel(logrus.PanicLevel)
	case FatalLevel:
		log.SetLevel(logrus.FatalLevel)
	case ErrorLevel:
		log.SetLevel(logrus.ErrorLevel)
	case WarnLevel:
		log.SetLevel(logrus.WarnLevel)
	case InfoLevel:
		log.SetLevel(logrus.InfoLevel)
	case DebugLevel:
		log.SetLevel(logrus.DebugLevel)
	case TraceLevel:
		log.SetLevel(logrus.TraceLevel)
	}
}

const (
	// PanicLevel level, highest level of severity. Logs and then calls panic with the
	// message passed to Debug, Info, ...
	PanicLevel Level = iota
	// FatalLevel level. Logs and then calls `logger.Exit(1)`. It will exit even if the
	// logging level is set to Panic.
	FatalLevel
	// ErrorLevel level. Logs. Used for errors that should definitely be noted.
	// Commonly used for hooks to send errors to an error tracking service.
	ErrorLevel
	// WarnLevel level. Non-critical entries that deserve eyes.
	WarnLevel
	// InfoLevel level. General operational entries about what's going on inside the
	// application.
	InfoLevel
	// DebugLevel level. Usually only enabled when debugging. Very verbose logging.
	DebugLevel
	// TraceLevel level. Designates finer-grained informational events than the Debug.
	TraceLevel
)

// LogError logs with log level ErrorLevel
func LogError(args ...interface{}) {
	log.Error(args...)
}

// LogWarn logs with log level WarnLevel
func LogWarn(args ...interface{}) {
	log.Warn(args...)
}

// LogInfo logs with log level InfoLevel
func LogInfo(args ...interface{}) {
	log.Info(args...)
}

// LogDebug logs with log level DebugLevel
func LogDebug(args ...interface{}) {
	log.Debug(args...)
}

// LogTrace logs with log level TraceLevel
func LogTrace(args ...interface{}) {
	log.Trace(args...)
}

// Fields type, used to pass to `WithFields`.
type Fields map[string]interface{}

// LogWithFields sets key values for additional logging
func LogWithFields(f Fields) *logrus.Entry {
	return log.WithFields(logrus.Fields(f))
}
