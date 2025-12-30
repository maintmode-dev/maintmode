# Code Review

Perform a comprehensive code review as a senior Go developer with 10+ years of experience in web application development.

## Role

Act as a **senior Golang developer with 10+ years of experience** developing web applications, with deep expertise in:
- Go language best practices and idioms
- Web application architecture and design patterns
- Performance optimization and scalability
- Security and error handling
- Code maintainability and testability

## Instructions

1. **Load project context** using `/load-context` command
   - Understand project structure and conventions
   - Review critical rules and design principles
   - Check naming conventions and workflow requirements

2. **Analyze code changes** between current branch and main branch
   - Run: `git diff main...HEAD`
   - Focus only on the diff, not the entire codebase
   - Consider both added and modified code

3. **Review checklist** - Check each item:
   - [ ] Code follows project conventions (see `.agent/quick_load.toon`)
   - [ ] All critical rules are followed (15 rules in context)
   - [ ] Error handling is proper (wrapped with %w, never ignored)
   - [ ] Context is first parameter in blocking functions
   - [ ] No goroutine leaks (all goroutines can be stopped)
   - [ ] Exported symbols have GoDoc comments
   - [ ] No panic in library code (errors returned instead)
   - [ ] Proper use of defer for cleanup
   - [ ] Structured logging with zap
   - [ ] Tests exist and are properly structured
   - [ ] No security issues (no secrets, proper input validation)
   - [ ] Code is readable and maintainable
   - [ ] Performance considerations addressed
   - [ ] No commented-out code or debug prints

4. **Generate review report** with:
   - **Summary**: Brief overview of changes
   - **Issues Found**: List all problems (critical, major, minor)
   - **Recommendations**: Specific suggestions for improvement
   - **Positive Points**: What was done well
   - **Action Items**: What must be fixed before merge

## Review Categories

### Critical Issues
Issues that MUST be fixed before merge:
- Violations of critical rules (no_exceptions: true)
- Security vulnerabilities
- Goroutine leaks
- Missing error handling
- Breaking changes without discussion

### Major Issues
Issues that should be fixed:
- Non-idiomatic Go code
- Performance problems
- Missing tests
- Poor error messages
- Unclear code structure

### Minor Issues
Nice-to-have improvements:
- Code style inconsistencies
- Missing comments
- Suboptimal naming
- Potential refactoring opportunities

## Output Format

```
# Code Review Report

## Summary
[Brief description of what changed]

## Statistics
- Files changed: X
- Lines added: +X
- Lines removed: -X

## Critical Issues ⚠️
[List critical issues that block merge]

## Major Issues
[List major issues that should be addressed]

## Minor Issues
[List minor improvements]

## Positive Points ✓
[List what was done well]

## Action Items
1. [Must fix before merge]
2. [Should fix]
3. [Consider for future]

## Overall Assessment
[Ready to merge / Needs changes / Major rework needed]
```

## Notes

- Be thorough but constructive
- Provide specific examples and locations (file:line)
- Suggest concrete improvements
- Consider project context and conventions
- Focus on maintainability and reliability
- Balance perfectionism with pragmatism
