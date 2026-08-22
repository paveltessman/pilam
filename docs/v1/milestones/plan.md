# Implementation Plan: milestones

This plan orders the work of `docs/v1/milestones/feature.md` into phases.

---

## Where the code goes

The feature follows the layers of `docs/v1/architecture.md`. It adds one domain package and touches four places that already exist.

| Place | Role |
| ----- | ---- |
| `internal/calendar` | The new domain package. Milestone types, templates, milestones, and the rules of §4 to §7. |
| `internal/postgres` | The queries behind the ports of `calendar`. |
| `db/migrations` | One migration for the four new tables. |
| `internal/http` | The handlers and the views of the five screens of §8. |
| `internal/seed` | The default template of §10, and the demo calendars. |

The `catalog` package keeps its own job. A model does not know about its milestones. The screens join the two packages at the handler.

---

## Phases

### M0 — The schema

One migration for the milestone type, the milestone template, the template item, and the milestone. The queries and the generated code that go with it.

The constraints carry the decisions of the feature. A model holds one milestone per type, an offset is zero or negative, and a fact date is not in the future.

**Done when:** the migration runs up and down.

---

### M1 — The pure kernel

The rules of the feature, inside `internal/calendar`, with no database and no clock. A function that needs the current day takes it as an argument.

This phase covers four things:

- The template applied to a target date, which gives the dates of §6.
- The derived state of one milestone and the roll-up of a model, a drop, and a season, from §5.
- The cascade of §7.1, which returns the list of milestones that move.
- The drop shift of §7.3, which moves the baseline and the plan together.

**Done when:** table-driven tests cover the four rules, including a milestone that holds a fact date, a model with no calendar, and a plan date that moves earlier.

---

### M2 — The service and the storage

The kernel gets a database and an audit trail. The service holds the writes a member and a root user make.

Every write lands in one transaction with its audit entries, the way `catalog` already does it. A drop shift writes one entry per milestone it moves, and the request id groups them.

**Done when:** the service tests prove the actions of §9, the audit trail holds the old value and the new value of every date move, and the baseline stays still on an ordinary edit.

---

### M3 — The seed

The default template of §10, the calendars of the seeded models, and the facts a real season would already hold.

**Done when:** a seeded database leaves 8 to 12 models late, spread unevenly across the drops, and a test asserts that number.

---

### M4 — The admin screens

The milestone types of §8.3 and the templates of §8.4. A root user reaches both.

The template editor carries the two columns of §3.2, the offset and the gap, and the date preview beside them.

**Done when:** a root user builds a template from nothing, and the preview shows the dates it produces from a target date.

---

### M5 — The model screen

The calendar section of §8.1, and the template field on model create from §8.5.

This phase brings the confirm dialog of §7.1 to the screen. The user sees the milestones that move before the app writes anything.

**Done when:** demo steps 2 and 3 of §13 run. A new model shows 15 dated milestones, and one plan date that moves 10 days later carries the later milestones with it.

---

### M6 — The milestone list

The season-wide screen of §8.2. The filters, the grouping, the selection, the two bulk actions of §7.2, and the CSV export.

**Done when:** demo steps 4 and 5 of §13 run. A model that passed its plan date yesterday shows as late this morning, and one action marks four color-models done.

---

### M7 — The roll-ups

The state line of §5 on the drop screen and the season screen, and the drop shift of §7.3 on the drop screen.

**Done when:** demo step 6 of §13 runs. A drop moves 14 days, the user confirms, and the assortment stops being late.

---

## What each phase must settle

The feature document leaves these open. The phase that meets one decides it and writes the reason down.

**M0.** Where the baseline rule lives. The database can hold it, or the service can. The choice decides how much of §4 a query can break.

**M1.** What the cascade returns. The confirm dialog of §7.1 needs the list before the write, so the same call serves the preview and the write.

**M2.** How the app applies a template to a model that already holds one. Section §6 says it adds only the missing types, and never touches a date.

**M5 and M6.** How much of the calendar section and the milestone list share a view. Both show a milestone row with the same columns and the same actions.
