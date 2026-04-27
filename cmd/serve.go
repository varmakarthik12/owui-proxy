package cmd

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/varmakarthik12/owui-proxy/internal/config"
	"github.com/varmakarthik12/owui-proxy/internal/server"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Ollama-compatible proxy server",
	Long: `Start a local HTTP server that speaks the Ollama REST API on the
configured port. All requests are translated and forwarded to the
configured Open WebUI instance.`,
	RunE: runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)

	f := serveCmd.Flags()

	// Required
	f.String("endpoint", "", "Base URL of Open WebUI (e.g. https://myowui.example.com)")
	f.String("token", "", "Bearer token / Open WebUI API key")

	// Optional
	f.Int("port", 11434, "Local port to listen on")
	f.String("listen-addr", "127.0.0.1", "Bind address (default: localhost-only)")
	f.Bool("bind-all", false, "Bind to 0.0.0.0 (overrides --listen-addr, exposes on network)")
	f.String("mock-version", "0.6.5", "Ollama version string returned by /api/version")
	f.String("api-prefix", "/api", "Open WebUI API prefix for upstream URLs")
	f.Duration("timeout", 300_000_000_000, "HTTP client timeout for upstream requests (e.g. 300s)")
	f.Float64("rate-limit", 0, "Max requests/sec per client IP (0 = disabled)")
	f.String("tls-cert", "", "TLS certificate file path (enables HTTPS)")
	f.String("tls-key", "", "TLS key file path")
	f.Int64("max-body-size", 100*1024*1024, "Max request body size in bytes (default: 100MB)")
	f.Bool("no-default-capabilities", false, "Disable appending default capabilities (completion,vision,tools,thinking) to all models")
	f.Int("default-context-length", 262144, "Minimum context window size reported in /api/show model_info (default: 256K tokens)")
	f.Bool("no-context-length-override", false, "Disable raising small upstream context lengths to --default-context-length")
	f.String("model-prefix", "owui-proxy/", "Prefix added to model IDs in /v1/models and /api/tags; stripped before forwarding (empty string disables)")

	// Bind all flags to viper
	_ = viper.BindPFlag("endpoint", f.Lookup("endpoint"))
	_ = viper.BindPFlag("token", f.Lookup("token"))
	_ = viper.BindPFlag("proxy_port", f.Lookup("port"))
	_ = viper.BindPFlag("listen_addr", f.Lookup("listen-addr"))
	_ = viper.BindPFlag("bind_all", f.Lookup("bind-all"))
	_ = viper.BindPFlag("mock_version", f.Lookup("mock-version"))
	_ = viper.BindPFlag("api_prefix", f.Lookup("api-prefix"))
	_ = viper.BindPFlag("timeout", f.Lookup("timeout"))
	_ = viper.BindPFlag("rate_limit", f.Lookup("rate-limit"))
	_ = viper.BindPFlag("tls_cert", f.Lookup("tls-cert"))
	_ = viper.BindPFlag("tls_key", f.Lookup("tls-key"))
	_ = viper.BindPFlag("max_body_size", f.Lookup("max-body-size"))
	_ = viper.BindPFlag("no_default_capabilities", f.Lookup("no-default-capabilities"))
	_ = viper.BindPFlag("default_context_length", f.Lookup("default-context-length"))
	_ = viper.BindPFlag("no_context_length_override", f.Lookup("no-context-length-override"))
	_ = viper.BindPFlag("model_prefix", f.Lookup("model-prefix"))

	// Env var overrides (without OWUI_ prefix for some)
	_ = viper.BindEnv("endpoint", "OWUI_ENDPOINT")
	_ = viper.BindEnv("token", "OWUI_TOKEN")
	_ = viper.BindEnv("proxy_port", "OWUI_PROXY_PORT")
	_ = viper.BindEnv("listen_addr", "OWUI_LISTEN_ADDR")
	_ = viper.BindEnv("bind_all", "OWUI_BIND_ALL")
	_ = viper.BindEnv("mock_version", "OWUI_MOCK_VERSION")
	_ = viper.BindEnv("api_prefix", "OWUI_API_PREFIX")
	_ = viper.BindEnv("timeout", "OWUI_TIMEOUT")
	_ = viper.BindEnv("rate_limit", "OWUI_RATE_LIMIT")
	_ = viper.BindEnv("tls_cert", "OWUI_TLS_CERT")
	_ = viper.BindEnv("tls_key", "OWUI_TLS_KEY")
	_ = viper.BindEnv("max_body_size", "OWUI_MAX_BODY_SIZE")
	_ = viper.BindEnv("no_default_capabilities", "OWUI_NO_DEFAULT_CAPABILITIES")
	_ = viper.BindEnv("default_context_length", "OWUI_DEFAULT_CONTEXT_LENGTH")
	_ = viper.BindEnv("no_context_length_override", "OWUI_NO_CONTEXT_LENGTH_OVERRIDE")
	_ = viper.BindEnv("model_prefix", "OWUI_MODEL_PREFIX")
}

func runServe(cmd *cobra.Command, args []string) error {
	// Warn if token is passed via CLI flag (visible in process listing).
	if cmd.Flags().Changed("token") {
		slog.Warn("--token passed as CLI flag. Prefer OWUI_TOKEN env var to avoid exposure in process listings (ps aux).")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	slog.Info("starting owui-proxy",
		"endpoint", cfg.Endpoint,
		"token", MaskToken(cfg.Token),
		"listen", cfg.ListenAddress(),
		"mock_version", cfg.MockVersion,
		"api_prefix", cfg.APIPrefix,
		"timeout", cfg.Timeout,
	)

	return server.Run(cmd.Context(), cfg)
}
