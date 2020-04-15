package util

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type ChecklistItem struct {
	Title  string
	Script string

	ExpectMatch  string `yaml:"expect"`
	ExpectScript string `yaml:"expect_script"`

	RunbookID   string `yaml:"runbook_id"`
	RunbookStep string `yaml:"runbook_step"`
}

type Checklist = []ChecklistItem

type ChecklistFile struct {
	Title        string
	Checklist    Checklist
	Libs         []string
	Env          map[string]string `yaml:"vars"`
	RequireTools []string          `yaml:"require_tools"`
	RunbookSteps []string          `yaml:"runbook_steps"`
	Filename     string            `yaml:"-"`
}

func LoadChecklist(filename string) (*ChecklistFile, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("Could not read %s: %s", filename, err.Error())
	}

	var cf ChecklistFile
	err = yaml.Unmarshal(content, &cf)
	if err != nil {
		return nil, fmt.Errorf("Could not parse %s: %s", filename, err.Error())
	}

	cf.Filename = filename
	return &cf, nil
}
