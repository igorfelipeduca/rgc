package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
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
	Parent    *ComponentNode
}

var createdComponents = make(map[string]Component)
var rootComponents []*ComponentNode

func main() {
	// Read GitHub token from environment variable
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		log.Fatal("GITHUB_TOKEN environment variable not set")
	}

	// Create an authenticated GitHub client
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	// Specify the repository owner and name
	owner := "igorfelipeduca"
	repo := "fictional-octo-broccoli"

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()

	// Process repository contents
	err := ProcessRepository(ctx, client, owner, repo)
	if err != nil {
		log.Fatalf("Error processing repository: %v", err)
	}

	// Build component tree
	err = BuildComponentTree(ctx, client, owner, repo)
	if err != nil {
		log.Fatalf("Error building component tree: %v", err)
	}

	// Print results
	PrintResults()

	fmt.Printf("\nRoot Components: %d\n", len(rootComponents))
}

func ProcessRepository(ctx context.Context, client *github.Client, owner, repo string) error {
	_, dirContent, _, err := client.Repositories.GetContents(ctx, owner, repo, "", nil)
	if err != nil {
		return fmt.Errorf("error getting repository contents: %v", err)
	}

	for _, content := range dirContent {
		if *content.Type == "dir" {
			err = processDirectory(ctx, client, owner, repo, *content.Path)
			if err != nil {
				return err
			}
		} else if *content.Type == "file" {
			err = processFile(ctx, client, owner, repo, *content.Path)
			if err != nil {
				return err
			}
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
			err = processDirectory(ctx, client, owner, repo, *content.Path)
			if err != nil {
				return err
			}
		} else if *content.Type == "file" {
			err = processFile(ctx, client, owner, repo, *content.Path)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func processFile(ctx context.Context, client *github.Client, owner, repo, path string) error {
	fileContent, _, _, err := client.Repositories.GetContents(ctx, owner, repo, path, nil)
	if err != nil {
		return fmt.Errorf("error getting file contents: %v", err)
	}

	content, err := fileContent.GetContent()
	if err != nil {
		return fmt.Errorf("error decoding file contents: %v", err)
	}

	extractComponents(content, path)
	return nil
}

func extractComponents(content, path string) {
	exportRegex := regexp.MustCompile(`(?m)^export\s+(default\s+)?(function|class|const)\s+(\w+)|export\s+const\s+(\w+)\s*=\s*(\(|\w+\s*=>)`)
	matches := exportRegex.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		componentName := match[3]
		if componentName == "" {
			componentName = match[4]
		}
		if componentName != "" {
			createdComponents[componentName] = Component{Name: componentName, Path: path}
			fmt.Printf("Found component: %s in %s\n", componentName, path)
		}
	}

	funcRegex := regexp.MustCompile(`(?m)^function\s+(\w+)`)
	funcMatches := funcRegex.FindAllStringSubmatch(content, -1)

	for _, match := range funcMatches {
		componentName := match[1]
		if componentName != "" && createdComponents[componentName].Name == "" {
			createdComponents[componentName] = Component{Name: componentName, Path: path}
			fmt.Printf("Found function component: %s in %s\n", componentName, path)
		}
	}
}

func BuildComponentTree(ctx context.Context, client *github.Client, owner, repo string) error {
	componentNodes := make(map[string]*ComponentNode)

	for name, comp := range createdComponents {
		componentNodes[name] = &ComponentNode{Component: comp}
	}

	for _, node := range componentNodes {
		fileContent, _, _, err := client.Repositories.GetContents(ctx, owner, repo, node.Component.Path, nil)
		if err != nil {
			fmt.Printf("Warning: Error reading file %s: %v\n", node.Component.Path, err)
			continue
		}

		content, err := fileContent.GetContent()
		if err != nil {
			fmt.Printf("Warning: Error decoding content of %s: %v\n", node.Component.Path, err)
			continue
		}

		childRegex := regexp.MustCompile(`<([A-Z]\w+)[\s/>]`)
		matches := childRegex.FindAllStringSubmatch(content, -1)

		for _, match := range matches {
			childName := match[1]
			if childNode, exists := componentNodes[childName]; exists {
				node.Children = append(node.Children, childNode)
				childNode.Parent = node
			}
		}
	}

	for _, node := range componentNodes {
		if node.Parent == nil {
			rootComponents = append(rootComponents, node)
		}
	}

	return nil
}

func PrintResults() {
	fmt.Println("\nComponent Tree:")
	printComponentTree(rootComponents, 0)
}

func printComponentTree(nodes []*ComponentNode, depth int) {
	for _, node := range nodes {
		fmt.Printf("%s- %s (%s)\n", strings.Repeat("  ", depth), node.Component.Name, node.Component.Path)
		printComponentTree(node.Children, depth+1)
	}
}