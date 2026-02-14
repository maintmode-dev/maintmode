# Troubleshooting PR Creation

Common issues and their solutions when creating pull requests.

## No Commits Ahead of Main

**Problem**: Branch has no new commits to submit.

```bash
$ git log origin/main..HEAD
# No output
```

**Solutions**:
1. Check if you're on the correct branch:
   ```bash
   git branch --show-current
   ```

2. Check if you meant to work on a different branch:
   ```bash
   git branch -a
   ```

3. Verify your changes weren't already merged:
   ```bash
   git log --oneline -10
   ```

## Branch Not Pushed

**Problem**: Remote doesn't have the branch.

```bash
$ gh pr create
fatal: The current branch has no upstream branch
```

**Solution**:
```bash
git push -u origin HEAD
```

## PR Already Exists

**Problem**: A PR for this branch already exists.

```bash
$ gh pr create
pull request create failed: a pull request for branch "feature-x" already exists
```

**Solutions**:

1. View existing PR:
   ```bash
   gh pr view
   ```

2. Update the existing PR by pushing more commits:
   ```bash
   git push origin HEAD
   ```

3. Edit PR details:
   ```bash
   gh pr edit
   ```

## Merge Conflicts

**Problem**: Branch conflicts with base branch.

```bash
$ git rebase origin/main
CONFLICT (content): Merge conflict in file.go
```

**Solution**:

1. View conflicting files:
   ```bash
   git status
   ```

2. Resolve conflicts in your editor:
   - Look for conflict markers: `<<<<<<<`, `=======`, `>>>>>>>`
   - Keep the correct code
   - Remove conflict markers

3. Mark as resolved:
   ```bash
   git add file.go
   ```

4. Continue rebase:
   ```bash
   git rebase --continue
   ```

5. If you want to abort:
   ```bash
   git rebase --abort
   ```

## Authentication Failed

**Problem**: Not authenticated with GitHub.

```bash
$ gh pr create
error: authentication required
```

**Solution**:
```bash
gh auth login
```

Follow the prompts to authenticate.

## gh CLI Not Installed

**Problem**: gh command not found.

**Solutions**:

macOS:
```bash
brew install gh
```

Linux:
```bash
# Debian/Ubuntu
sudo apt install gh

# Fedora
sudo dnf install gh
```

Windows:
```powershell
# Using winget
winget install --id GitHub.cli

# Using scoop
scoop install gh
```

Or download from: https://cli.github.com/

## Uncommitted Changes

**Problem**: Working directory has uncommitted changes.

```bash
$ git status
Changes not staged for commit:
  modified:   file.go
```

**Solutions**:

1. Commit them:
   ```bash
   git add .
   git commit -m "description"
   ```

2. Stash temporarily:
   ```bash
   git stash
   # ... create PR ...
   git stash pop
   ```

3. Discard them (use with caution):
   ```bash
   git restore .
   ```

## Cannot Push: Branch Protected

**Problem**: Direct push to protected branch not allowed.

```bash
$ git push origin main
remote: error: GH006: Protected branch update failed
```

**Solution**: You must create a PR instead. Switch to a feature branch:

```bash
git checkout -b feature/my-changes
git push -u origin feature/my-changes
gh pr create
```

## Failed CI Checks

**Problem**: PR created but CI checks are failing.

**Solutions**:

1. View check results:
   ```bash
   gh pr checks
   ```

2. Fix issues and push again:
   ```bash
   # Make fixes
   git add .
   git commit -m "fix: resolve CI issues"
   git push origin HEAD
   ```

3. Re-run failed checks (if transient failure):
   ```bash
   gh pr checks --watch
   ```

## Large Diff Warning

**Problem**: PR is very large and difficult to review.

**Best Practice**: Break into smaller PRs:

1. Create separate branches for logical chunks
2. Submit PRs incrementally
3. Wait for approval before next PR

Example workflow:
```bash
# First PR
git checkout -b feature/part-1
# ... make changes ...
gh pr create

# After first PR merged
git checkout main
git pull
git checkout -b feature/part-2
# ... more changes ...
gh pr create
```

## Can't Find Base Branch

**Problem**: Base branch doesn't exist or has unusual name.

**Solution**: Specify base explicitly:

```bash
gh pr create --base develop
```

Or check available branches:
```bash
git branch -r
```

## Rebase Conflicts Too Complex

**Problem**: Too many conflicts during rebase.

**Alternative Strategy**: Merge instead (creates merge commit):

```bash
git fetch origin
git merge origin/main
# Resolve conflicts
git add .
git commit
git push origin HEAD
```

Note: Check project guidelines - some prefer rebase, others allow merge commits.
