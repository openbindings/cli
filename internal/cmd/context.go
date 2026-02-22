package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/openbindings/cli/internal/app"
	"github.com/openbindings/cli/internal/delegates"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newContextCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Manage binding context (credentials, headers, environment)",
		Long: `Manage named contexts for operation execution.

A context is a named collection of credentials, headers, cookies,
environment variables, and metadata. When executing an operation,
pass --context <name> to apply a context to the request.

Credentials are stored securely in the OS keychain. Non-secret
fields (headers, environment, metadata) are stored in config files.`,
	}

	cmd.AddCommand(
		newContextListCmd(),
		newContextShowCmd(),
		newContextSetCmd(),
		newContextRemoveCmd(),
		newContextGetCmd(),
	)

	return cmd
}

func newContextListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all named contexts",
		RunE: func(cmd *cobra.Command, args []string) error {
			summaries, err := app.ListContexts()
			if err != nil {
				return app.ExitResult{Code: 1, Message: err.Error(), ToStderr: true}
			}
			format, outputPath := getOutputFlags(cmd)
			return app.OutputResultText(summaries, format, outputPath, func() string {
				return app.RenderContextList(summaries)
			})
		},
	}
}

func newContextShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show context details (secrets masked)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			ctx, err := app.GetContext(name)
			if err != nil {
				return app.ExitResult{Code: 1, Message: err.Error(), ToStderr: true}
			}
			format, outputPath := getOutputFlags(cmd)
			return app.OutputResultText(ctx, format, outputPath, func() string {
				return app.RenderBindingContext(ctx)
			})
		},
	}
}

func newContextSetCmd() *cobra.Command {
	var (
		bearerToken string
		apiKey      string
		basic       bool
		headers     []string
		cookies     []string
		envVars     []string
		metaEntries []string
	)

	cmd := &cobra.Command{
		Use:   "set <name>",
		Short: "Set context fields (credentials, headers, environment)",
		Long: `Set fields on a named context. Creates the context if it doesn't exist.

Credential flags (--bearer-token, --api-key, --basic) store values
securely in the OS keychain. If no value is provided after the flag,
you'll be prompted to enter it securely.

Non-secret flags (--header, --cookie, --env, --meta) are stored in
a config file and can be specified multiple times.

Examples:
  ob context set github --bearer-token
  ob context set github --bearer-token=ghp_xxxx
  ob context set stripe --api-key
  ob context set myapi --basic
  ob context set github --header "Accept: application/vnd.github+json"
  ob context set myapi --env "API_URL=https://api.example.com"
  ob context set myapi --meta "org=acme"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			cfg, err := app.LoadContextConfig(name)
			if err != nil {
				return app.ExitResult{Code: 1, Message: err.Error(), ToStderr: true}
			}

			cred, err := app.LoadContextCredentials(name)
			if err != nil {
				return app.ExitResult{Code: 1, Message: err.Error(), ToStderr: true}
			}

			credChanged := false

			if cmd.Flags().Changed("bearer-token") {
				val := bearerToken
				if val == "" {
					v, err := promptSecret("Bearer token: ")
					if err != nil {
						return app.ExitResult{Code: 1, Message: err.Error(), ToStderr: true}
					}
					val = v
				}
				if cred == nil {
					cred = &delegates.Credentials{}
				}
				cred.BearerToken = val
				credChanged = true
			}

			if cmd.Flags().Changed("api-key") {
				val := apiKey
				if val == "" {
					v, err := promptSecret("API key: ")
					if err != nil {
						return app.ExitResult{Code: 1, Message: err.Error(), ToStderr: true}
					}
					val = v
				}
				if cred == nil {
					cred = &delegates.Credentials{}
				}
				cred.APIKey = val
				credChanged = true
			}

			if basic {
				fmt.Print("Username: ")
				var username string
				if _, err := fmt.Scanln(&username); err != nil {
					return app.ExitResult{Code: 1, Message: fmt.Sprintf("reading username: %v", err), ToStderr: true}
				}
				password, err := promptSecret("Password: ")
				if err != nil {
					return app.ExitResult{Code: 1, Message: err.Error(), ToStderr: true}
				}
				if cred == nil {
					cred = &delegates.Credentials{}
				}
				cred.Basic = &delegates.BasicCredentials{
					Username: username,
					Password: password,
				}
				credChanged = true
			}

			cfgChanged := false

			for _, h := range headers {
				k, v, ok := parseKV(h, ":")
				if !ok {
					return app.ExitResult{Code: 1, Message: fmt.Sprintf("invalid header %q (expected \"Key: Value\")", h), ToStderr: true}
				}
				if cfg.Headers == nil {
					cfg.Headers = make(map[string]string)
				}
				cfg.Headers[k] = v
				cfgChanged = true
			}

			for _, c := range cookies {
				k, v, ok := parseKV(c, "=")
				if !ok {
					return app.ExitResult{Code: 1, Message: fmt.Sprintf("invalid cookie %q (expected \"Key=Value\")", c), ToStderr: true}
				}
				if cfg.Cookies == nil {
					cfg.Cookies = make(map[string]string)
				}
				cfg.Cookies[k] = v
				cfgChanged = true
			}

			for _, e := range envVars {
				k, v, ok := parseKV(e, "=")
				if !ok {
					return app.ExitResult{Code: 1, Message: fmt.Sprintf("invalid env %q (expected \"VAR=value\")", e), ToStderr: true}
				}
				if cfg.Environment == nil {
					cfg.Environment = make(map[string]string)
				}
				cfg.Environment[k] = v
				cfgChanged = true
			}

			for _, m := range metaEntries {
				k, v, ok := parseKV(m, "=")
				if !ok {
					return app.ExitResult{Code: 1, Message: fmt.Sprintf("invalid meta %q (expected \"key=value\")", m), ToStderr: true}
				}
				if cfg.Metadata == nil {
					cfg.Metadata = make(map[string]any)
				}
				cfg.Metadata[k] = v
				cfgChanged = true
			}

			if !credChanged && !cfgChanged {
				return app.ExitResult{Code: 1, Message: "no fields specified; use --bearer-token, --api-key, --basic, --header, --cookie, --env, or --meta", ToStderr: true}
			}

			if credChanged {
				if err := app.SaveContextCredentials(name, cred); err != nil {
					return app.ExitResult{Code: 1, Message: err.Error(), ToStderr: true}
				}
			}

			if cfgChanged {
				if err := app.SaveContextConfig(name, cfg); err != nil {
					return app.ExitResult{Code: 1, Message: err.Error(), ToStderr: true}
				}
			}

			fmt.Fprintf(os.Stderr, "Context %q updated.\n", name)
			return nil
		},
	}

	cmd.Flags().StringVar(&bearerToken, "bearer-token", "", "bearer token (prompts if empty)")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key (prompts if empty)")
	cmd.Flags().BoolVar(&basic, "basic", false, "set basic auth (prompts for username and password)")
	cmd.Flags().StringArrayVar(&headers, "header", nil, "add header as \"Key: Value\" (repeatable)")
	cmd.Flags().StringArrayVar(&cookies, "cookie", nil, "add cookie as \"Key=Value\" (repeatable)")
	cmd.Flags().StringArrayVar(&envVars, "env", nil, "add env var as \"VAR=value\" (repeatable)")
	cmd.Flags().StringArrayVar(&metaEntries, "meta", nil, "add metadata as \"key=value\" (repeatable)")

	return cmd
}

func newContextRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm"},
		Short:   "Remove a named context",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if !app.ContextExists(name) {
				return app.ExitResult{Code: 1, Message: fmt.Sprintf("context %q not found", name), ToStderr: true}
			}
			if err := app.DeleteContext(name); err != nil {
				return app.ExitResult{Code: 1, Message: err.Error(), ToStderr: true}
			}
			fmt.Fprintf(os.Stderr, "Context %q removed.\n", name)
			return nil
		},
	}
}

func newContextGetCmd() *cobra.Command {
	var contextName string

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get binding context for a target",
		Long: `Get the binding context that would be applied when executing against
a binding target. Returns credentials, headers, environment variables,
and other context configured for the target.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := app.GetContext(contextName)
			if err != nil {
				return app.ExitResult{Code: 1, Message: err.Error(), ToStderr: true}
			}
			format, outputPath := getOutputFlags(cmd)
			return app.OutputResultText(ctx, format, outputPath, func() string {
				return app.RenderBindingContext(ctx)
			})
		},
	}

	cmd.Flags().StringVar(&contextName, "name", "", "context name to load")

	return cmd
}

// promptSecret prompts for a secret value with no echo.
func promptSecret(prompt string) (string, error) {
	fmt.Print(prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("reading secret: %w", err)
	}
	val := strings.TrimSpace(string(b))
	if val == "" {
		return "", fmt.Errorf("empty value")
	}
	return val, nil
}

// parseKV splits a string on the first occurrence of sep, trimming whitespace.
func parseKV(s, sep string) (key, value string, ok bool) {
	idx := strings.Index(s, sep)
	if idx < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(s[:idx])
	value = strings.TrimSpace(s[idx+len(sep):])
	if key == "" {
		return "", "", false
	}
	return key, value, true
}
