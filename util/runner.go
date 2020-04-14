package util

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
)

type Runner struct {
	CacheDir       string
	Config         *Config
	StderrCallback func(string)
}

func CreateRunner(c *Config) (*Runner, error) {
	var dir string
	var err error

	if c.UserTempDir == "" {
		dir, err = ioutil.TempDir("", "pcheck")
		if err != nil {
			return nil, fmt.Errorf("Could not create temp dir: %s", err.Error())
		}
	} else {
		dir = c.UserTempDir
		os.MkdirAll(dir, os.ModePerm)
	}

	return &Runner{
		CacheDir:       dir,
		Config:         c,
		StderrCallback: nil,
	}, nil
}

func (r *Runner) Cleanup() {
	if r.Config.UserTempDir == "" {
		os.RemoveAll(r.CacheDir)
	}
}

/**
 * Return a list of tools that are required, yet not found in path
 */
func (r *Runner) GetMissingTools() []string {
	var missing []string
	var tools []string = []string{
		"awk",
		"bash",
		"cat",
		"curl",
		"dcos",
		"jq",
		"shasum",
		"tr",
	}

	tools = append(tools, r.Config.UserTools...)

	for _, tool := range tools {
		_, err := exec.LookPath(tool)
		if err != nil {
			missing = append(missing, tool)
		}
	}

	return missing
}

/**
 * Execute the given script and collect stdout/stderr
 */
func (r *Runner) Run(script string) (string, string, error) {
	return r.RunWithValue(script, "")
}

/**
 * Execute the given script and collect stdout/stderr
 */
func (r *Runner) RunWithValue(script string, value string) (string, string, error) {
	cmd := exec.Command("bash")

	// Open I/O pipes
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", fmt.Errorf("Unable to open stdout pipe: %s", err.Error())
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", "", fmt.Errorf("Unable to open stdout pipe: %s", err.Error())
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", "", fmt.Errorf("Unable to open stdin pipe: %s", err.Error())
	}

	// Prepare environment
	list := r.Config.GetEnvList()
	list = append(list, fmt.Sprintf("CACHE_DIR=%s", r.CacheDir))
	if value != "" {
		list = append(list, fmt.Sprintf("VALUE=%s", value))
	}
	cmd.Env = append(os.Environ(), list...)

	err = cmd.Start()
	if err != nil {
		return "", "", fmt.Errorf("Unable to start process: %s", err.Error())
	}

	io.WriteString(stdin, fmt.Sprintf("%s\n%s\n%s", BashLibrary, r.Config.UserLib, script))
	stdin.Close()

	sserr := ""
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		sserr += line + "\n"
		if r.StderrCallback != nil {
			r.StderrCallback(line)
		}
	}
	stderr.Close()

	ssout, err := ioutil.ReadAll(stdout)
	if err != nil {
		return "", "", fmt.Errorf("Unable to read stdout: %s", err.Error())
	}
	stdout.Close()

	err = cmd.Wait()
	if err != nil {
		if xerr, ok := err.(*exec.ExitError); ok {
			return string(ssout), string(sserr), xerr
		}
		return "", "", fmt.Errorf("Execution error: %s", err.Error())
	}

	return string(ssout), sserr, nil
}
