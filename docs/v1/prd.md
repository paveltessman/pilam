# PRD: v1

This document is the specification for v1.

---

## 1. What this product is

pilam is a product lifecycle management system (PLM) for an apparel brand. It replaces a seasonal Google Sheet.

The sheet carries five systems flattened into one grid: a product catalog, a buy plan, a critical-path date tracker, a sourcing and compliance record, and a logistics tracker.

---

## 2. The system we replace

Everything in this section comes from the live workbook. The decisions later in the document lean on these facts.

### 2.1 Shape

One workbook per season, two tabs, 120 columns each.

| Tab | Grain | Size | Purpose |
|---|---|---|---|
| `Base` | One color-model | Header plus about 850 rows | The master record. All 120 attributes. |
| `Order by Size` | One color-model | Pre-formatted to about 855 rows | The quantity buy, per size and per channel. |

The second tab repeats the first 13 identity columns of `Base` and then replaces everything else with a size-level quantity matrix. The two tabs join by repeated identity columns. There is no key column.

### 2.2 The row

A row is a color-model: one article number in one color (the color encoded as a code plus a Russian name plus an English name). A cell note says that after the design review a sketch is broken down into color-models, so one design becomes several rows.

A row is never deleted. A cell note on the `active` column says to untick it when a model is cancelled, and not to delete the row.

Each row carries three names: a working name, a trade name, and a legal name for labelling and certification.

### 2.3 The controlled lists

- status: 15 states, from trims and fabric through samples, bulk production, quality control, and logistics, to on sale. The last two states are exits, not steps: postponed to the next season, and cancelled.
- `Newness`, three values: new, modified carry-over, straight carry-over.
- Five classification lists: line (4 values), direction (9), class (6), category (14), and type (37). They are nominally a hierarchy and they are stored as five flat dropdowns. Nothing enforces agreement between them, and direction and class overlap on three values.
- Sourcing type, two values: import and domestic.
- Shipment mode, four values, each label carrying its own transit time in days.
- Inspection result, certification status, and four fabric test results, each a short list.

### 2.4 The dimensions the sheet cannot express

- **Channel.** Two channels, the brand's own and a marketplace. Channel is not a column. It multiplies other columns, so every quantity exists three times over: a total, an own-channel figure, and a marketplace figure.
- **Size.** Three size families: letter, letter plus body height, and numeric range. On `Base` the size run is one joined string. On the other tab it becomes a 3 by 3 block of columns, three channel scopes by three families. Only the block that matches the model gets filled.
- **Planning stage.** The same commercial figures appear three times, at design review, at line review, and as actuals. The actuals block is seeded by formula from the line review block, so "not yet actual" and "came in exactly on plan" hold the same value.
- **People.** Four role columns per row: product manager, designer, pattern maker, and garment technologist.
- **Shared records.** Fabric, factory, supplier, and certificate repeat inline on every row that uses them. A certificate is valid for about five years, so it outlives several seasons and is retyped in each.

### 2.5 The dates

About 30 date columns, almost all a plan and fact pair or triple.

Four sample rounds, each with a tech-pack priority date, a tech-pack actual date, and a sample received date. Then a production and delivery chain: ready for inspection, inspection result, ready to ship, ship date, warehouse arrival, ready for sale, and the actual release wave.

Several of those columns are named "forecast / fact" and hold both in one cell. The intermediate values get overwritten in place, so the history is lost.

Five derived dates and one KPI live as cell formulas:

| Derived value | Rule |
|---|---|
| Warehouse arrival, forecast | Ship date plus the transit days of the shipment mode |
| Ready for sale, forecast | Warehouse arrival plus 7 days |
| Photo sample handover | Ship date minus 14 days |
| Actuals block | Copied from the line review block |
| OTIF | Market intro date minus actual warehouse arrival. Positive means early. |

The transit constants and the two offsets are hardcoded inside formulas, and the transit numbers are duplicated in the dropdown labels.

The OTIF column stays blank until the goods physically land, which makes the brand's headline delivery KPI a post-mortem.

### 2.6 What is already broken in it

- No stable primary key. The article number is the de-facto key and the second tab joins by repeating identity columns.
- Plan and fact in one cell destroys history.
- Actuals default to plan by formula, so variance is unmeasurable.
- Design-level facts repeat across every color of one design, and the copies drift apart.
- Even the single mock row does not balance: the channel quantities do not sum to the stated total, and one of them is stored as text.

---

## 3. The generic stance

The problem with a PLM built for one brand is that the brand changes its process, and every change becomes a developer ticket. So the product draws a line: a small spine lives in the schema, and everything above the spine lives as data.

After the deployment, an administrator must be able to change the following without a developer and without a release:

- add a field to a model, rename it, reorder it, retire it
- change any list of allowed values, and add a new value
- change the milestone types, their order, and their lead times
- create a milestone template and apply it to a model
- create seasons, drops, and users

The line has two sides, and both sides are explicit.

| In the schema (the spine) | Data, editable from the UI |
|---|---|
| season, drop, model, model photo | every field on a model, and its label, help text, order, and group |
| milestone, milestone type, milestone template, template item | every list of allowed values, including status and the five classification lists |
| field, field group, field option, field value | milestone types, templates, lead-time offsets, default owners |
| user, session epoch, audit entry | seasons, drops, users |

---

## 4. Naming

The entity that carries an article number is a **model**. This document uses "schema" for database structure, so "model" never means two things.

One model is one color-model: one article in one color. See §6 for details.

---

## 5. Goals and non-goals

G1. The model list replaces the `Base` tab for identity and dates. A user opens one screen, sees the season's assortment, filters it, sorts it, edits a cell, and does not open the workbook to finish the job.

G2. Milestones are records, and slip is visible before the goods ship. Three dates per milestone, a frozen baseline, and one list of everything that runs late across the season.

G3. Administrator can manage model fields.

G4. Nothing in v1 needs a rewrite to carry v2. Costing, quantities, and reference entities are absent from v1. They must arrive as additions, not as a redesign.

### Non-goals for v1

- No costing and no planning stages.
- No quantities. No channel split, no size grid, no order value. The `Order by Size` tab has no equivalent.
- No reference entities. Fabric, partner, and certificate stay as values in a list, not as records with their own screens.
- No season transitions. No copy-forward, no carry-over, no postponement action.
- No baseline replan action. A baseline is written once, when a template is applied.
- No formulas, no notifications, no email, no comments.
- No mobile layout.
- No self-registration and no password reset by email. An administrator creates a user and hands over the first password.

---

## 6. The grain decision

One model is one article in one color, exactly as the workbook works today.

The alternative is a split: a Style is the design, a Colorway is that design in one color, and the design-level facts live once. That split is the correct end state, but v1 does not build it.

---

## 7. Fields: the configuration mechanism

One mechanism carries both the workbook's own columns and anything an administrator adds later.

### 7.1 What a field is

- `entity` - `model` in v1, held by a check constraint. The column exists so a second entity is a migration, not a re-keying.
- `key` - Stable machine key, lower snake case. Generated from the first label, then immutable. Unique per entity. Exports, imports, and saved views refer to the key.
- `label` - The display name. Editable at any time, with no effect on stored values.
- `kind` - One of the eight kinds in §7.2. Immutable after creation.
- `help_text` - Optional.
- `group` - The panel the field appears in on the model page. A field group has a name and a position.
- `position` - Order inside the group.
- `required` - Blocks a save that leaves the field empty. See details in §7.5.
- `searchable` - Text kinds only. The list search reads every searchable text field.
- `number_scale` - Number kind only. Decimal places, 0 to 4.
- `active` - A retired field disappears from every form and view. Its values stay in the database.

### 7.2 Field kinds

- `text` - Single line, 500 characters.
- `long_text` - Multi line, 8000 characters. Never a list column.
- `number` - `numeric`, scale from `number_scale`.
- `date` - A calendar date.
- `checkbox` - Absent reads as false.
- `select` - One option from the field's own list.
- `multi_select` - Several options, one row per option.
- `user` - A reference to an active user.

---

## 8. Milestones

This is the standard time and action calendar of apparel PLM.

### 8.1 The three dates

| Date | Written | Mutable |
|---|---|---|
| `baseline_date` | Once, when a template is applied | No in v1 |
| `forecast_date` | Whenever reality changes | Yes |
| `actual_date` | Once, when the thing happens | Cleared only by an explicit undo |

A blank actual means not done. Nothing pre-fills an actual.

### 8.2 Milestone type

A milestone type is data: a key, a label, a position, and an `active` flag.

### 8.3 Templates and offsets

A template has a name and a list of items. An item carries:

| Attribute | Meaning |
|---|---|
| `type_id` | Which milestone type this item creates |
| `depends_on` | Another item in the same template, or empty |
| `offset_days` | Days from the anchor. Negative means before the anchor. |
| `default_owner` | An optional user, copied onto the milestone |
| `position` | Display order on the model page |

The anchor is the drop's target date. An item with no `depends_on` counts from the anchor. An item with a `depends_on` counts from that item's date.

Operations:

1. **Apply.** Resolve every item to a date from the anchor, and write that date as both the baseline and the first forecast.
2. **Propagate.** When a forecast moves, shift every dependent milestone by the same arithmetic. A milestone that already has an actual date never moves, and neither does anything it anchors.

Both operations are pure functions over a graph and a date. The clock reaches them as a `today` parameter. This is the highest-risk logic in the product, and it gets table-driven tests that include cycles, gaps, missing anchors, and stamped actuals.

A cycle is rejected when the template is saved.

### 8.4 The milestone record

A milestone belongs to one model. It carries the three dates, an owner, a note, a position, and the template item it came from.

A user adds a milestone to one model by hand, and removes it while it has no actual date. So a model that deviates from its template does not need a new template.

Changing a template never rewrites the milestones that were already created from it. An apply is an event, not a live link.

### 8.5 State is derived, never stored

| State | Rule |
|---|---|
| `done` | The actual date is set |
| `late` | No actual date, and the baseline is before today |
| `at_risk` | No actual date, and the forecast is after the baseline |
| `planned` | Everything else |

A model is at risk when any of its milestones is at risk or late.

---

## 9. Users, roles, and sessions

### 9.1 Users

A user has an email, a name, a password hash, a role, an `active` flag, and a session epoch.

An administrator creates the user and sets a first password.

### 9.2 Roles

Two roles, and the role sits on the user row.

| Role | Can |
|---|---|
| `member` | Read and edit every model, milestone, photo, season, and drop |
| `root` | Everything a member can, plus the field screens, the milestone type and template screens, and the user screens |

### 9.3 Sessions and revocation

The signed cookie already in the code carries the user id and the session epoch. The user row carries the current epoch. A mismatch rejects the request.

A password change bumps the epoch. Deactivating a user bumps the epoch. Both take effect on the next request, on every device, with no session table to sweep.

### 9.4 Login hardening

- The login form throttles per account after five failures, with a growing delay.
- A wrong password and an unknown login return the same message and take a similar time.
- The password is at least 12 characters and at most 128. No composition rules.
- The cookie is `HttpOnly`, `Secure`, and `SameSite=Lax`. CSRF protection covers every write.

---

## 10. Screens

### 10.1 S1 Login

Email and password, inline field errors, and the forced password change on the first login. One destination after a success: S2.

### 10.2 S2 Model list (the main screen)

- Columns are chosen by the user. A column picker over every active field, plus the schema columns and the photo thumbnail. The choice saves per user, as one default view.
- Filters: season, drop, active, milestone state, and any `select`, `multi_select`, or `checkbox` field.
- Search: the article, the color, and every field marked `searchable`.
- Sort: any column except `long_text` and `multi_select`.
- Group by drop, optional, with a count per group.
- Inline edit for `text`, `number`, `date`, `checkbox`, `select`, and `user`. One cell, one request, one audit row. Errors render in the cell.
- Export the current view to CSV, with the visible columns and the current filters.
- Row click opens S3.

### 10.3 S3 Model page

- Header: the article, the color, the photo, the season, the drop, the `active` flag, and the at-risk state.
- One panel per field group, with the fields in their configured order, and the help text reachable per field.
- Photos: multi upload through the existing media store, the first as the thumbnail, drag to reorder.
- Milestones: every milestone in template order, with the three dates, the owner, the derived state, and the slip in days.
- A milestone row opens an editor: the forecast is the one editable date, "mark done" stamps today, and the baseline is greyed with a note that explains why.
- Moving a forecast shifts the dependent milestones and updates the at-risk state in the same response.
- History: the audit trail for this model, newest first, with who, when, and old to new.

### 10.4 S4 Milestone list across models

One list for the whole season, sorted by slip in days.

Filters: drop, milestone type, owner, and state. A row opens the milestone editor for that model.

### 10.5 S5 Admin

One section per configurable thing, and each one obeys §7.5.

- Fields and field groups, with their options.
- Milestone types.
- Milestone templates, with the dependency graph and the offsets.
- Users.
- Seasons and drops.

Every write here goes to the audit trail with the same shape as a data write.

### 10.6 S6 Model create

Season, drop, article, color, and a template to apply. Every other field is optional at create time, unless it is required.

---

## 11. Audit

Every change records who, what, when, and old to new.

One table: time, actor, entity, entity id, action, field key, old value, new value, the request id.

Three rules:

1. The service that performs the write records the entry, inside the same transaction.
2. A configuration change is audited like a data change.
3. The request id ties the entry to the log lines of the same request.
