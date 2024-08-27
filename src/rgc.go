package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v39/github"
	"golang.org/x/oauth2"
)

type Component struct {
	Name string
	Path string
}

type ComponentNode struct {
	Component Component
	Children  []*ComponentNode
	Parent    *ComponentNode `json:"-"` // This will exclude Parent from JSON serialization
}

// Custom MarshalJSON method to handle circular references
func (cn *ComponentNode) MarshalJSON() ([]byte, error) {
	type Alias ComponentNode
	return json.Marshal(&struct {
		*Alias
		Children []*ComponentNode `json:"children,omitempty"`
	}{
		Alias:    (*Alias)(cn),
		Children: cn.Children,
	})
}

var (
	createdComponents = make(map[string]Component)
	rootComponents    []*ComponentNode
	componentsMutex   sync.Mutex
)

func ProcessRepository(username, repo string) ([]*ComponentNode, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN environment variable not set")
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	ctx, cancel := context.WithTimeout(ctx, 75*time.Second)
	defer cancel()

	err := processRepoContents(ctx, client, username, repo)
	if err != nil {
		return nil, fmt.Errorf("error processing repository: %v", err)
	}

	err = buildComponentTree(ctx, client, username, repo)
	if err != nil {
		return nil, fmt.Errorf("error building component tree: %v", err)
	}

	_, err = json.MarshalIndent(rootComponents, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("error marshaling JSON: %v", err)
	}

	return rootComponents, nil
}

func processRepoContents(ctx context.Context, client *github.Client, owner, repo string) error {
	_, dirContent, _, err := client.Repositories.GetContents(ctx, owner, repo, "", nil)
	if err != nil {
		return fmt.Errorf("error getting repository contents: %v", err)
	}

	for _, content := range dirContent {
		if *content.Type == "dir" {
			err := processDirectory(ctx, client, owner, repo, *content.Path)
			if err != nil {
				return err
			}
		} else if *content.Type == "file" {
			processFile(*content.Path)
		}
	}

	return nil
}

func processDirectory(ctx context.Context, client *github.Client, owner, repo, path string) error {
	_, dirContent, _, err := client.Repositories.GetContents(ctx, owner, repo, path, nil)
	if err != nil {
		return fmt.Errorf("error getting directory contents: %v", err)
	}

	for _, content := range dirContent {
		if *content.Type == "dir" {
			err := processDirectory(ctx, client, owner, repo, *content.Path)
			if err != nil {
				return err
			}
		} else if *content.Type == "file" {
			processFile(*content.Path)
		}
	}

	return nil
}

func processFile(path string) {
	componentsMutex.Lock()
	defer componentsMutex.Unlock()

	if isComponent(path) {
		name := extractComponentName(path)
		createdComponents[name] = Component{Name: name, Path: path}
	}
}

func isComponent(path string) bool {
	return strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".jsx") || strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx")
}

func extractComponentName(path string) string {
	parts := strings.Split(path, "/")
	fileName := parts[len(parts)-1]
	return strings.TrimSuffix(fileName, filepath.Ext(fileName))
}

func buildComponentTree(ctx context.Context, client *github.Client, owner, repo string) error {
	for _, component := range createdComponents {
		node := &ComponentNode{Component: component}
		content, _, _, err := client.Repositories.GetContents(ctx, owner, repo, component.Path, nil)
		if err != nil {
			return fmt.Errorf("error getting file contents: %v", err)
		}

		fileContent, err := content.GetContent()
		if err != nil {
			return fmt.Errorf("error decoding file contents: %v", err)
		}

		childComponents := findChildComponents(fileContent)
		for _, childName := range childComponents {
			if childComponent, ok := createdComponents[childName]; ok {
				childNode := &ComponentNode{Component: childComponent, Parent: node}
				node.Children = append(node.Children, childNode)
			}
		}

		if node.Parent == nil {
			rootComponents = append(rootComponents, node)
		}
	}

	return nil
}

func findChildComponents(content string) []string {
	var childComponents []string
	re := regexp.MustCompile(`import\s+(\w+)\s+from`)
	matches := re.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 {
			childComponents = append(childComponents, match[1])
		}
	}
	return childComponents
}
