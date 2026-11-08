---
name: forge-github
description: "How each step of the workflow skill is done on GitHub with gh — Issues, pull requests, Closes #n, plain labels and keeping one status, the closing-word trap, draft pull requests, milestones, and merging only after the checks. Activate with the workflow skill whenever it sends you to the forge."
---

# Skill: The workflow on GitHub

The `workflow` skill says what happens and when; this one says how on GitHub. Its MR is a pull
request here. Use `gh` directly — wf wraps none of it. Where `gh` has a flag for something, use it;
`gh api` is for the few things it does not cover, named below. In `gh api` paths, `{owner}/{repo}`
is filled in from the current repository.

---

## Issues

- **Read one** before touching a file: `gh issue view <n> --comments`.
- **Search before creating one:** `gh issue list --search "<words>" --state all`.
- **Create one** with its labels and milestone from the start:
  `gh issue create --title "…" --body-file issue.md --label "type: feature" --label "area: <area>" --label "status: todo" --milestone "<name>"`.
- **Comment:** `gh issue comment <n> --body "…"`.
- **Link two Issues** by naming `#<other>` in the description or a comment: GitHub shows the
  reference on both. The order of work is written in each Issue's description, never left to the
  link: the description is what every reader and every forge has.

## Labels: plain, so one status is kept by hand

Every label of the workflow skill is a plain label here, its group and value joined by a colon and
a space: `status: todo`, `status: in progress`, `status: in review`, `status: blocked`;
`type: feature`, `type: bug`, `type: chore`, `type: docs`, `type: decision`; `area: <area>`.

**GitHub has no exclusive labels:** adding a new status never removes the old one. So a status
change removes the old label and adds the new one in the same command:

```sh
gh issue edit <n> --remove-label "status: in progress" --add-label "status: in review"
```

Before setting a status, check that the Issue carries exactly one `status:` label
(`gh issue view <n>`), and remove any other in the same command if a hand edit left two.

The board is the Issue list by status: `gh issue list --label "status: in review"`. It shows open
Issues only: a closed Issue keeps the status it had.

**A label a CI job reads**, such as `api-passthrough`, which lets the backend's breaking-change job
accept a break, is a plain one on the pull request: `gh pr edit <n> --add-label api-passthrough`.
Adding it starts a new run, as the calling workflow listens to `labeled`; re-running an old run
does not see it, as a run reads the labels its event carried.

## Milestones

- **List the open ones** before assigning or creating: `gh api "repos/{owner}/{repo}/milestones?state=open"`.
- **Create one** only by the workflow skill's rules:
  `gh api -X POST "repos/{owner}/{repo}/milestones" -f title="<name>" -f description="…"`.
- **Assign it:** `gh issue edit <n> --milestone "<name>"`.

## Pull requests

- **Open one as a draft**, on the developer's go, after the first push:
  `gh pr create --draft --title "…" --body-file pr.md --base main`.
  The description carries `Closes #n` from the start: GitHub acts on it at merge, draft or not.
- **Take it out of draft** when the Issue's work is done: `gh pr ready <n>`, together with the
  status change above.
- **Where GitHub refuses a draft** — a private repository on GitHub Free or Pro — open it without
  `--draft` and with `[WIP] ` in front of its title, and drop that prefix
  (`gh pr edit <n> --title "…"`) where `gh pr ready` would run.
- **Read a review:** `gh pr view <n> --comments` shows the conversation; the comments on lines of
  the diff are `gh api "repos/{owner}/{repo}/pulls/<n>/comments"`; the diff is `gh pr diff <n>`.
- **Merge** only when asked. Squashed, with the message the workflow skill shapes:
  `gh pr merge <n> --squash --subject "<summary line> (#<n>)" --body-file message.md`; otherwise
  `gh pr merge <n> --merge`. Never `--delete-branch`: it switches the checkout to `main`, which the
  root checkout already holds, so it fails in a worktree after the merge went through. Delete the
  remote branch after the merge with `git push origin --delete <branch>`, unless the repository
  deletes merged branches itself, and the local one as the workflow skill says.
- **`gh pr merge` never waits for the checks:** with none required it merges at once, and with
  required ones still running it refuses. So wait for them first — `gh pr checks <n> --watch` — and
  say so to the developer when there are none.

## Closing

- `Closes #n` in a pull request description closes the Issue when the pull request merges **into
  the default branch**. One merged into another branch closes nothing.
- **The closing-word trap.** GitHub reads "close", "fix" or "resolve" in any form, followed by an
  Issue reference, as a command, whatever the words around it: "does not close #n" closes #n at
  merge. A pull request that must not close an Issue writes "#n stays open until …", and after the
  merge checks that the Issue is still open — `gh issue view <n>` — reopening it with
  `gh issue reopen <n>` if GitHub closed it anyway.

## The plan's own pull request

It carries `Part of #parent` and no closing word next to any Issue number — the parent is closed by
the last Issue of the plan, not by the plan.
