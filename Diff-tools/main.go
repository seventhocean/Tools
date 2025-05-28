package main

import (
	"flag"
	"fmt"
	"os"

	"bytes"
	"html/template"

	"github.com/rivo/tview"
	"gopkg.in/yaml.v3"
)

type DiffType int

const (
	DiffAdd DiffType = iota
	DiffDelete
	DiffModify
)

type Diff struct {
	Type     DiffType
	Path     string
	OldValue string
	NewValue string
	Line     int
}

func loadYAML(path string) (*yaml.Node, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var root yaml.Node
	if err := yaml.Unmarshal(content, &root); err != nil {
		return nil, err
	}
	return &root, nil
}

func compareNodes(oldNode, newNode *yaml.Node, path string, diffs *[]Diff) {
	if oldNode.Kind == yaml.MappingNode && newNode.Kind == yaml.MappingNode {
		oldKeys := make(map[string]*yaml.Node)
		for i := 0; i < len(oldNode.Content); i += 2 {
			key := oldNode.Content[i].Value
			oldKeys[key] = oldNode.Content[i+1]
		}

		for i := 0; i < len(newNode.Content); i += 2 {
			keyNode := newNode.Content[i]
			valueNode := newNode.Content[i+1]
			newPath := fmt.Sprintf("%s.%s", path, keyNode.Value)
			if path == "" {
				newPath = keyNode.Value
			}

			if oldVal, exists := oldKeys[keyNode.Value]; exists {
				if oldVal.Value != valueNode.Value {
					*diffs = append(*diffs, Diff{
						Type:     DiffModify,
						Path:     newPath,
						OldValue: oldVal.Value,
						NewValue: valueNode.Value,
						Line:     valueNode.Line,
					})
				}
				compareNodes(oldVal, valueNode, newPath, diffs)
				delete(oldKeys, keyNode.Value)
			} else {
				*diffs = append(*diffs, Diff{
					Type:     DiffAdd,
					Path:     newPath,
					NewValue: valueNode.Value,
					Line:     valueNode.Line,
				})
			}
		}

		for key, oldVal := range oldKeys {
			newPath := fmt.Sprintf("%s.%s", path, key)
			if path == "" {
				newPath = key
			}
			*diffs = append(*diffs, Diff{
				Type:     DiffDelete,
				Path:     newPath,
				OldValue: oldVal.Value,
				Line:     oldVal.Line,
			})
		}
	}
}

func renderDiffs(diffs []Diff) tview.Primitive {
	left := tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	right := tview.NewTextView().SetDynamicColors(true).SetWrap(true)

	for _, d := range diffs {
		switch d.Type {
		case DiffDelete:
			fmt.Fprintf(left, "[red]-%s (Line %d): %s\n", d.Path, d.Line, d.OldValue)
		case DiffAdd:
			fmt.Fprintf(right, "[green]+%s (Line %d): %s\n", d.Path, d.Line, d.NewValue)
		case DiffModify:
			fmt.Fprintf(left, "[yellow]~%s (Line %d): %s →\n", d.Path, d.Line, d.OldValue)
			fmt.Fprintf(right, "[yellow]~%s (Line %d):   → %s\n", d.Path, d.Line, d.NewValue)
		}
	}

	return tview.NewFlex().AddItem(left, 0, 1, false).AddItem(right, 0, 1, false)
}

func generateHTMLReport(diffs []Diff, outputPath string) error {
	//go:generate go-bindata -pkg main -o bindata.go template.html

import (
	"github.com/go-bindata/go-bindata/v3/bindata"
)

func generateHTMLReport(diffs []Diff, outputPath string) error {
	// 从嵌入资源中读取模板
	asset, err := bindata.Asset("template.html")
	if err != nil {
		return fmt.Errorf("failed to read embedded template: %w", err)
	}
	tmpl, err := template.New("report").Parse(string(asset))
	if err != nil {
		return err
	}

	type TemplateDiff struct {
		Type     string
		Path     string
		OldValue string
		NewValue string
		Line     int
	}

	var templateDiffs []TemplateDiff
	for _, d := range diffs {
		typeStr := ""
		switch d.Type {
		case DiffDelete:
			typeStr = "delete"
		case DiffAdd:
			typeStr = "add"
		case DiffModify:
			typeStr = "modify"
		}
		templateDiffs = append(templateDiffs, TemplateDiff{
			Type:     typeStr,
			Path:     d.Path,
			OldValue: d.OldValue,
			NewValue: d.NewValue,
			Line:     d.Line,
		})
	}

	tmpl, err := template.New("report").Parse(htmlTemplate)
	if err != nil {
		return err
	}

	output := &bytes.Buffer{}
	if err := tmpl.Execute(output, templateDiffs); err != nil {
		return err
	}

	return os.WriteFile(outputPath, output.Bytes(), 0644)
}

func main() {
	var (
		htmlOutput = flag.String("o", "", "HTML output file")
		ignoreWS   = flag.Bool("ignore-whitespace", false, "Ignore whitespace differences")
	)
	flag.Parse()

	if len(flag.Args()) != 2 {
		fmt.Println("Usage: yamldiff [options] file1.yaml file2.yaml")
		os.Exit(1)
	}

	file1, err := loadYAML(flag.Args()[0])
	if err != nil {
		fmt.Printf("Error loading file1: %v\n", err)
		os.Exit(1)
	}

	app := tview.NewApplication()
	flex := tview.NewFlex()

	flex.AddItem(tview.NewTextView().SetText("File1 Content"), 0, 1, false)
	flex.AddItem(tview.NewTextView().SetText("File2 Content"), 0, 1, false)

	if err := app.SetRoot(flex, true).Run(); err != nil {
		panic(err)
	}
}
