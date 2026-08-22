# Design: milestones

This document designs feature 2 of `docs/v1/prd.md`: the time and action calendar of a model.

It decides what the feature does and what the user sees. It does not name tables, packages, or routes. The implementation plan comes after this document.

---

## 1. What the calendar answers

The brand works backwards from the day a drop goes on sale. Every model runs through the same chain of steps: tech packs, samples, a fit approval, bulk production, an inspection, a shipment, a warehouse arrival, and the day the goods are ready to sell.

The workbook holds that chain in about 30 date columns. `docs/v1/table_desc.md` §8 lists them. Three failures of the workbook belong to this feature:

- A plan date and a fact date share one cell. The slip disappears the moment somebody types over the plan.
- The lead times live inside cell formulas. Only the author of the workbook can change them.
- OTIF stays blank until the goods land. It reports the past and warns nobody.

The calendar answers the question: what runs late today, and what runs late next week.

---

## 2. Scope

We build:

- **Milestone types** and **milestone templates**, as configuration a root user edits.
- **One calendar per model**, built from a template and dated from the drop.
- Three dates on every milestone: a baseline, a plan, and a fact.
- A **derived state** per milestone, and a roll-up state per model.
- The **calendar section** of the model screen.
- The **milestone list**, which covers a whole season in one screen.

---

## 3. The objects

### 3.1 Milestone type

One step of the critical path, named once and used by every template and every model.

| Field       | Meaning                                                     |
| ----------- | ----------------------------------------------------------- |
| Short name  | The column label on a dense list.                           |
| Description | What the step is, in the words of the team. Unique.         |
| Active      | A retired type keeps its milestones and leaves the pickers. |

### 3.2 Milestone template

An ordered list of types with their offsets. The brand keeps a few templates, such as one per sourcing type.

| Field   | Meaning                                                                 |
| ------- | ----------------------------------------------------------------------- |
| Name    | `Import, rail` or `Domestic`. Unique.                                   |
| Default | One template is the default for a new model.                            |
| Active  | A retired template leaves the pickers and keeps the calendars it built. |

One template item:

| Field          | Meaning                                                        |
| -------------- | -------------------------------------------------------------- |
| Milestone type | The step. One type appears once per template.                  |
| Offset         | Days from the target date of the drop. Zero or negative.       |
| Position       | The order on the screen. Ties on the offset break by position. |

The editor shows a second, computed column beside the offset: the gap in days to the item above it. A user edits either column. The gap is how a garment technologist thinks, and the offset is what the calendar needs, so we show both and store the offset.

### 3.3 Milestone

One step of one model. A milestone belongs to a model and never to a drop or a season.

| Field          | Meaning                                                                           |
| -------------- | --------------------------------------------------------------------------------- |
| Milestone type | The step.                                                                         |
| Baseline date  | The date the calendar first promised. See §4.                                     |
| Plan date      | The date the team expects now.                                                    |
| Fact date      | The day the step was done. Empty until then. Set by clicking a "complete" button. |
| Note           | Free text, up to 500 characters. Why the date moved.                              |
| Active         | An inactive milestone leaves the calendar and stays in the history.               |

---

## 4. The three dates

The workbook keeps one cell per date and loses the history. We keep three dates, and each one answers a different question.

**Baseline.** What did we promise. The calendar sets it when it applies the template, and an ordinary edit never moves it. It moves in one case only: the drop moves its target date and the user confirms the shift. See §7.3.

**Plan.** What do we expect now. It starts equal to the baseline. A member edits it, and every edit lands in the audit trail.

**Fact.** What happened. A member stamps it, and the app offers today as the value. A fact date must not be later than today. A member can clear a fact that was stamped by mistake.

Slip in days is the plan date minus the baseline date. Positive means the step moved later than the promise. The same subtraction on the fact date gives the slip that really happened.

---

## 5. The derived state

The app computes the state on every read and stores nothing. Today comes from the business timezone.

| State   | Rule                                                          |
| ------- | ------------------------------------------------------------- |
| Done    | The fact date is set.                                         |
| Late    | No fact date, and the plan date is before today.              |
| Due     | No fact date, and the plan date falls inside the next 7 days. |
| Planned | Anything else.                                                |

A done milestone also reports whether it landed on time. On time means the fact date is not later than the baseline date.

The window of 7 days is a constant of v1. It becomes configuration when somebody asks for a second number.

**The state of a model.** Late when one milestone is late. Due when none is late and one is due. Done when every milestone holds a fact date. Planned in every other case. The model list of feature 4 shows this state and the next milestone that comes due.

**The state of a drop and a season** is a count of the models in each state. It appears on the drop screen and the season screen as a single line. A product manager finds the trouble there without opening the list.

---

## 6. How a calendar starts

A user picks a template on the model create screen. The default template is preselected. The user can pick `no calendar` instead.

The app reads the target date of the drop and writes one milestone per template item. The baseline date and the plan date both become the target date plus the offset. The fact date stays empty.

A model that already holds a calendar can take a template again. The app then adds only the milestone types the model lacks. It never touches a date that exists. This is how a new step reaches the 850 models of a season after the season started.

A model with no calendar keeps no milestones and never appears as late. The milestone list counts these models per drop, so nobody loses a model by forgetting the template.

The offsets of the template are calendar days. The app does not know weekends, and it does not know holidays. The lead times of the trade already include them.

---

## 7. How a date moves

### 7.1 One plan date

A user opens the calendar section or the milestone list and types a new plan date.

The app shows what follows before it writes. Moving a plan date later pushes every later milestone of the same model by the same number of days. The list of the milestones that move appears in the dialog, and the user can turn the shift off.

The shift skips a milestone that holds a fact date. Work that is done does not move.

Moving a plan date earlier moves that milestone alone. Pulling a whole chain forward is a plan, not a side effect.

### 7.2 Many dates at once

The milestone list carries a selection. A user selects rows and applies one action to all of them:

- Stamp a fact date on the selection.
- Shift the plan dates of the selection by a number of days.

One design becomes four color-models, and four color-models receive the same sample on the same day. Without this action the app is slower than the workbook at the most common job of the week.

### 7.3 The drop moves

A drop that moves its target date carries its whole assortment. The drop screen asks one question: shift the calendars of the models of this drop by the same number of days.

The shift moves the baseline date and the plan date together, and it skips a milestone that holds a fact date. Moving the baseline is what makes this different from an ordinary edit: the wave really did move, and the models are not late any more.

The user can decline. The calendars then stay where they are, and the models start to run late against a drop that already moved.

---

## 8. Screens

### 8.1 The calendar section of the model screen

The main working surface. It sits under the attributes and above the history.

One row per milestone, in plan date order. The columns are the type, the baseline, the plan, the fact, the state, and the slip in days. A row in the late state carries a marker. The baseline stays quiet until the plan moves away from it.

Actions on a row: edit the plan date, stamp the fact as today, pick another fact date, clear the fact, write a note, deactivate the milestone.

Actions on the section: apply a template, add one milestone of a type the model lacks.

### 8.2 The milestone list

The screen goal G2 of the PRD asks for. It covers the whole season, and it is the answer to `OTIF reports the past`.

One row per milestone, across every model of the season. The columns are the model article, the drop, the type, the plan, the fact, the state, and the slip.

The screen opens on the late and due rows of the current season, worst slip first. The filters are the drop, the milestone type, the state, and the free-text search on the article. The user can group the rows by drop or by type.

The list holds a selection and the bulk actions of §7.2. It exports the current view as CSV, the same way the model list of feature 4 does.

### 8.3 Admin: milestone types

A list of types with their state, plus create, rename, and deactivate. A root user reaches it. Every write lands in the audit trail.

### 8.4 Admin: milestone templates

A list of templates, plus the editor of one template.

The editor holds the ordered items. A user adds a type, sets the offset or the gap, reorders the rows, and removes a row. Beside the table the editor shows a preview: a date picker holding a target date, and the dates the template produces from it. A lead time of 270 days is hard to read as a number and easy to read as a date.

### 8.5 Model create

One more field: the template. The default template is preselected, and `no calendar` is available.

---

## 9. Roles and the audit trail

A member reads and edits milestones: the plan date, the fact date, the note, and the active flag. A member applies a template to a model.

A root user also edits the types and the templates.

Every write lands in the trail, as PRD §4 requires. A milestone is its own entity in the trail, and the model screen shows the entries of the model and of its milestones together. A template item has no screen of its own. The trail records its changes under the template, the way it records a photo change under the model.

The trail records these actions:

- the calendar applied
- the plan date moved
- the fact date stamped, and the fact date cleared
- the note changed
- the milestone deactivated, and the milestone reactivated
- the drop shift, as one entry per milestone it moved

---

## 10. The default configuration

The seed loads one template, `Import, rail`, with 15 items. The offsets come from `table_desc.md` §7 and §8, where the workbook holds them inside formulas.

| #   | Milestone type               | Offset | Gap | Date, for a drop on 16.07.2026 |
| --- | ---------------------------- | ------ | --- | ------------------------------ |
| 1   | Tech pack for the 1st sample | -270   |     | 19.10.2025                     |
| 2   | 1st sample received          | -240   | 30  | 18.11.2025                     |
| 3   | Tech pack for the 2nd sample | -230   | 10  | 28.11.2025                     |
| 4   | 2nd sample received          | -200   | 30  | 28.12.2025                     |
| 5   | Tech pack for the PPS        | -190   | 10  | 07.01.2026                     |
| 6   | PPS received                 | -160   | 30  | 06.02.2026                     |
| 7   | Fit approved                 | -155   | 5   | 11.02.2026                     |
| 8   | Tech pack for bulk           | -150   | 5   | 16.02.2026                     |
| 9   | Bulk production started      | -140   | 10  | 26.02.2026                     |
| 10  | Ready for inspection         | -95    | 45  | 12.04.2026                     |
| 11  | Inspection passed            | -88    | 7   | 19.04.2026                     |
| 12  | Photo sample handed over     | -88    | 0   | 19.04.2026                     |
| 13  | Shipped                      | -74    | 14  | 03.05.2026                     |
| 14  | Arrived at the warehouse     | -14    | 60  | 02.07.2026                     |
| 15  | Ready for sale               | -7     | 7   | 09.07.2026                     |

The 60 days between the shipment and the warehouse are the rail transit time of the workbook. A second template, `Import, air`, changes that one number to 14 and moves the whole chain 46 days later. This is the reason the lead times are data.

The seed applies the template to every seeded model and stamps the facts that a real season would already hold. It leaves 8 to 12 models genuinely late, spread unevenly across the drops, so the milestone list has something to show on the first run.

---

## 11. Not in this feature

- An owner per milestone. The people columns of the workbook arrive with feature 3.
- A result per milestone, such as `passed` or `failed` on an inspection or a fabric test.
- Notifications, email, and comments. The note field is the only place a reason lives.
- A dependency graph. See §12.
- Working days and holidays.
- An off-calendar flag on a model. A model with no calendar covers the case.
- Reports, dashboards, and the OTIF number. The data this feature writes is what a later report reads.
- A calendar on a drop or a season. Only a model holds milestones.

---

## 12. Two decisions that change the PRD

**A dependency graph becomes one offset per item.** PRD §5 says `templates with lead times and dependencies`. This design gives every template item one offset from the target date of the drop, and one shift rule: a plan date that moves later pushes the later milestones of the model. The order of the list is the dependency. The result on the screen is the same, and the feature carries no scheduling engine. A real graph, where two chains join at the bulk order, is an addition later and not a redesign.

**A third date joins the plan and the fact.** PRD §5 says `a plan against a fact`. This design adds the baseline of §4. Two dates cannot measure the slip, because the plan itself moves. The baseline costs one quiet column on the screen and answers the failure that PRD §1 names.

---

## 13. Done when

1. A root user creates a milestone type and a template, and no release ships.
2. A user creates a model with the default template and sees 15 dated milestones, computed from the target date of the drop.
3. A user moves one plan date 10 days later, confirms the shift, and the later milestones of the model move 10 days. The milestones that hold a fact date stay.
4. A model whose plan date passed yesterday, with no fact date, appears in the milestone list as late this morning.
5. A merchandiser marks the same milestone done on four color-models in one action.
6. A drop moves 14 days, the user confirms, and the assortment stops being late.
7. The audit trail holds every date move, with the old value and the new value.
