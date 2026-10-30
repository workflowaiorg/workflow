---
name: forge-gitlab
description: "How each step of the workflow skill is done on GitLab with glab — Issues, merge requests, Closes #n, scoped labels and the one-call status swap GitLab Free needs, the closing-word trap, Draft merge requests, milestones and the board. Activate with the workflow skill whenever it sends you to the forge."
---

# Skill: The workflow on GitLab

The `workflow` skill says what happens and when; this one says how on GitLab. Use `glab` directly —
wf wraps none of it. Where `glab` has a flag for something, use it; `glab api` is for the few things
it does not cover, named below.

---

## Issues

- **Read one** before touching a file: `glab issue view <n> --comments`.
- **Search before creating one:** `glab issue list --search "<words>"`, and `--all` for closed ones.
- **Create one** with its labels and milestone from the start:
  `glab issue create --title "…" --description "…" --label "type::feature,area::<area>,status::todo" --milestone "<name>"`.
  A long description goes in a file first: `--description "$(cat issue.md)"`.
- **Comment:** `glab issue note <n> --message "…"`.
- **Link two Issues:** `--linked-issues <other>` when creating one; for two that exist,
  `glab api -X POST "projects/:id/issues/<n>/links" -f target_project_id=<project id> -f target_issue_iid=<other>`.
  **GitLab Free has no "blocks" link**, only "relates to", so the order of work is written in each
  Issue's description, never left to the link.

## Labels: scoped, and not exclusive on Free

Every label of the workflow skill is a scoped label here: `status::todo`, `status::in progress`,
`status::in review`, `status::blocked`; `type::feature`, `type::bug`, `type::chore`, `type::docs`,
`type::decision`; `area::<area>`.

**On GitLab Free, `::` labels are not mutually exclusive:** GitLab will not remove the old status
when a new one is added. So a status change is **one call** that removes the old label and adds the
new one together — never two, which would leave the Issue with both or neither for a moment, and
with both for good if the second call fails:

```sh
glab api -X PUT "projects/:id/issues/<n>" -f remove_labels="status::in progress" -f add_labels="status::in review"
```

Before setting a status, check that the Issue carries exactly one `status::` label
(`glab issue view <n>`), and repair it in the same call if a hand edit on the board left two.

The board has one column per `status::` label. Its columns show open Issues only: a closed Issue
moves to "Closed" on its own, keeping the status it had.

## Milestones

- **List the open ones** before assigning or creating: `glab api "projects/:id/milestones?state=active"`.
- **Create one** only by the workflow skill's rules:
  `glab api -X POST "projects/:id/milestones" -f title="<name>" -f description="…"`.
- **Assign it:** `glab issue update <n> --milestone "<name>"`.

## Merge requests

- **Open one as a Draft**, on the developer's go, after the first push:
  `glab mr create --draft --title "…" --description "…" --source-branch <branch> --target-branch main`.
  The description carries `Closes #n` from the start: GitLab acts on it at merge, Draft or not.
- **Take it out of Draft** when the Issue's work is done: `glab mr update <n> --ready`, together
  with the status swap above.
- **Read a review:** `glab mr view <n> --comments`; the diff with `glab mr diff <n>`.
- **Merge** only when asked: `glab mr merge <n> --squash` when the developer said squash, plain
  `glab mr merge <n>` otherwise. `glab mr merge` sets auto-merge by default, which waits for the
  pipeline — **but only when a pipeline exists.** With none running (CI minutes used up, say), it
  merges at once. So check the pipeline first — `glab mr view <n>` shows it — and say so to the
  developer when there is none.

## Closing

- `Closes #n` in a merge request description closes the Issue when the merge request merges, as
  long as the project has auto-close on (*Settings → Repository → Default branch*).
- **The closing-word trap.** GitLab reads a closing word next to an Issue reference as a command,
  whatever the words around it: "does not close #n" closes #n at merge. A merge request that
  must not close an Issue writes "#n stays open until …", and after the merge checks that the Issue
  is still open — `glab issue view <n>` — reopening it with `glab issue reopen <n>` if GitLab closed
  it anyway.

## The plan's own merge request

It carries `Part of #parent` and no closing word next to any Issue number — the parent is closed by
the last Issue of the plan, not by the plan.
