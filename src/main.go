package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
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
	Parent    *ComponentNode
}

var (
	createdComponents = make(map[string]Component)
	rootComponents    []*ComponentNode
	componentsMutex   sync.Mutex
)

func main() {
	// Check if repository is provided as command-line argument
	if len(os.Args) < 2 {
		log.Fatal("Please provide the repository in the format 'username/repo'")
	}

	// Parse repository information
	repoInfo := strings.Split(os.Args[1], "/")
	if len(repoInfo) != 2 {
		log.Fatal("Invalid repository format. Please use 'username/repo'")
	}
	owner, repo := repoInfo[0], repoInfo[1]

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

	var wg sync.WaitGroup
	errChan := make(chan error, len(dirContent))

	for _, content := range dirContent {
		wg.Add(1)
		go func(content *github.RepositoryContent) {
			defer wg.Done()
			var err error
			if *content.Type == "dir" {
				err = processDirectory(ctx, client, owner, repo, *content.Path)
			} else if *content.Type == "file" {
				err = processFile(ctx, client, owner, repo, *content.Path)
			}
			if err != nil {
				errChan <- err
			}
		}(content)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

func processDirectory(ctx context.Context, client *github.Client, owner, repo, path string) error {
	_, dirContent, _, err := client.Repositories.GetContents(ctx, owner, repo, path, nil)
	if err != nil {
		fmt.Printf("Warning: Error getting directory contents for %s: %v\n", path, err)
		return nil
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(dirContent))

	for _, content := range dirContent {
		wg.Add(1)
		go func(content *github.RepositoryContent) {
			defer wg.Done()
			var err error
			if *content.Type == "dir" {
				err = processDirectory(ctx, client, owner, repo, *content.Path)
			} else if *content.Type == "file" {
				err = processFile(ctx, client, owner, repo, *content.Path)
			}
			if err != nil {
				errChan <- err
			}
		}(content)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

func processFile(ctx context.Context, client *github.Client, owner, repo, path string) error {
	fileContent, _, _, err := client.Repositories.GetContents(ctx, owner, repo, path, nil)
	if err != nil {
		fmt.Printf("Warning: Error getting file contents for %s: %v\n", path, err)
		return nil
	}

	if fileContent.GetSize() > 1000000 { // Skip files larger than 1MB
		fmt.Printf("Warning: Skipping large file %s (size: %d bytes)\n", path, fileContent.GetSize())
		return nil
	}

	content, err := fileContent.GetContent()
	if err != nil {
		fmt.Printf("Warning: Error decoding content of %s: %v\n", path, err)
		return nil
	}

	extractComponents(content, path)
	return nil
}

func extractComponents(content, path string) {
	exportRegex := regexp.MustCompile(`(?m)^export\s+(default\s+)?(function|class|const)\s+(\w+)|export\s+const\s+(\w+)\s*=\s*(\(|\w+\s*=>)`)
	matches := exportRegex.FindAllStringSubmatch(content, -1)

	componentsMutex.Lock()
	defer componentsMutex.Unlock()

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

	var wg sync.WaitGroup
	errChan := make(chan error, len(componentNodes))

	for _, node := range componentNodes {
		wg.Add(1)
		go func(node *ComponentNode) {
			defer wg.Done()
			fileContent, _, _, err := client.Repositories.GetContents(ctx, owner, repo, node.Component.Path, nil)
			if err != nil {
				errChan <- fmt.Errorf("error reading file %s: %v", node.Component.Path, err)
				return
			}

			content, err := fileContent.GetContent()
			if err != nil {
				errChan <- fmt.Errorf("error decoding content of %s: %v", node.Component.Path, err)
				return
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
		}(node)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			fmt.Printf("Warning: %v\n", err)
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
	jsonData, err := json.MarshalIndent(rootComponents, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling JSON: %v\n", err)
		return
	}
	fmt.Println(string(jsonData))
}

func printComponentTree(nodes []*ComponentNode, depth int) {
	for _, node := range nodes {
		fmt.Printf("%s- %s (%s)\n", strings.Repeat("  ", depth), node.Component.Name, node.Component.Path)
		printComponentTree(node.Children, depth+1)
	}
}