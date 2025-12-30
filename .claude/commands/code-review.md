# Code Review

Perform a comprehensive code review as a senior Go developer with 10+ years of experience in web application development.

## Preliminary Check (FIRST STEP - before starting review)

**IMPORTANT**: Always perform this check BEFORE starting the review process.

1. **Get current branch name**:
   ```bash
   git branch --show-current
   ```

2. **Check for existing review report**:
   - Check if file `codereview-{current-branch}.md` exists in project root
   - Use the Read tool or Bash to check: `[ -f "codereview-{branch-name}.md" ] && echo "EXISTS" || echo "NOT_FOUND"`

3. **Determine review type**:
   - If report file EXISTS → This is a **RE-REVIEW** (proceed to "Re-Review Instructions" section)
   - If report file NOT_FOUND → This is an **INITIAL REVIEW** (proceed to standard review in "Instructions" section)

## Re-Review Instructions (use ONLY if existing report found)

When performing a **re-review** (existing report found):

### Step 1: Load Previous Report

1. **Read the existing report**:
   - Read `codereview-{branch-name}.md` file completely
   - Extract all issues from previous review organized by category:
     - Critical Issues (Критические проблемы)
     - Major Issues (Важные проблемы)
     - Minor Issues (Незначительные проблемы)
   - Note the issue numbers (e.g., #1, #2, #3) and full descriptions
   - Note the file locations mentioned (e.g., `internal/server/server.go:40`)

### Step 2: Analyze Current Code State

1. **Get current code changes**:
   ```bash
   git diff main...HEAD
   ```

2. **For EACH issue from previous report**:
   - Read the specific file and lines mentioned in the issue
   - Compare with the issue description
   - Determine if the issue is FIXED or NOT FIXED

### Step 3: Verify If Issue Is Fixed

For each issue, mark it as **FIXED** if:
- The code file/lines mentioned in the issue have been changed
- The problematic pattern described is no longer present
- The fix follows project conventions from `.agent/quick_load.toon`
- The fix matches or is similar to the suggested solution

Mark as **NOT FIXED** if:
- Code has not changed at the mentioned location
- The problem still exists in the code
- Changes were made but don't address the issue
- The issue was moved to another location

### Step 4: Identify New Issues

1. **Perform standard code review** on current changes (follow the checklist in "Instructions" section)
2. **Look for NEW problems** not mentioned in the previous report
3. **Compare with previous issues** to avoid duplicates

### Step 5: Generate Updated Report

Create an updated report with the following structure:

```markdown
# Отчет о Code Review

**Ветка:** {branch-name}
**Базовая ветка:** main
**Ревьюер:** Senior Go Developer (AI Assistant)
**Дата:** {current-date}

---

## 🔄 Повторное ревью

**Дата первичного ревью**: {date-from-previous-report}
**Дата повторного ревью**: {current-date}

### Изменения с момента последнего ревью

**Статистика**:
- ✅ Исправлено проблем: X
- ⚠️ Осталось проблем: Y
- 🆕 Новых проблем: Z

### Исправленные проблемы

{List of FIXED issues with strikethrough and brief description}
1. ~~Критическая проблема #1~~ - {brief description of what was fixed}
2. ~~Важная проблема #6~~ - {brief description of what was fixed}
...

### Оставшиеся проблемы

Следующие проблемы из предыдущего ревью всё ещё не исправлены:
- Критическая проблема #2: {title}
- Критическая проблема #3: {title}
...

### Новые проблемы

Обнаружены следующие новые проблемы:
- Критическая проблема #{new-number}: {title}
...

---

{Continue with standard report structure for remaining and new issues}

## Краткое описание
...

## Статистика
{Update statistics based on remaining + new issues}

## Критические проблемы ⚠️
{ONLY remaining and new critical issues with their original/new numbers}

## Важные проблемы
{ONLY remaining and new major issues}

## Незначительные проблемы
{ONLY remaining and new minor issues}

## Положительные моменты ✓
{Update based on what was fixed + what's still good}

## План действий
{Update action items for remaining and new issues}

## Общая оценка
{Update overall assessment based on current state}
```

### Important Re-Review Notes

- **ALWAYS read the actual code** to verify if issue is fixed - don't assume
- **Be thorough**: an issue is fixed ONLY if the code actually changed appropriately
- **New issues can appear** due to fixes - check for that
- **Update statistics accurately** based on actual count of remaining/new issues
- **Preserve issue numbers**: Remaining issues keep original numbers, new issues get new numbers
- **Remove fixed issues completely** from the main sections (they only appear in "Исправленные проблемы")

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

5. **Save report** to: codereview-{branch-name}.md
   - For **INITIAL REVIEW**: Create new file with full report in Russian
   - For **RE-REVIEW**: Overwrite existing file with updated report in Russian
   - Ensure all fixed issues are completely removed from main sections
   - Ensure remaining and new issues are properly numbered
   - Preserve context from original report for remaining issues

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

## Examples

### Example: Determining If Issue Is Fixed

**Original Issue**:
```
### 4. **Игнорируется ошибка от `zap.NewProduction()`** (Правило #2)

**Местоположение:** `cmd/maintmode/main.go:23`

```go
logger, _ := zap.NewProduction()
```

**Проблема:** Ошибка от `zap.NewProduction()` игнорируется
```

**To verify if fixed**:
1. Read `cmd/maintmode/main.go` lines around 23
2. Check if code changed from `logger, _ := zap.NewProduction()` to something that handles error
3. If found: `logger, err := zap.NewProduction(); if err != nil { ... }` → **FIXED**
4. If still `logger, _ := ...` → **NOT FIXED**

### Example: Issue Numbering in Re-Review

**Before** (5 issues total):
- #1 Missing GoDoc (Critical)
- #2 Goroutine leak (Critical)
- #3 Hardcoded credentials (Critical)
- #4 Ignored error (Critical)
- #5 Commented code (Critical)

**After fixes** (#1, #4, #5 fixed):
- Исправленные: #1, #4, #5
- Оставшиеся: #2, #3
- Новые: #6 (if any new issue found)

**Report will show**:
- Критические проблемы: #2, #3, #6
- NOT: #1, #4, #5 (they appear only in "Исправленные проблемы")

## Important Notes

**For ALL reviews**:
- Be thorough but constructive
- Provide specific examples and locations (file:line)
- Suggest concrete improvements
- Consider project context and conventions
- Focus on maintainability and reliability
- Balance perfectionism with pragmatism

**For RE-REVIEWS specifically**:
- **READ the code** - don't assume issues are fixed without verification
- **Check every issue** from previous report individually
- **Look for new issues** - fixes can introduce new problems
- **Update statistics** accurately (count actual issues, don't estimate)
- **Preserve context** - keep detailed descriptions for remaining issues
- **Be consistent** with issue numbering (keep original numbers for remaining)
- **Document fixes** briefly in "Исправленные проблемы" section
- **Multiple re-reviews** - each re-review compares against the most recent report
