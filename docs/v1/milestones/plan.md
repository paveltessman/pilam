# Implementation Plan: milestones

---

## PRs

### PR1 — Milestone types

A root user creates a milestone type, renames it, and retires it.

One migration adds the milestone type table with the two unique indexes on the short name and the description. The service holds the writes, and every write lands in the audit trail.

**Done when:** A root user creates a type, a member cannot reach the screen, and a retired type keeps its row.

---

### PR2 — Milestone templates

A root user builds an ordered list of types with their offsets.

One migration adds the template and the template item. The constraints carry the decisions of the feature: one type appears once per template, an offset is zero or negative, and at most one template is the default. The order index is deferrable, like `model_photo`, because a reorder swaps two rows.

The template item has no screen of its own, so the trail records its changes under the template.

The template editor of carries the two columns, the offset and the gap, and the date preview beside them. The gap is the days to the item above, the offset is the days from the target date of the drop, and the app stores the offset.

The preview needs the first pure rule of the package. A template applied to a target date gives one milestone per template item, with the baseline date and the plan date both equal to the target date plus the offset.

**Done when:** A root user builds a template from nothing, edits either the offset or the gap, reorders the rows, and the preview shows the dates the template produces from a target date.

---

### PR3 — Milestone section on the model screen

The milestone section appears on the model screen. That section holds one row per milestone in plan date order, with the columns type, baseline, plan, fact, state, and slip in days. A late row carries a marker.

One migration adds the milestone table. A model holds one milestone per type, a note is 500 characters at most.

The service holds four writes:

- Apply a template to a model. The dates come from the target date of the drop and the offsets, and the baseline date and the plan date start equal.
- Apply a template to a model that already holds a calendar. It adds only the missing types and never touches a date.
- Add one milestone of a type the model lacks.
- Edit one row: the plan date, the fact date, the note, and the active flag.

An edit of a plan date here moves that one milestone. The cascade of §7.1, where a plan date that moves later pushes the later milestones of the same model, arrives in later  PRs.

---

### PR4 — The seed

The demo data, with its 15 items and their offsets from -270 to -7 days, the calendars of the seeded models, and the facts a real season would already hold.

---

The next PRs will be planned later.
