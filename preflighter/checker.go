package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

/**
 * Runs the given item script and returns the stdount/stderr
 */
func runItemScript(item *ChecklistItem, runner *Runner) (string, string, error) {
	sout, serr, err := runner.Run(item.Script)
	if err != nil {
		if xerr, ok := err.(*exec.ExitError); ok {
			err = fmt.Errorf("Exited with %d", xerr.ExitCode())
		}
	}

	sout = strings.Trim(sout, "\r\n\t ")
	return sout, serr, err
}

func canCheckItem(item *ChecklistItem) bool {
	return item.ExpectScript != "" || item.ExpectMatch != ""
}

/**
 * Runs the item's automatic checks
 */
func checkItemValue(item *ChecklistItem, runner *Runner, value string) (bool, string, error) {
	// If there is a script, call-out to the given script to compute
	// if the result obtained is valid
	if item.ExpectScript != "" {
		_, serr, err := runner.RunWithValue(item.ExpectScript, value)
		if err != nil {
			if xerr, ok := err.(*exec.ExitError); ok {
				if xerr.ExitCode() != 0 {
					return false, serr, nil
				}
			}
			return false, serr, err
		}
		return true, serr, nil
	}

	// If there is a regular expression, check now
	if item.ExpectMatch != "" {
		re := regexp.MustCompile(item.ExpectMatch)
		return re.MatchString(value),
			fmt.Sprintf("      Regex: %s\nDon't match: \"%s\"\n", item.ExpectMatch, value),
			nil
	}

	return false, "No expect condition", nil
}

/**
 * Runs the item's automatic checks
 */
func runItemCheck(item *ChecklistItem, runner *Runner) (string, string, bool, error) {
	value, serr, err := runItemScript(item, runner)
	if err != nil {
		return "", "", false, err
	}

	ok, cserr, err := checkItemValue(item, runner, value)
	if err != nil {
		return "", "", false, err
	}
	if !ok {
		return value, cserr, false, nil
	}

	return value, serr, ok, nil
}
