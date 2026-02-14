# Git Best Practices

Complete guide to commit hygiene, branch management, and preparing clean PRs.

## Commit Hygiene

### Atomic Commits

Each commit should represent a single logical change:

```bash
# Good - single logical change
git commit -m "Add user authentication endpoint"

# Bad - multiple unrelated changes
git commit -m "Add auth, fix typo, update deps"
```

### Clear Commit Messages

Follow conventional commit format when possible:

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`

Examples:
```
feat(auth): add JWT authentication
fix(api): resolve null pointer in user handler
docs(readme): update installation instructions
```

### No Merge Commits

Prefer rebasing over merging to keep history clean:

```bash
# Good - rebase
git fetch origin
git rebase origin/main

# Avoid - creates merge commits
git merge origin/main
```

## Branch Management

### Rebase on Latest Main

Before creating PR, ensure branch is up-to-date:

```bash
git fetch origin
git rebase origin/main
```

If conflicts occur, resolve them and continue:
```bash
# Resolve conflicts in your editor
git add .
git rebase --continue
```

### Interactive Rebase for Cleanup

If commits are messy, use interactive rebase:

```bash
git rebase -i origin/main
```

Common operations:
- `pick` - keep commit as-is
- `reword` - change commit message
- `squash` - combine with previous commit
- `fixup` - combine with previous, discard message
- `drop` - remove commit

Example:
```
pick abc1234 Add feature X
fixup def5678 Fix typo
fixup ghi9012 Fix another typo
pick jkl3456 Add tests for feature X
```

### Squashing Multiple Commits

When you have many small "WIP" commits, squash them:

```bash
# Squash last 3 commits
git rebase -i HEAD~3

# Or squash all commits since branching from main
git rebase -i origin/main
```

In the interactive editor, change `pick` to `squash` or `fixup` for commits to combine.

## Pushing Changes

### Initial Push

```bash
git push origin HEAD
```

Or set upstream:
```bash
git push -u origin HEAD
```

### After Rebase (Force Push)

After rebasing, you need to force push:

```bash
# Safer - prevents overwriting if remote changed
git push origin HEAD --force-with-lease

# Less safe - forces push regardless
git push origin HEAD --force
```

**Warning**: Only force push if you're the only one working on the branch.

## Branch Naming

Use descriptive, conventional names:

```bash
# Features
feature/user-authentication
feature/payment-integration

# Bug fixes
fix/null-pointer-users
fix/issue-123

# Hotfixes
hotfix/critical-security-patch

# Refactoring
refactor/simplify-auth

# Documentation
docs/update-readme
```

## Best Practices Summary

1. **One commit per logical change** - Makes review easier
2. **Descriptive commit messages** - Explain why, not just what
3. **Rebase, don't merge** - Keep history linear
4. **Clean up before PR** - Squash WIP commits
5. **Test before pushing** - Ensure code works
6. **Keep branches short-lived** - Merge quickly
7. **Sync with main regularly** - Avoid large conflicts
8. **Use force-with-lease** - Safer than force push
