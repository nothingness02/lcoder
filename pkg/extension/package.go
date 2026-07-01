package extension

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Package represents an installable package containing agents/skills/tools.
type Package struct {
	Info    PackageInfo
	RootDir string
}

// PackageInfo is the metadata from lcoder-package.yaml.
type PackageInfo struct {
	Name         string         `yaml:"name"`
	Version      string         `yaml:"version"`
	Author       string         `yaml:"author"`
	Description  string         `yaml:"description"`
	ConfigSchema map[string]any `yaml:"config_schema"`
}

// LoadPackage reads a package directory and validates its metadata.
func LoadPackage(dir string) (*Package, error) {
	infoPath := filepath.Join(dir, "lcoder-package.yaml")
	if _, err := os.Stat(infoPath); err != nil {
		infoPath = filepath.Join(dir, "lcoder-package.yml")
		if _, err := os.Stat(infoPath); err != nil {
			return nil, fmt.Errorf("package metadata not found in %s", dir)
		}
	}
	data, err := os.ReadFile(infoPath)
	if err != nil {
		return nil, err
	}
	var info PackageInfo
	if err := yaml.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("invalid package metadata: %w", err)
	}
	if info.Name == "" {
		return nil, fmt.Errorf("package name is required")
	}
	return &Package{Info: info, RootDir: dir}, nil
}

// HasAgents reports whether the package includes agent mode definitions.
func (p *Package) HasAgents() bool {
	_, err := os.Stat(filepath.Join(p.RootDir, "agents"))
	return err == nil
}

// HasSkills reports whether the package includes skills.
func (p *Package) HasSkills() bool {
	_, err := os.Stat(filepath.Join(p.RootDir, "skills"))
	return err == nil
}

// HasTools reports whether the package includes tools.
func (p *Package) HasTools() bool {
	_, err := os.Stat(filepath.Join(p.RootDir, "tools"))
	return err == nil
}

// AgentDir returns the package agents directory.
func (p *Package) AgentDir() string {
	return filepath.Join(p.RootDir, "agents")
}

// SkillDir returns the package skills directory.
func (p *Package) SkillDir() string {
	return filepath.Join(p.RootDir, "skills")
}

// ToolDir returns the package tools directory.
func (p *Package) ToolDir() string {
	return filepath.Join(p.RootDir, "tools")
}
