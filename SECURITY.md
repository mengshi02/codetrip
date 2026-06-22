# Security Policy

## Supported Versions

| Version | Supported |
| ------- | --------- |
| 0.1.x   | Yes       |

## Reporting a Vulnerability

We take security vulnerabilities seriously. If you discover a security issue in codetrip, please report it responsibly.

**Do not open a public GitHub issue.**

Instead, please:

1. Email security reports to the maintainers via GitHub's private vulnerability reporting feature
2. Include a clear description of the vulnerability, affected components, and steps to reproduce
3. Allow reasonable time for a response before any public disclosure

## Security Considerations

### Embedding Endpoints

codetrip supports connecting to external embedding API endpoints via HTTP. Be aware that:

- API keys passed via `--api-key` or environment variables may appear in process listings
- Prefer using environment variables over CLI flags for sensitive credentials
- Ensure embedding endpoints are trusted and use HTTPS

### MCP Server

The MCP server communicates over stdio. When using codetrip as an MCP tool server:

- The server has read access to all indexed repository data
- Ensure the trip directory (`~/.codetrip`) has appropriate file permissions
- The MCP server does not expose network ports by default

### Data Storage

codetrip stores indexed data locally in the trip directory using Pebble:

- No data is sent to external services unless you configure embedding endpoints
- Index data reflects your source code structure — treat it with the same sensitivity as source code
- The trip directory should be protected with appropriate filesystem permissions

## Response Timeline

- **Acknowledgment**: Within 48 hours
- **Initial Assessment**: Within 5 business days
- **Fix / Mitigation**: Depends on severity, typically within 14 days for critical issues