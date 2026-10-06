package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/thinkerqaq/devtool/adapters/extensionloader"
	"github.com/thinkerqaq/devtool/core/host"
	"github.com/thinkerqaq/devtool/sdk/readiness"
)

var errProjectNotReady = errors.New("project is not ready")

type initSummary struct {
	Project string             `json:"project"`
	Root    string             `json:"root"`
	Ready   bool               `json:"ready"`
	Issues  int                `json:"issues"`
	Reports []readiness.Report `json:"reports"`
}

func runInit(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOutput := fs.Bool("json", false, "print machine-readable readiness report")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args())
	}

	h, err := host.OpenProject(ctx, "", extensionloader.Resolve)
	if err != nil {
		return err
	}
	defer h.Close()

	request := readiness.Request{
		Root:       h.Project.Root,
		Workspaces: projectCodeWorkspace(h).Workspaces,
	}
	entries := h.ReadinessProviders()
	reports := make([]readiness.Report, len(entries))

	var wg sync.WaitGroup
	for i := range entries {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			report, checkErr := entries[i].Checker.CheckReadiness(ctx, request)
			if strings.TrimSpace(report.Provider) == "" {
				report.Provider = entries[i].ExtensionID
			}
			if checkErr != nil {
				report.Ready = false
				report.Issues = append(report.Issues, readiness.Issue{
					Kind:     readiness.KindProviderUnavailable,
					Resource: entries[i].ExtensionID,
					Message:  checkErr.Error(),
				})
			}
			reports[i] = report
		}()
	}
	wg.Wait()

	summary := initSummary{
		Project: h.Project.Config.Project.Name,
		Root:    h.Project.Root,
		Ready:   true,
		Reports: reports,
	}
	for _, report := range reports {
		if !report.Ready {
			summary.Ready = false
		}
		summary.Issues += len(report.Issues)
	}

	if *jsonOutput {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(summary); err != nil {
			return err
		}
	} else {
		printInitSummary(out, summary)
	}
	if !summary.Ready {
		return errProjectNotReady
	}
	return nil
}

func printInitSummary(out io.Writer, summary initSummary) {
	fmt.Fprintln(out, "DevTool Init")
	fmt.Fprintf(out, "Project: %s\n", summary.Project)
	for _, report := range summary.Reports {
		if report.Ready {
			fmt.Fprintf(out, "✓ %s\n", report.Provider)
			continue
		}
		fmt.Fprintf(out, "✗ %s\n", report.Provider)
		for _, issue := range report.Issues {
			fmt.Fprintf(out, "  - [%s]", issue.Kind)
			if issue.Resource != "" {
				fmt.Fprintf(out, " %s:", issue.Resource)
			}
			fmt.Fprintf(out, " %s\n", issue.Message)
			if issue.Remediation != "" {
				fmt.Fprintf(out, "    remediation: %s\n", issue.Remediation)
			}
			for key, value := range issue.Details {
				fmt.Fprintf(out, "    %s: %s\n", key, value)
			}
		}
	}
	if summary.Ready {
		fmt.Fprintln(out, "READY")
		return
	}
	fmt.Fprintf(out, "NOT READY (%d issues)\n", summary.Issues)
}
