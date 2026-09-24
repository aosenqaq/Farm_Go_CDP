# HQ Farm Sync Exclusion Design

## Goal

Keep the complete `代理版打包配置/HQ Farm/` directory local so that its agent credentials, branding assets, and release artifacts do not participate in future Git synchronization.

## Scope

- Add `代理版打包配置/HQ Farm/` to the repository `.gitignore` file.
- Remove every currently tracked path below that directory from the Git index with `git rm -r --cached`.
- Preserve all local files. No file in the directory is deleted from disk.

## Behavior

After the change, Git treats the directory as ignored. The next commit removes the already tracked entries from the repository, and later agent builds do not create untracked changes for files below this directory.

## Validation

- Confirm the ignore rule matches a representative agent EXE path.
- Confirm `git status --short` reports directory removals from the index and no local file has been deleted.
