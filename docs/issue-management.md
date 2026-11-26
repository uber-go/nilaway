# Issue management

Newly opened issues receive `needs triaging`, including blank and API-created reports.
Existing labels are preserved. Pull requests are excluded. The workflow does not classify
reports, assign people, or enforce report formatting.

Maintainers review the triage queue, choose labels, and request missing information.
This intake workflow does not send reminders, close issues, or change the existing backlog.

Before deployment, ensure GitHub Issues is enabled and the `needs triaging` label exists.
The workflow takes effect when it lands on the default branch. It does not check out repository
code or execute issue content.

Check workflow syntax before merging.

```sh
actionlint .github/workflows/issue-intake.yml
```
