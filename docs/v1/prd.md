# PRD: v1

pilam is a product lifecycle management system (PLM) for one apparel brand.

This document declares the shape of the system and the features we expect. It does not design a feature. Each feature gets its own design document when we implement it.

Tech stack: `docs/v1/stack.md`. Code structure: `docs/v1/architecture.md`.

---

## 1. The system we replace

The brand runs one Google Sheet workbook per season. The workbook holds two tabs of 120 columns. One row is one color-model, and one season holds about 850 rows.

The workbook acts as five systems in one grid: a product catalog, a buy plan, a critical-path date tracker, a sourcing and compliance record, and a logistics tracker.

`docs/v1/table_desc.md` holds the column-level analysis. What fails in the workbook today:

- No stable key. The two tabs join by repeated identity columns.
- A plan date and a fact date share one cell, so the history is lost.
- Actual figures come from plan figures by formula, so variance is unmeasurable.
- Design-level facts repeat on every color of one design, and the copies drift apart.
- Channel, size, and planning stage are not columns. They multiply the other columns instead.
- Lead times and date offsets sit inside cell formulas.
- OTIF, the headline delivery KPI, stays blank until the goods land. It reports the past and warns nobody.

## 2. What we build

A web application for the staff of one brand. The users are product managers, designers, pattern makers, garment technologists, and merchandisers. The system holds the data of one company.

## 3. Goals for v1

- **G1.** The model list replaces the `Base` tab for identity and dates. A user opens one screen, filters the season assortment, edits a value, and does not open the workbook to finish the job.
- **G2.** Slip is visible before the goods ship. Every model carries a dated calendar, and one screen lists everything that runs late across the season.
- **G3.** The features that v1 leaves out arrive later as additions, not as a redesign.

## 4. Decisions

**Custom fields are one narrow feature.** An administrator adds a custom field to a model without a release. A custom field holds a value and a label, and nothing else depends on it.

**One grain: the model.** One model is one article in one color, as the workbook works today. The split into a style and a colorway is the correct end state, and it's a subject for future versions.

**Nothing is deleted.** A cancelled model keeps its row and loses its active flag. The same rule covers seasons, drops, and users.

**Configuration is data.** An administrator changes the following without a developer and without a release:

- seasons, drops, and users
- the values of the controlled lists
- the milestone types
- the milestone templates, with their lead times

**Two roles.** A member reads and edits the catalog and the calendar. A root user also reaches the configuration screens and the user screens.

**Every write is audited.** A configuration change is audited like a data change.

## 5. Features in v1

We build them in this order.

1. **Catalog.** Seasons, drops, models, and model photos.
2. **Milestones.** A time and action calendar per model. Milestone types, templates with lead times and dependencies, a plan against a fact, and a derived late state.
3. **Model attributes.** The workbook columns for identity, naming, classification, and status, as schema columns with their controlled lists.
4. **Model list.** Filter, search, sort, group by drop, a per-user column choice, inline edit, and a CSV export of the current view.
5. **Custom fields.** The administrator can add more attributes to the main entities (season, drop, model) as a runtime configuration.
6. **Users and access.** An administrator creates a user and hands over a first password. Password change, deactivation, and session revocation.
7. **Audit trail.** Who changed what, when, and from which value to which value.

## 6. Not in v1

- Costing, price, and markup.
- Quantities. No channel split, no size grid, and no order value. The `Order by Size` tab has no equivalent.
- Planning stages. The design review and line review copies of the commercial figures.
- Reference entities. Fabric, partner, factory, and certificate stay values in a list, not records with their own screens.
- Season transitions. No copy-forward, no carry-over, and no postponement action.
- Reports and dashboards, including OTIF.
- Notifications, email, and comments.
- A mobile layout.
- Self-registration and password reset by email.

## 7. Screens

| Screen | Purpose |
|---|---|
| Login | Sign in, and the forced password change on the first login. |
| Model list | The main screen. The season assortment, filtered and edited in place. |
| Model page | One model: the attributes, the photos, the calendar, and the history. |
| Milestone list | The late work across the season, in one list. |
| Model create | A new model, with a calendar template applied. |
| Admin | Seasons, drops, users, controlled lists, milestone types, and milestone templates. |
