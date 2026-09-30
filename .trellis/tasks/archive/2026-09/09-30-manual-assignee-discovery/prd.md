# Manual creation assignee discovery

## Request

Reuse the creation-assistant picker shown in the supplied reference for the
assignee popover in manual issue creation, on both Web and Desktop.

## Acceptance criteria

- Both creation modes use the same searchable actor directory, category tabs,
  responsibility previews, favorites, recent actors and full-directory access.
- Manual creation retains member selection, an always-reachable Unassigned
  option, the existing pill trigger and overflow-menu opening behavior.
- Archived actors stay hidden; inaccessible or runtime-unbound actors cannot
  be assigned. Private-agent indicators and disabled reasons remain available.
- Manual selection updates only the manual draft. Successful manual creation
  records recent actor usage without changing the default or last quick-create
  assistant. Failed creation and merely opening/selecting do not record usage.
- English and Simplified Chinese copy describes direct assignment accurately.
- Real-primitive tests cover search/selection and nested popup dismissal;
  targeted tests, typecheck, lint and visual inspection provide evidence.

## Boundaries

No API, dependencies, issue-detail/board picker, mobile or deployment changes.
User has authorized implementation and continued execution in this thread.
