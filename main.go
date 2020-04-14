package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	. "github.com/logrusorgru/aurora"
	. "github.com/mesosphere-incubator/preflighter/util"
)

func main() {
	fTempDir := flag.String("temp", "", "keep temporary files in the given directory")
	fSkipPtr := flag.Int("s", 0, "the number of items to skip")
	fListPtr := flag.Bool("l", false, "list the items and exit")
	fAutoPtr := flag.Bool("a", false, "run the tests unattended")
	flag.Parse()
	if len(flag.Args()) == 0 {
		UxPrintError(fmt.Errorf("Please specify one or more checklists to process"))
		return
	}

	// Read the checklists from the given arguments
	var checklists []*ChecklistFile
	for _, fname := range flag.Args() {
		checklist, err := LoadChecklist(fname)
		if err != nil {
			UxPrintError(err)
			return
		}
		checklists = append(checklists, checklist)
	}

	// Check for required environment variables
	failed := false
	for _, file := range checklists {
		for key, value := range file.Env {
			if strings.HasPrefix(value, "${") {
				if len(value) < 3 {
					file.Env[key] = ""
					continue
				}

				cmd := value[2 : len(value)-1]
				out, err := exec.Command("bash", "-c", cmd).Output()
				if err != nil {
					failed = true
					UxPrintError(fmt.Errorf("Unable to execute '%s': %s", cmd, err.Error()))
				}

				file.Env[key] = strings.TrimRight(string(out), "\n\r\t ")

			} else if value == "<" {
				if os.Getenv(key) == "" {
					failed = true
					UxPrintError(fmt.Errorf("Missing required %s environment variable", key))
				}
			}
		}
	}
	if failed {
		os.Exit(1)
	}

	// Check if we should just list and exit
	if *fListPtr {
		i := 0
		for _, list := range checklists {
			fmt.Printf("In %s (%s):\n", list.Filename, list.Title)
			for _, item := range list.Checklist {
				i += 1
				fmt.Printf(" %2d. %s\n", i, item.Title)
			}
			fmt.Println()
		}
		fmt.Printf("%d items in total\n", i)
		os.Exit(0)
	}

	// Prepare configuration
	config, err := CreateConfig()
	if err != nil {
		UxPrintError(err)
		return
	}
	if *fTempDir != "" {
		config.UserTempDir = *fTempDir
	}
	for _, checklist := range checklists {
		err = config.AddChecklistFile(checklist)
		if err != nil {
			UxPrintError(err)
			return
		}
	}

	// Create the runner component that executes scripts in a well-prepared
	// environment.
	runner, err := CreateRunner(config)
	if err != nil {
		UxPrintError(err)
		return
	}

	// Check if all the required utilities exst
	missing := runner.GetMissingTools()
	if len(missing) > 0 {
		UxPrintError(fmt.Errorf("There are missing executables from your path:"))
		for _, name := range missing {
			fmt.Printf(" ‣ Did not find '%s'\n", name)
		}
		return
	}

	fmt.Println("==========================================")
	fmt.Printf(" %s Pre-Flight Checklist\n", checklists[0].Title)
	fmt.Println("==========================================")
	fmt.Println()

	var allItems []ChecklistItem
	for _, list := range checklists {
		for _, item := range list.Checklist {
			allItems = append(allItems, item)
		}
	}

	failure := false
	for _, item := range allItems[:*fSkipPtr] {
		UxBlankItem(&item)
	}
	for _, item := range allItems[*fSkipPtr:] {
		if failure {
			UxSkipItem(&item, "ABORTED")
		} else {

			if *fAutoPtr {
				// Perform passive checks if we are running in auto mode
				if !CanCheckItem(&item) {
					UxSkipItem(&item, "NO CHECKS")
				} else {
					value, serr, ok, err := RunItemCheck(&item, runner)
					if err != nil {
						UxFailItem(&item, err.Error(), serr)
						failure = true
					} else if !ok {
						UxFailItem(&item, value, serr)
						failure = true
					} else {
						UxPassItem(&item, value)
					}
				}

			} else {
				// Otherwise go through the UI
				if !UxCheckItem(&item, runner) {
					failure = true
				}
			}
		}
	}

	if failure {
		fmt.Println()
		fmt.Println("🚨 ", Bold(Red("There was a failed item. You are not clear to continue")))
		os.Exit(1)
	} else {
		fmt.Println()
		fmt.Println("🍺 ", Bold("All checks are passing. You are clear to continue"))
		os.Exit(0)
	}
}
