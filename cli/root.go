// Package cli builds the tarsier command tree and its configuration layering.
package cli

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	envPrefix  = "TARSIER"
	configName = "tarsier"
)

const (
	formatText = "text"
	formatJSON = "json"
	formatHTML = "html"
)

// NewRootCommand builds the command tree. Settings resolve flag > env >
// config file > default.
func NewRootCommand() *cobra.Command {
	var configFile string
	v := viper.New()

	root := &cobra.Command{
		Use:   "tarsier",
		Short: "Critical-path observability auditor",
		Long: "tarsier finds observability gaps in microservices and maps them to\n" +
			"named business-critical workflows.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if err := loadConfig(v, configFile); err != nil {
				return err
			}
			if err := applyConfig(cmd.Flags(), v); err != nil {
				return err
			}
			return setupLogger(cmd.Flags())
		},
	}

	f := root.PersistentFlags()
	f.StringVar(&configFile, "config", "", "config file (default: ./"+configName+".yaml)")
	f.String("log-level", "info", "log level: debug, info, warn, error")
	f.String("output", formatText, "output format: text, json, html")

	root.AddCommand(newVersionCommand(), newScanCommand(), newCheckCommand(), newCheckCardinalityCommand())
	return root
}

func loadConfig(v *viper.Viper, configFile string) error {
	v.SetEnvPrefix(envPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	v.AutomaticEnv()

	if configFile != "" {
		v.SetConfigFile(configFile)
	} else {
		v.SetConfigName(configName)
		v.AddConfigPath(".")
	}

	var notFound viper.ConfigFileNotFoundError
	switch err := v.ReadInConfig(); {
	case err == nil, errors.As(err, &notFound):
		return nil
	case configFile == "" && errors.Is(err, os.ErrNotExist):
		return nil
	default:
		return fmt.Errorf("read config: %w", err)
	}
}

func applyConfig(flags *pflag.FlagSet, v *viper.Viper) error {
	var err error
	flags.VisitAll(func(f *pflag.Flag) {
		if err != nil || f.Changed || !v.IsSet(f.Name) {
			return
		}
		if setErr := flags.Set(f.Name, fmt.Sprintf("%v", v.Get(f.Name))); setErr != nil {
			err = fmt.Errorf("config value for --%s: %w", f.Name, setErr)
		}
	})
	return err
}

func setupLogger(flags *pflag.FlagSet) error {
	levelName, _ := flags.GetString("log-level")
	format, _ := flags.GetString("output")

	var level slog.Level
	if err := level.UnmarshalText([]byte(levelName)); err != nil {
		return fmt.Errorf("invalid --log-level %q: want debug, info, warn or error", levelName)
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	switch format {
	case formatJSON:
		handler = slog.NewJSONHandler(os.Stderr, opts)
	case formatText, formatHTML, "":
		handler = slog.NewTextHandler(os.Stderr, opts)
	default:
		return fmt.Errorf("invalid --output %q: want text, json or html", format)
	}
	slog.SetDefault(slog.New(handler))
	return nil
}
