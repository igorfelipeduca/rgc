# RGC (React Garbage Collector)

RGC is a tool designed to analyze React component structures in GitHub repositories. It provides insights into the component hierarchy and relationships within a React project.

## Features

- Scans GitHub repositories for React components
- Builds a component tree to visualize component relationships
- Supports various file extensions (.js, .jsx, .ts, .tsx)
- Provides a REST API for easy integration

## Setup

1. Clone the repository
2. Install dependencies (Go modules)
3. Set up your GitHub Personal Access Token:
   - Create a token at https://github.com/settings/tokens
   - Export the token in your terminal:
     ```
     export GITHUB_TOKEN=your_token_here
     ```
   - Disclaimer: If you're running RGC on a Windows machine, you'll need to use `set` instead of `export`. So the command would be:
     ```
     set GITHUB_TOKEN=your_token_here
     ```
4. Run the application: `go run src/main.go`

The server will start on port 8080.

## API Usage

The application exposes a single endpoint:

- `POST /garbage`
  - Payload: `{ "username": "github_username", "repo": "repository_name" }`
  - Returns: A JSON object containing the component tree

## How It Works

1. The application receives a GitHub username and repository name
2. It fetches the repository contents using the GitHub API (authenticated with your personal token)
3. It scans all files and directories for React components
4. A component tree is built, showing the hierarchy and relationships
5. The result is returned as a JSON response

## GitHub Personal Access Token

A GitHub Personal Access Token is required to authenticate API requests and avoid rate limiting. To set up your token:

1. Go to GitHub Settings > Developer settings > Personal access tokens
2. Click "Generate new token"
3. Give it a name and select the "repo" scope
4. Copy the generated token
5. Export it in your terminal before running the application:
   ```
   export GITHUB_TOKEN=your_token_here
   ```
 - Disclaimer: If you're running RGC on a Windows machine, you'll need to use `set` instead of `export`. So the command would be:
  ```
  set GITHUB_TOKEN=your_token_here
  ```

Never share your token or commit it to version control. If you suspect your token has been compromised, revoke it immediately and generate a new one.

## Note

This tool is designed for analysis purposes and does not modify any code in the target repository.
