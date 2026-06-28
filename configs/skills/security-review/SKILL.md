---
name: security-review
when_to_use: Review code for security vulnerabilities, injection risks, authentication, and authorization problems.
steps:
  - Read the requested file(s) in full.
  - Identify input validation, authentication, and injection risks.
  - Report findings with severity and suggested fixes.
examples:
  - "Review auth.ts for security issues"
  - "Check this API for SQL injection"
output_format: |
  - Summary
  - Findings (severity + description + fix)
---

# Security Review

Use this skill when the user asks for a security review of code, configuration, or architecture.
