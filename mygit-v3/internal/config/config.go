package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds key-value pairs from the config file.
type Config struct {
	data map[string]string
}

// globalConfigPath returns ~/.mygitconfig
func globalConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".mygitconfig")
}

// Load reads the global config file. Returns empty config if not found.
func Load() *Config {
	c := &Config{data: map[string]string{}}
	path := globalConfigPath()
	if path == "" {
		return c
	}
	f, err := os.Open(path)
	if err != nil {
		return c
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			c.data[key] = val
		}
	}
	return c
}

// Save writes config to ~/.mygitconfig
func (c *Config) Save() error {
	path := globalConfigPath()
	if path == "" {
		return fmt.Errorf("cannot determine home directory")
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	fmt.Fprintln(w, "# mygit config file")
	for k, v := range c.data {
		fmt.Fprintf(w, "%s = %s\n", k, v)
	}
	return w.Flush()
}

// Get retrieves a config value.
func (c *Config) Get(key string) string {
	return c.data[key]
}

// Set stores a config value.
func (c *Config) Set(key, value string) {
	c.data[key] = value
}

// AuthorName returns user.name from config, env, or default.
func (c *Config) AuthorName() string {
	if v := os.Getenv("GIT_AUTHOR_NAME"); v != "" {
		return v
	}
	if v := c.Get("user.name"); v != "" {
		return v
	}
	// Try to read from real git config
	if v := readGitConfig("user.name"); v != "" {
		return v
	}
	return "Developer"
}

// AuthorEmail returns user.email from config, env, or default.
func (c *Config) AuthorEmail() string {
	if v := os.Getenv("GIT_AUTHOR_EMAIL"); v != "" {
		return v
	}
	if v := c.Get("user.email"); v != "" {
		return v
	}
	if v := readGitConfig("user.email"); v != "" {
		return v
	}
	return "dev@example.com"
}

// readGitConfig reads a value from real git's global config.
func readGitConfig(key string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	// Try ~/.gitconfig
	path := filepath.Join(home, ".gitconfig")
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	// Simple parser for [user] section
	section := ""
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.Trim(line, "[]"))
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := section + "." + strings.TrimSpace(parts[0])
			if k == key {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}
