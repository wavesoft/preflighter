package util

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
)

type Config struct {
	Env         map[string]string
	UserLib     string
	UserTools   []string
	UserTempDir string
}

func CreateConfig() (*Config, error) {
	config := &Config{
		Env:       make(map[string]string),
		UserLib:   "",
		UserTools: nil,
	}

	// Get the cluster URL
	out, err := exec.Command("dcos", "config", "show", "core.dcos_url").Output()
	if err != nil {
		return nil, fmt.Errorf("Could not get cluster: %s", err.Error())
	}
	config.Env["DCOS_URL"] = strings.Trim(string(out), "\r\n\t ")

	// Get the ACS token
	out, err = exec.Command("dcos", "config", "show", "core.dcos_acs_token").Output()
	if err != nil {
		return nil, fmt.Errorf("Could not get cluster: %s", err.Error())
	}
	config.Env["DCOS_ACS_TOKEN"] = strings.Trim(string(out), "\r\n\t ")

	return config, nil
}

func (c *Config) AddChecklistFile(f *ChecklistFile) error {
	// Collect environment variables
	if f.Env != nil {
		for name, value := range f.Env {
			c.Env[name] = value
		}
	}

	// Pre-load library scripts
	for _, lib := range f.Libs {
		content, err := ioutil.ReadFile(lib)
		if err != nil {
			return fmt.Errorf("Could not load library script %s: %s", lib, err.Error())
		}

		c.UserLib = fmt.Sprintf("%s\n%s", c.UserLib, string(content))
	}

	// Collect tools
	for _, tool := range f.RequireTools {
		c.UserTools = append(c.UserTools, tool)
	}
	return nil
}

func (c *Config) GetEnvList() []string {
	var list []string

	for k, v := range c.Env {
		list = append(list, fmt.Sprintf("%s=%s", k, v))
	}

	return list
}
