package main

import (
	"flag"
	"fmt"
	"os"

	. "github.com/logrusorgru/aurora"
)

func main() {
	fTempDir := flag.String("temp", "", "keep temporary files in the given directory")
	fSkipPtr := flag.Int("s", 0, "the number of items to skip")
	fListPtr := flag.Bool("l", false, "list the items and exit")
	fAutoPtr := flag.Bool("a", false, "run the tests unattended")
	flag.Parse()
	if len(flag.Args()) == 0 {
		uxPrintError(fmt.Errorf("Please specify one or more checklists to process"))
		return
	}

	config, err := CreateConfig()
	if err != nil {
		uxPrintError(err)
		return
	}

	if *fTempDir != "" {
		config.UserTempDir = *fTempDir
	}

	// Read the checklists from the given arguments
	var checklists []*ChecklistFile
	for _, fname := range flag.Args() {
		checklist, err := LoadChecklist(fname)
		if err != nil {
			uxPrintError(err)
			return
		}
		err = config.AddChecklistFile(checklist)
		if err != nil {
			uxPrintError(err)
			return
		}

		checklists = append(checklists, checklist)
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

	// Create the runner component that executes scripts in a well-prepared
	// environment.
	runner, err := CreateRunner(config)
	if err != nil {
		uxPrintError(err)
		return
	}

	// Check if all the required utilities exst
	missing := runner.getMissingTools()
	if len(missing) > 0 {
		uxPrintError(fmt.Errorf("There are missing binaries from your environment:"))
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
		uxBlankItem(&item)
	}
	for _, item := range allItems[*fSkipPtr:] {
		if failure {
			uxSkipItem(&item, "ABORTED")
		} else {

			if *fAutoPtr {
				// Perform passive checks if we are running in auto mode
				if !canCheckItem(&item) {
					uxSkipItem(&item, "NO CHECKS")
				} else {
					value, serr, ok, err := runItemCheck(&item, runner)
					if err != nil {
						uxFailItem(&item, err.Error(), serr)
						failure = true
					} else if !ok {
						uxFailItem(&item, value, serr)
						failure = true
					} else {
						uxPassItem(&item, value)
					}
				}

			} else {
				// Otherwise go through the UI
				if !uxCheckItem(&item, runner) {
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
