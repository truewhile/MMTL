package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// SaveDatabaseConfig updates or creates config.yaml with the specified database configuration.
func SaveDatabaseConfig(dbType, dsn string) error {
	configPath := "config.yaml"
	data := make(map[string]any)

	content, err := os.ReadFile(configPath)
	if err == nil {
		if err := yaml.Unmarshal(content, &data); err != nil {
			data = make(map[string]any)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read config.yaml: %w", err)
	}

	dbSection, ok := data["database"].(map[string]any)
	if !ok {
		dbSection = make(map[string]any)
	}
	dbSection["type"] = dbType
	dbSection["dsn"] = dsn
	data["database"] = dbSection

	out, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal config.yaml: %w", err)
	}

	if err := os.WriteFile(configPath, out, 0644); err != nil {
		return fmt.Errorf("write config.yaml: %w", err)
	}
	return nil
}
