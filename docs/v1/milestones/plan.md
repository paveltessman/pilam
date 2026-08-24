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

### PR5 — A calendar from the model create screen

A user creates a model, picks a template, and lands on a card that already holds 15 dated milestones.

No migration. The create form takes one more control, the template. The default template is preselected, and `no calendar` is a choice of the same control. The list of templates comes from the same reader the calendar section uses, so a retired template is not offered.

**Done when:** A user creates a model with the default template and sees the 15 milestones, dated from the target date of the drop. A user who picks `no calendar` gets a card with an empty calendar section.

---

### PR6 — The cascade of one plan date

A user moves a plan date 10 days later, reads what follows, and confirms.

No migration. The feature is one pure rule and one dialog.

The rule of §7.1: a plan date that moves later pushes every later milestone of the same model by the same number of days. Later means the plan date is after the old plan date of the row the user moved. A row that holds a fact date does not move, and neither does an inactive row. A plan date that moves earlier moves its own row alone.

The service gets one write, the plan date move. It takes the milestone, the new plan date, and the flag that turns the shift off. The write runs in one transaction, and it lands in the trail as one entry per milestone it moves.

The screen shows the rule before it writes. The date box asks the server for the preview, and the dialog lists the rows that move with the date each of them leaves and the date it takes. A switch turns the shift off, and the dialog then writes the one row.

**Done when:** A user moves one plan date 10 days later, confirms the shift, and the later milestones of the model move 10 days. The milestones that hold a fact date stay. The trail holds one entry per moved row, with the old value and the new value.

---

### PR7 — The milestone list

The screen goal G2 of the PRD. One screen holds the late work of a whole season.

The list opens on the late and due rows of the current season, worst slip first. The columns are the model article, the drop, the type, the plan, the fact, the state, and the slip. The row links to the card of its model. The filters are the drop, the milestone type, the state, and the free-text search on the article. The user groups the rows by drop or by type. The screen exports the current view as CSV.

The list also counts the models of the season that hold no calendar, per drop. A model with no calendar never reads as late, so the count is the only place it appears.

The screen is the one milestone screen a member reaches.

**Done when:** A model whose plan date passed yesterday, with no fact date, appears in the list as late this morning. The filters narrow the list, the grouping holds, and the export writes the rows the screen shows.

---

### PR8 — The calendar state on the model list

A user opens the model list and reads which models to worry about.

No migration. The roll-up rule of §5, plus one column on the list.

The rule takes the summary of one calendar and answers the state of the model. Late when one milestone is late. Due when none is late and one is due. Done when every milestone holds a fact date. Planned in every other case. A model with no calendar holds a fifth answer, no calendar, and it never reads as late.

**Done when:** The model list shows a late model as late, with the step and the days, on the morning after its plan date passed. A model with no calendar reads as such and not as late.

---

The next PRs will be planned later.
