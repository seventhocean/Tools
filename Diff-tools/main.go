package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/rivo/tview"
	"gopkg.in/yaml.v3"
)

type YAMLNode struct {
	Value    interface{}
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
