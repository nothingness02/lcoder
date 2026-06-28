package config

// HookConfig holds declarative hook configuration.
type HookConfig struct {
	Audit              AuditHookConfig              `yaml:"audit"`
	SensitiveFileCheck SensitiveFileCheckHookConfig `yaml:"sensitive_file_check"`
	BashDenylist       BashDenylistHookConfig       `yaml:"bash_denylist"`
}

// AuditHookConfig enables or disables audit logging.
type AuditHookConfig struct {
	Enabled bool `yaml:"enabled"`
}

// SensitiveFileCheckHookConfig blocks access to sensitive paths.
type SensitiveFileCheckHookConfig struct {
	Enabled  bool     `yaml:"enabled"`
	Patterns []string `yaml:"patterns"`
}

// BashDenylistHookConfig blocks dangerous bash substrings.
type BashDenylistHookConfig struct {
	Enabled  bool     `yaml:"enabled"`
	Patterns []string `yaml:"patterns"`
}

// ExtensionConfig describes a Go extension to load.
type ExtensionConfig struct {
	Name   string         `yaml:"name"`
	Source string         `yaml:"source"`
	Config map[string]any `yaml:"config"`
}

// PackageConfig describes an installed package containing modes/skills/tools.
type PackageConfig struct {
	Name   string         `yaml:"name"`
	Source string         `yaml:"source"`
	Path   string         `yaml:"path"`
	Config map[string]any `yaml:"config"`
}
