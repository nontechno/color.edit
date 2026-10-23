package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"color.edit/internal/incident"
	"color.edit/spec"
)

func main() { os.Exit(execute(os.Args[1:])) }

func execute(args []string) (exitCode int) {
	reportDir := defaultReportDir()
	reporter := &incident.Reporter{Dir: reportDir, Backend: "color.edit"}
	defer func() {
		if recovered := recover(); recovered != nil {
			err := fmt.Errorf("panic: %v", recovered)
			path := reporter.Record("color editor crash", err, debug.Stack(), map[string]string{"component": "main"})
			fmt.Fprintf(os.Stderr, "color.edit crashed: %s\nincident report: %s\n", incident.Redact(err.Error()), path)
			exitCode = 2
		}
	}()
	options, err := parseArguments(args, os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		// Command-line mistakes are the caller's to fix; they are not incidents.
		var usage *usageError
		if errors.As(err, &usage) {
			if !usage.reported {
				fmt.Fprintf(os.Stderr, "color.edit: %s\nRun 'color.edit --help' for usage.\n", incident.Redact(err.Error()))
			}
			return 1
		}
		path := reporter.Record("parse arguments", err, nil, map[string]string{"phase": "startup"})
		fmt.Fprintf(os.Stderr, "color.edit: %s\nincident report: %s\n", incident.Redact(err.Error()), path)
		return 1
	}
	reporter.Dir = options.reportDir
	specification, err := spec.Load(options.specPath)
	if err != nil {
		path := reporter.Record("load preview specification", err, nil, map[string]string{"file": filepath.Base(options.specPath)})
		fmt.Fprintf(os.Stderr, "color.edit: %s\nincident report: %s\n", incident.Redact(err.Error()), path)
		return 1
	}
	document, err := loadAssignments(options.colorsPath, specification)
	if err != nil {
		path := reporter.Record("load color assignments", err, nil, map[string]string{"file": filepath.Base(options.colorsPath)})
		fmt.Fprintf(os.Stderr, "color.edit: %s\nincident report: %s\n", incident.Redact(err.Error()), path)
		return 1
	}
	application, err := newEditorApp(document, reporter)
	if err != nil {
		path := reporter.Record("initialize color editor", err, nil, map[string]string{"specification": specification.Name})
		fmt.Fprintf(os.Stderr, "color.edit: %s\nincident report: %s\n", incident.Redact(err.Error()), path)
		return 1
	}
	if err := application.run(); err != nil {
		path := reporter.Record("run color editor", err, debug.Stack(), map[string]string{"specification": specification.Name})
		fmt.Fprintf(os.Stderr, "color.edit: %s\nincident report: %s\n", incident.Redact(err.Error()), path)
		return 1
	}
	return 0
}

type cliOptions struct {
	specPath   string
	colorsPath string
	reportDir  string
}

// usageError is a command-line mistake. reported is true when the flag package
// has already printed the message and usage text, so it is not printed again.
type usageError struct {
	err      error
	reported bool
}

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

func parseArguments(args []string, output io.Writer) (cliOptions, error) {
	options := cliOptions{reportDir: defaultReportDir()}
	fs := flag.NewFlagSet("color.edit", flag.ContinueOnError)
	fs.SetOutput(output)
	fs.Usage = func() {
		fmt.Fprintln(output, "color.edit - application-independent terminal color editor")
		fmt.Fprintln(output)
		fmt.Fprintln(output, "Usage:")
		fmt.Fprintln(output, "  color.edit --spec <preview.json> --colors <assignments.json>")
		fmt.Fprintln(output)
		fmt.Fprintln(output, "Options:")
		fs.PrintDefaults()
	}
	fs.StringVar(&options.specPath, "spec", "", "declarative preview specification")
	fs.StringVar(&options.colorsPath, "colors", "", "JSON color-assignment file to edit")
	fs.StringVar(&options.reportDir, "report-dir", options.reportDir, "directory for sanitized incident reports")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return options, err
		}
		return options, &usageError{err: err, reported: true}
	}
	if len(fs.Args()) > 0 {
		return options, &usageError{err: fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))}
	}
	options.specPath = strings.TrimSpace(options.specPath)
	options.colorsPath = strings.TrimSpace(options.colorsPath)
	if options.specPath == "" || options.colorsPath == "" {
		return options, &usageError{err: errors.New("both --spec and --colors are required")}
	}
	return options, nil
}

func defaultReportDir() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return filepath.Join(os.TempDir(), "color.edit", "reports")
	}
	return filepath.Join(home, ".local", "state", "color.edit", "reports")
}
