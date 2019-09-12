package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/briandowns/spinner"
	. "github.com/logrusorgru/aurora"
)

const PENDING = 0
const PROMPT = 1
const ERROR = 2
const SUCCESS = 3
const SKIP = 4
const BLANK = 5

type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

func getWidth() uint {
	ws := &winsize{}
	retCode, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(syscall.Stdin),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)))

	if int(retCode) == -1 {
		panic(errno)
	}
	return uint(ws.Col)
}

type uxPendingMonitor struct {
	item          *ChecklistItem
	spinner       *spinner.Spinner
	started       time.Time
	expandTimeout time.Duration
	lines         []string
	expanded      bool
	lineCount     int
}

func createPendingMonitor(item *ChecklistItem, expandTimeout time.Duration) *uxPendingMonitor {
	sp := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	lineCount := 8

	return &uxPendingMonitor{
		item:          item,
		spinner:       sp,
		expandTimeout: expandTimeout,
		expanded:      false,
		lineCount:     lineCount,
		lines:         make([]string, lineCount),
	}
}

func (m *uxPendingMonitor) Start() {
	printLine(PENDING, m.item.Title, "", "")
	m.spinner.Start()
	m.started = time.Now()
}

func (m *uxPendingMonitor) Stop() {
	if m.expanded {
		m.collapseLines()
		m.expanded = false
	}
	m.spinner.Stop()
}

func (m *uxPendingMonitor) HandleLine(line string) {
	// Shift liens and collect the new line
	for i := 0; i < m.lineCount-1; i++ {
		m.lines[i] = m.lines[i+1]
	}
	m.lines[m.lineCount-1] = line

	// If it's time, expand now
	if !m.expanded && ((time.Now().Sub(m.started)) > m.expandTimeout) {
		m.expand()
	} else if m.expanded {
		m.redrawLines()
	}
}

func (m *uxPendingMonitor) collapseLines() {
	for i := 0; i < 3+m.lineCount; i++ {
		fmt.Printf("\r\x1B[K\n")
	}
	fmt.Printf("\x1B[%dA\r", 3+m.lineCount)
}

func (m *uxPendingMonitor) printLines() {
	fmt.Println()
	fmt.Println(Bold("     ╒ Progress"))
	for _, line := range m.lines {
		fmt.Println(Bold("     │ "), line)
	}
	fmt.Println(Bold("     ╘ ∙∙∙"))
}

func (m *uxPendingMonitor) redrawLines() {
	m.spinner.Lock()
	m.collapseLines()
	m.printLines()
	// Focus on the top line
	fmt.Printf("\x1B[%dA\r", 3+m.lineCount)
	printLine(PENDING, m.item.Title, "", "")
	fmt.Printf("| ")
	m.spinner.Unlock()
}

func (m *uxPendingMonitor) expand() {
	if m.expanded {
		return
	}

	// Allocate space and render the first expansion
	m.expanded = true
	m.printLines()
	fmt.Printf("\x1B[%dA\r", 3+m.lineCount)
	printLine(PENDING, m.item.Title, "", "")
}

func readChar() string {
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	return strings.Trim(text, "\r\n\t ")
}

func rewindLine() {
	fmt.Printf("\r\x1B[K")
}

func printLine(status int, title string, value interface{}, prompt string) {
	icon := " "
	wrapText := func(v interface{}) interface{} { return v }

	switch status {
	case PENDING:
		icon = "⏳"
	case PROMPT:
		icon = "❔"
	case ERROR:
		icon = "❗️"
		wrapText = func(v interface{}) interface{} { return Bold(Red(v)) }
	case SUCCESS:
		icon = "✅"
		wrapText = func(v interface{}) interface{} { return Bold(Green(v)) }
	case SKIP:
		wrapText = func(v interface{}) interface{} { return Yellow(v) }
	}

	fmt.Printf("  %s  %-35s : ", icon, wrapText(title))
	if value != "" || prompt != "" {
		fmt.Printf("%-60s", wrapText(value))
	}
	if prompt != "" {
		fmt.Printf(" : %s", wrapText(prompt))
	}
}

func printBlock(block string, title string) {
	fmt.Println(Bold("     ╒ " + title))
	lines := strings.Split(block, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		fmt.Println(Bold("     │ "), line)
	}
	fmt.Println(Bold("     ╘ ●"))
}

func uxPrintError(err error) {
	fmt.Println(Bold(Red("ERROR:")), Bold(White(err.Error())))
}

func uxBlankItem(item *ChecklistItem) {
	printLine(BLANK, item.Title, "---", "---")
	fmt.Println()
}

func uxSkipItem(item *ChecklistItem, reason string) {
	printLine(SKIP, item.Title, "---", reason)
	fmt.Println()
}

func uxPassItem(item *ChecklistItem, value string) {
	printLine(SUCCESS, item.Title, value, "PASS")
	fmt.Println()
}

func uxFailItem(item *ChecklistItem, value string, cerr string) {
	printLine(ERROR, item.Title, value, "FAIL")
	fmt.Println()
	printBlock(item.Script, "Script")
	printBlock(cerr, "Command Output")
	fmt.Println()
}

func uxCheckItem(item *ChecklistItem, runner *Runner) bool {
	for {
		moni := createPendingMonitor(item, 10*time.Second)
		moni.Start()
		runner.StderrCallback = moni.HandleLine
		sout, serr, err := runItemScript(item, runner)
		moni.Stop()
		if err != nil {
			rewindLine()
			printLine(ERROR, item.Title, err.Error(), "ERROR")
			fmt.Println()
			printBlock(item.Script, "Script")
			printBlock(sout+"\n"+serr, "Command Output")
			fmt.Println()
			fmt.Printf("   Do you want to re-try? [Y/n] ")

			c := readChar()
			fmt.Printf("\x1B[1A")
			rewindLine()

			switch c {
			case "N", "n":
				return false
			}
			continue
		}

		for {
			rewindLine()
			printLine(PROMPT, item.Title, Bold(sout), "OK? [Y/n/s/v] ")
			c := readChar()
			fmt.Printf("\x1B[1A")

			switch c {
			case "y", "Y", "":
				rewindLine()
				printLine(SUCCESS, item.Title, sout, "PASS")
				fmt.Println()
				return true

			case "s", "S":
				rewindLine()
				printLine(SKIP, item.Title, sout, "SKIP")
				fmt.Println()
				return true

			case "v", "V":
				fmt.Println()
				printBlock(item.Script, "Script")
				printBlock(serr, "Command Output")
				fmt.Println()
				continue

			case "n", "N":
				rewindLine()
				printLine(ERROR, item.Title, sout, "FAIL")
				fmt.Println()
				return false
			}
		}

	}
}
