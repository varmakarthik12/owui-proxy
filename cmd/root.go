package cmd

import (
	"log/slog"
	"os"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Version, Commit, and BuildDate are populated via ldflags at build time.
	// They are exported so GoReleaser and Makefile can target them directly.
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

// InitVersionInfo initializes version metadata, falling back to Go module
// build info if ldflags weren't provided.
func InitVersionInfo() {
	if Version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok {
			if info.Main.Version != "" && info.Main.Version != "(devel)" {
				Version = info.Main.Version
			}
			for _, setting := range info.Settings {
				switch setting.Key {
				case "vcs.revision":
					Commit = setting.Value
				case "vcs.time":
					BuildDate = setting.Value
				}
			}
		}
	}
}

var rootCmd = &cobra.Command{
	Use:   "owui-proxy",
	Short: "Local Ollama-compatible API proxy for Open WebUI",
	Long: `owui-proxy acts as a local Ollama-compatible API server that accepts
requests in Ollama's native API format, translates them to Open WebUI's
OpenAI-compatible API format, calls Open WebUI, and translates the
response back to Ollama format before returning to the client.

Clients never know they are talking to Open WebUI — to them, owui-proxy
looks and behaves exactly like a native Ollama server.`,
}

// Execute is the top-level entry point called from main().
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().String("log-level", "info", "Log level: debug, info, warn, error")
	rootCmd.PersistentFlags().String("log-format", "text", "Log format: text or json")
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable colored terminal output")

	_ = viper.BindPFlag("log_level", rootCmd.PersistentFlags().Lookup("log-level"))
	_ = viper.BindPFlag("log_format", rootCmd.PersistentFlags().Lookup("log-format"))
	_ = viper.BindPFlag("no_color", rootCmd.PersistentFlags().Lookup("no-color"))
}

func initConfig() {
	viper.SetEnvPrefix("OWUI")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))

	// Also bind NO_COLOR without prefix.
	if v := os.Getenv("NO_COLOR"); v != "" {
		viper.Set("no_color", true)
	}

	initLogger()
}

func initLogger() {
	levelStr := viper.GetString("log_level")
	format := viper.GetString("log_format")

	var level slog.Level
	switch strings.ToLower(levelStr) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	switch strings.ToLower(format) {
	case "json":
		handler = slog.NewJSONHandler(os.Stderr, opts)
	default:
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	slog.SetDefault(slog.New(handler))
}

// MaskToken returns a masked version of a bearer token for safe logging.
func MaskToken(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	if strings.HasPrefix(token, "sk-") && len(token) > 12 {
		return token[:7] + "...****"
	}
	if len(token) > 20 {
		return token[:5] + "..." + token[len(token)-5:]
	}
	return token[:4] + "****"
}
