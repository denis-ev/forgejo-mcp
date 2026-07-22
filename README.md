# Gitea/Forgejo MCP Server

> Turn AI into your code repository management assistant

A [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server that enables you to manage Gitea/Forgejo repositories through AI assistants like Claude, Gemini, and Copilot.

## 🍴 About This Fork

This is an actively maintained fork of [raohwork/forgejo-mcp](https://github.com/raohwork/forgejo-mcp). As of **2026-07-22**, the upstream repository has had no maintainer activity for an extended period (multiple community pull requests, including bug fixes, sitting unreviewed). This fork exists to keep the project moving: merging in vetted community contributions, fixing bugs found in day-to-day use, and tracking newer Forgejo API surface (this fork is developed and tested against **Forgejo v16.0.1**).

If upstream becomes active again, changes here may be proposed back; until then, treat this fork as the actively developed line.

## 🚀 Why Use Forgejo MCP Server?

If you want to:
- **Smart progress tracking**: Let AI help you track project progress and analyze bottlenecks
- **Automated issue categorization**: Automatically tag issue labels and set milestones based on content
- **Priority sorting**: Let AI analyze issue content to help prioritize tasks
- **Code review assistance**: Get AI suggestions and insights in Pull Requests
- **Project documentation organization**: Automatically organize Wiki documents and release notes

Then this tool is made for you!

## ✨ Supported Features

### Issue Management
- Create, edit, and view issues
- Add, remove, and replace labels
- Manage issue comments and attachments
- Set issue dependencies

### Project Organization
- Manage labels (create, edit, delete)
- Manage milestones (create, edit, delete)
- Repository search and listing

### Repository Browsing
- Read file contents and list directory entries at any ref (`get_file_contents`)
- List commits, optionally filtered by branch/SHA or path (`list_commits`)
- View a single commit's metadata and stats, optionally with its diff (`get_commit`)
- List and create branches (`list_branches`, `create_branch`)
- List and create tags (`list_tags`, `create_tag`)
- Read a commit's combined CI status and set commit statuses (`get_commit_status`, `create_commit_status`)

### Release Management
- Manage version releases
- Manage release attachments

### Pull Requests
- View, list, and create Pull Requests
- Merge Pull Requests (merge/rebase/rebase-merge/squash, optional auto-merge on checks) and check merge status
- Inspect a Pull Request's changed files (`get_pull_request_files`) and raw diff (`get_pull_request_diff`)
- List Pull Request reviews and inline review comments
- Create Pull Request reviews and reply to review comments

### Other Features
- Manage Wiki pages (create, edit, delete, list; handles both page titles and slugs)
- List a user's own/other users' repositories
- Forgejo/Gitea Actions:
  - View legacy Actions tasks (`list_action_tasks`)
  - List and inspect workflow runs (`list_action_runs`, `get_action_run`)
  - List jobs within a run (`list_action_run_jobs`)
  - Fetch raw job execution logs, including failure output (`get_action_job_logs`; requires a Forgejo version that exposes this endpoint, verified on v16.0.1+)

## 📦 Installation

### Method 1: Use docker (Recommended)

For STDIO mode, you can skip to **Usage** section.

For SSE/Streamable HTTP mode, you should run `forgejo-mcp` as server before configuring your MCP client.

Images for this fork are published to GitHub Container Registry (multi-arch: `linux/amd64`, `linux/arm64`):

```bash
docker run -p 8080:8080 -e FORGEJOMCP_TOKEN="my-forgejo-api-token" ghcr.io/denis-ev/forgejo-mcp http --address :8080 --server https://git.example.com
```

Available tags: `latest` and `vX.Y.Z` (tagged releases), `master` (latest commit on the default branch).

### Method 2: Install from source

```bash
go install github.com/denis-ev/forgejo-mcp@latest
```

### Method 3: Download Pre-compiled Binaries

Download the appropriate version for your operating system from the [Releases page](https://github.com/denis-ev/forgejo-mcp/releases).

## 🖥️ Usage

This tool provides two primary modes of operation: `stdio` for local integration and `http` for remote access.

Before actually setup you MCP client, you have to create an access token on the Forgejo/Gitea server.

1. Log in to your Forgejo/Gitea instance
2. Go to **Settings** → **Applications** → **Access Tokens**
3. Click **Generate New Token**
4. Select appropriate permission scopes (recommend at least `repository` and `issue` write permissions)
5. Copy the generated token

💡 **Tip**: For security, consider setting environment variables instead of using tokens directly in config:
```bash
export FORGEJOMCP_SERVER="https://your-forgejo-instance.com"
export FORGEJOMCP_TOKEN="your_access_token"
```

### Stdio Mode (for Local Clients)

This is the recommended mode for integrating with local AI assistant clients like Claude Desktop or Gemini CLI. It uses standard input/output for direct communication.

#### Configure Your AI Client

Using docker:

```json
{
  "mcpServers": {
    "forgejo": {
      "command": "docker",
      "args": [
        "--rm",
        "ghcr.io/denis-ev/forgejo-mcp",
        "stdio",
        "--server", "https://your-forgejo-instance.com",
        "--token", "your_access_token"
      ]
    }
  }
}
```

Installed from source or pre-built binary:

```json
{
  "mcpServers": {
    "forgejo": {
      "command": "/path/to/forgejo-mcp",
      "args": [
        "stdio",
        "--server", "https://your-forgejo-instance.com",
        "--token", "your_access_token"
      ]
    }
  }
}
```

You might want to take a look at **Security Recommendations** section for best practice.

### HTTP Server Mode (for Remote Access)

This mode starts a web server, allowing remote clients to connect via HTTP. It's ideal for web-based services or setting up a central gateway for multiple users.

Run the following command to start the server:
```bash
# with local binary
/path/to/forgejo-mcp http --address :8080 --server https://your-forgejo-instance.com

# with docker
docker run -p 8080:8080 -d --rm ghcr.io/denis-ev/forgejo-mcp http --address :8080 --server https://your-forgejo-instance.com
```

The server supports two operational modes:
- **Single-user mode**: If you provide a `--token` (or environment variable `FORGEJOMCP_TOKEN`) at startup, all operations will use that token.
  ```bash
  forgejo-mcp http --address :8080 --server https://git.example.com --token your_token
  ```
- **Multi-user mode**: If no token is provided, the server requires clients to send an `Authorization: Bearer <token>` header with each request, allowing it to serve multiple users securely.

#### Client Configuration

For clients that support connecting to a remote MCP server via HTTP, you can add a configuration like this. This example shows how to connect to a server running in multi-user mode:

```json
{
  "mcpServers": {
    "forgejo-remote": {
      "type": "sse",
      "url": "http://localhost:8080/sse",
      "headers": {
        "Authorization": "Bearer your_token"
      }
    }
  }
}
```

or `http` type (for Streamable HTTP, use different path in URL)

```json
{
  "mcpServers": {
    "forgejo-remote": {
      "type": "http",
      "url": "http://localhost:8080/",
      "headers": {
        "Authorization": "Bearer your_token"
      }
    }
  }
}
```

If connecting to a server in single-user mode, you can omit the `headers` field.

## 🛡️ Security Recommendations

1. **Use environment variables**: Set `FORGEJOMCP_SERVER` and `FORGEJOMCP_TOKEN`, then remove `--server` and `--token` from your configuration
2. **Limit token permissions**: Only grant necessary permission scopes
3. **Rotate tokens regularly**: Update access tokens periodically

## 📋 Usage Examples

After configuration, you can use natural language in your AI assistant to manage your repositories:

```
"Show me critical bug reports of this repo on my gitea server"

"According to our discussion above, create a detailed issue about this bug, then leave a comment on the issue to describe how we will fix it."

"Give me a report about current milestone. Recent progression in particular."

"Analyze recent pull requests and tell me which ones need priority review"
```

## 🤝 Support & Contributing

- **Bug Reports**: [GitHub Issues](https://github.com/denis-ev/forgejo-mcp/issues) (this fork) or [upstream Issues](https://github.com/raohwork/forgejo-mcp/issues)
- **Code Contributions**: Pull Requests are welcome, either here or upstream!

## 📄 License

This project is licensed under the [Mozilla Public License 2.0](LICENSE).

---

**Start making AI your code repository management partner!** 🚀
