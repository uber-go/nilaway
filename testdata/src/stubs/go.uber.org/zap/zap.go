package zap

// NewProduction builds a sensible production Logger.
func NewProduction() (*Logger, error) {
	return &Logger{}, nil
}


// Logger is a logger interface that provides structured, leveled logging.
type Logger struct{}

// Field represents a key-value pair for structured logging.
type Field struct{}

// Fatal logs a message at fatal level and then calls os.Exit(1).
func (l *Logger) Fatal(msg string, fields ...Field) {}

// Sugared returns a SugaredLogger wrapping this Logger.
func (l *Logger) Sugared() *SugaredLogger {
	return &SugaredLogger{}
}

// SugaredLogger wraps the base Logger to provide a more ergonomic, but slightly slower,
// API. In particular, any key/value pairs passed as arguments are added to the logged
// context using fmt.Sprint-style interpolation.
type SugaredLogger struct{}

// Fatal uses fmt.Sprint to construct and log a message, then calls os.Exit(1).
func (s *SugaredLogger) Fatal(args ...interface{}) {}

// Fatalf uses fmt.Sprintf to log a templated message, then calls os.Exit(1).
func (s *SugaredLogger) Fatalf(template string, args ...interface{}) {}

// Fatalln uses fmt.Sprintln to construct and log a message, then calls os.Exit(1).
func (s *SugaredLogger) Fatalln(args ...interface{}) {}

// Fatalw logs a message with some additional context, then calls os.Exit(1).
// The variadic key-value pairs are treated as they are in With.
func (s *SugaredLogger) Fatalw(msg string, keysAndValues ...interface{}) {}
