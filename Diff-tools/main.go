package main

import (
	"embed"
	"flag"
	"fmt"
	"os"

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
	Column   int
	Parent   *YAMLNode
	Children []*YAMLNode
}

func loadYAML(path string) (*YAMLNode, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var root interface{}
	dec := yaml.NewDecoder(
		yaml.WithMarkDecoder(
			yaml.WithMarkLocation(
				yaml.WithMarkLineColumn(),
			),
		),
	)
	if err := dec.Decode(content, &root); err != nil {
		return nil, err
	}

	return buildTree(root, nil, 1, 1), nil
}

func buildTree(node interface{}, parent *YAMLNode, line, column int) *YAMLNode {
	current := &YAMLNode{
		Value:  node,
		Line:   line,
		Column: column,
		Parent: parent,
	}

	if parent != nil {
		parent.Children = append(parent.Children, current)
	}

	if m, ok := node.(map[string]interface{}); ok {
		for k, v := range m {
			current.Children = append(current.Children, buildTree(v, current, line, column+2))
		}
	} else if arr, ok := node.([]interface{}); ok {
		for i, v := range arr {
			current.Children = append(current.Children, buildTree(v, current, line+i+1, column+2))
		}
	}

	return current
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "YAML Diff Tool v%s\n", version)
		fmt.Fprintf(os.Stderr, "Usage: %s [options] file1.yaml file2.yaml\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Options:")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nExamples:")
		fmt.Fprintln(os.Stderr, "  yamldiff config-old.yaml config-new.yaml")
		fmt.Fprintln(os.Stderr, "  yamldiff -html=report.html old.yaml new.yaml")
	}
	flag.Parse()

	if *showVersion {
		fmt.Printf("YAML Diff Tool v%s\n", version)
		os.Exit(0)
	}

	if len(flag.Args()) != 2 {
		flag.Usage()
		os.Exit(1)
	}

	file1, err := loadYAML(flag.Args()[0])
	if err != nil {
		fmt.Printf("Error loading %s: %v\n", flag.Args()[0], err)
		os.Exit(1)
	}
	file2, err := loadYAML(flag.Args()[1])
	if err != nil {
		fmt.Printf("Error loading %s: %v\n", flag.Args()[1], err)
		os.Exit(1)
	}

	var diffs []Diff
	compareNodes(file1, file2, "", &diffs)

	if *outputHTML != "" {
		if err := generateHTMLReport(diffs, *outputHTML); err != nil {
			fmt.Printf("Error generating HTML report: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("HTML report generated to %s\n", *outputHTML)
	}

	if *verbose {
		fmt.Printf("Found %d differences\n", len(diffs))
	}

	app := tview.NewApplication()
	flex := tview.NewFlex().SetDirection(tview.FlexRow)

	// Add header
	header := tview.NewTextView().
		SetDynamicColors(true).
		SetText(fmt.Sprintf("[white]Comparing [green]%s [white]and [green]%s [white]- Found [yellow]%d [white]differences", 
			flag.Args()[0], flag.Args()[1], len(diffs)))

	flex.AddItem(header, 1, 0, false)
	flex.AddItem(renderDiffs(diffs), 0, 1, false)

	if err := app.SetRoot(flex, true).Run(); err != nil {
		fmt.Printf("Error running application: %v\n", err)
		os.Exit(1)
	}
}

func loadYAML(path string) (*yaml.Node, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(content, &root); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return nil, fmt.Errorf("invalid YAML document structure")
	}

	return root.Content[0], nil
}

func compareNodes(oldNode, newNode *yaml.Node, path string, diffs *[]Diff) {
	if oldNode == nil && newNode == nil {
		return
	}

	if oldNode == nil {
		*diffs = append(*diffs, Diff{
			Type:     DiffAdd,
			Path:     path,
			NewValue: "entire subtree added",
			Line:     0,
		})
		return
	}

	if newNode == nil {
		*diffs = append(*diffs, Diff{
			Type:     DiffDelete,
			Path:     path,
			OldValue: "entire subtree removed",
			Line:     0,
		})
		return
	}

	if oldNode.Kind != newNode.Kind {
		*diffs = append(*diffs, Diff{
			Type:     DiffModify,
			Path:     path,
			OldValue: nodeKindToString(oldNode.Kind),
			NewValue: nodeKindToString(newNode.Kind),
			Line:     newNode.Line,
		})
		return
	}

	switch oldNode.Kind {
	case yaml.ScalarNode:
		oldVal, newVal := oldNode.Value, newNode.Value
		if *ignoreCase {
			oldVal = strings.ToLower(oldVal)
			newVal = strings.ToLower(newVal)
		}
		if oldVal != newVal {
			*diffs = append(*diffs, Diff{
				Type:     DiffModify,
				Path:     path,
				OldValue: oldNode.Value,
				NewValue: newNode.Value,
				Line:     newNode.Line,
			})
		}

	case yaml.MappingNode:
		oldKeys := make(map[string]*yaml.Node)
		for i := 0; i < len(oldNode.Content); i += 2 {
			key := oldNode.Content[i].Value
			if *ignoreCase {
				key = strings.ToLower(key)
			}
			oldKeys[key] = oldNode.Content[i+1]
		}

		for i := 0; i < len(newNode.Content); i += 2 {
			keyNode := newNode.Content[i]
			valueNode := newNode.Content[i+1]
			key := keyNode.Value
			if *ignoreCase {
				key = strings.ToLower(key)
			}

			newPath := path
			if path != "" {
				newPath += "."
			}
			newPath += keyNode.Value

			if oldVal, exists := oldKeys[key]; exists {
				compareNodes(oldVal, valueNode, newPath, diffs)
				delete(oldKeys, key)
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
			newPath := path
			if path != "" {
				newPath += "."
			}
			newPath += key

			*diffs = append(*diffs, Diff{
				Type:     DiffDelete,
				Path:     newPath,
				OldValue: oldVal.Value,
				Line:     oldVal.Line,
			})
		}

	case yaml.SequenceNode:
		maxLen := len(oldNode.Content)
		if len(newNode.Content) > maxLen {
			maxLen = len(newNode.Content)
		}

		for i := 0; i < maxLen; i++ {
			var oldChild, newChild *yaml.Node
			if i < len(oldNode.Content) {
				oldChild = oldNode.Content[i]
			}
			if i < len(newNode.Content) {
				newChild = newNode.Content[i]
			}

			newPath := fmt.Sprintf("%s[%d]", path, i)

			compareNodes(oldChild, newChild, newPath, diffs)
		}
	}
}

func nodeKindToString(kind yaml.Kind) string {
	switch kind {
	case yaml.DocumentNode:
		return "document"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.MappingNode:
		return "mapping"
	case yaml.ScalarNode:
		return "scalar"
	case yaml.AliasNode:
		return "alias"
	default:
		return "unknown"
	}
}

func renderDiffs(diffs []Diff) tview.Primitive {
	left := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true).
		SetTitle("Old Version").
		SetBorder(true)

	right := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true).
		SetTitle("New Version").
		SetBorder(true)

	for _, d := range diffs {
		switch d.Type {
		case DiffDelete:
			fmt.Fprintf(left, "[red]× %s (Line %d): [white]%s\n", d.Path, d.Line, d.OldValue)
		case DiffAdd:
			fmt.Fprintf(right, "[green]+ %s (Line %d): [white]%s\n", d.Path, d.Line, d.NewValue)
		case DiffModify:
			fmt.Fprintf(left, "[yellow]~ %s (Line %d): [white]%s\n", d.Path, d.Line, d.OldValue)
			fmt.Fprintf(right, "[yellow]~ %s (Line %d): [white]%s\n", d.Path, d.Line, d.NewValue)
		}
	}

	return tview.NewFlex().
		AddItem(left, 0, 1, false).
		AddItem(right, 0, 1, false)
}

func generateHTMLReport(diffs []Diff, outputPath string) error {
	if outputPath == "" {
		return fmt.Errorf("output path cannot be empty")
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	data, err := htmlTemplate.ReadFile("template.html")
	if err != nil {
		return fmt.Errorf("failed to read template: %w", err)
	}

	tmpl, err := template.New("report").Parse(string(data))
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	type TemplateDiff struct {
		Type      string
		Path      string
		OldValue  string
		NewValue  string
		Line      int
		HasOld    bool
		HasNew    bool
		IsModify  bool
		IsAdd     bool
		IsDelete  bool
	}

	var templateDiffs []TemplateDiff
	for _, d := range diffs {
		td := TemplateDiff{
			Path:     d.Path,
			OldValue: d.OldValue,
			NewValue: d.NewValue,
			Line:     d.Line,
			HasOld:   d.OldValue != "",
			HasNew:   d.NewValue != "",
		}

		switch d.Type {
		case DiffAdd:
			td.Type = "add"
			td.IsAdd = true
		case DiffDelete:
			td.Type = "delete"
			td.IsDelete = true
		case DiffModify:
			td.Type = "modify"
			td.IsModify = true
		}

		templateDiffs = append(templateDiffs, td)
	}

	reportData := struct {
		Diffs    []TemplateDiff
		File1    string
		File2    string
		DiffCount int
	}{
		Diffs:    templateDiffs,
		File1:    flag.Args()[0],
		File2:    flag.Args()[1],
		DiffCount: len(diffs),
	}

	var output bytes.Buffer
	if err := tmpl.Execute(&output, reportData); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	if err := os.WriteFile(outputPath, output.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	return nil
}