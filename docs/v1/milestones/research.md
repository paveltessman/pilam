# Research: milestones

Prior art for `docs/v1/milestones/feature.md`. It records what other people built, what they got wrong, and what that means for our design.

Date of the search: 22.08.2026.

---

## 0. What does not exist

No engineer published a post-mortem about building a time and action calendar for a fashion brand. The apparel vendors publish marketing pages. The apparel practitioners write about the process and not about the schema.

The useful material sits in three other places:

- Apparel process writers, who name the failures of the current practice.
- Forum threads and documents from tools that already shipped auto-shift and baselines.
- Clinical trial software and legal docketing software, where an anchor date plus an offset is the whole product.

---

## 1. The apparel writers

### 1.1 The limits of the conventional TNA

[Apparel Resources, limitations in the conventional approach to TNA](https://apparelresources.com/business-news/manufacturing/time-action-calendar-apparel-merchandising-limitations-conventional-approach-tna/) lists the failures of the practice. The list does not match ours.

- Durations are set in days or hours, and never in man-days or man-hours.
- 5 or 6 executives run 50 to 60 dependent activities across 8 to 10 orders, with no combined priority.
- Two executives hold two to-do lists, and nothing synchronizes them.
- Each order keeps its own TNA, and nobody joins them into one schedule.

The proposed fix is critical chain and multi-order integration, not a better date model.

Read this against §11 of the design. We move owners to feature 3. The domain literature says the people are the constraint that makes a calendar lie. The v1 decision still holds, because a calendar without an owner already beats a workbook cell that overwrites the plan. Expect the first real complaint to be about who, and not about when.

### 1.2 Backward planning and the 1-week rule

[Kōbō, critical path management for fashion production](https://www.kobolabs.io/blog/ops/critical-path) describes the same chain we seed: design approval, fabric sourcing, proto, fit sample, PP sample, bulk order, production, inspection, warehouse arrival. It plans backwards from the delivery date, the way our offsets do.

It carries one number worth keeping. A one week slip rarely stays one week, because factory slots and shipping windows do not stretch. That is the argument for the cascade of §7.1, written by somebody who runs the process.

### 1.3 The steps of critical path management

[Online Clothing Study, 5 key steps of critical path management](https://www.onlineclothingstudy.com/2014/02/5-key-steps-of-critical-path-management.html) gives the loop: define events, durations, and owners; set targets; update the status daily; manage the exceptions and re-plan; measure the result.

Step 3 is the reason our state is derived and not stored. The value of the calendar comes from a daily read, not from a report at the end.

---

## 2. The tools that shipped these mechanics

### 2.1 Asana, auto-shift of dependent dates

The most useful single link of the search: [Asana forum, "Don't auto-shift dates" turns off all date shifting for the project](https://forum.asana.com/t/dont-auto-shift-dates-option-turns-off-all-date-shifting-for-the-project/1053756).

Asana made auto-shift a project-level setting. A user asks for a fixed default plus the option to skip the shift in one case, without turning the setting off for the whole project. The all-or-nothing switch is the complaint.

Our §7.1 puts the choice in the confirm dialog of each edit, with the list of milestones that move. That is the design the Asana user asks for.

The second half of the behaviour appears in [the community tool that shifts all dates of a project](https://forum.asana.com/t/tool-shift-all-dates-of-a-project-by-a-number-of-days/200421), which moves the incomplete tasks only. See also [the Asana help page on auto-shifting dates for dependent tasks](https://help.asana.com/s/article/auto-shifting-dates-for-dependent-tasks?language=en_US). Our rule that a fact date freezes a milestone matches the convention.

### 2.2 Jira, the missing baseline

Old Jira Portfolio held a baseline. Jira Plans dropped it, and the request to return it stays open as JSWCLOUD-20495. See [the Atlassian community thread on baselines in Jira Plans](https://community.atlassian.com/forums/Jira-questions/Using-Jira-Plans-formally-portfolio-how-to-manage-baselines/qaq-p/2239965) and [the thread on Advanced Roadmaps](https://community.atlassian.com/forums/Advanced-Planning-in-Jira/Do-we-have-a-way-to-compar-and-track-against-baseline-in-the/td-p/1418281).

[Tempo sells a Gantt that builds a baseline by reading a Jira custom field](https://help.tempo.io/gantt-dc/latest/jira-based-baselines), or a formula, or the first transition to `In Progress`. That is a paid workaround for a missing column.

This supports §12 directly. A large planning product removed the third date, and users asked for it back for years. Our quiet baseline column costs one field and answers the failure that PRD §1 names.

### 2.3 A Gantt rebuilt around load

[dev.to, from task-centric to resource-aware](https://dev.to/nelson_li_c5265341756c7ab/from-task-centric-to-resource-aware-rebuilding-a-gantt-chart-for-real-projects-17lc) reports the first version was good at when and bad at who. The line worth keeping: a schedule looks correct right up until it fails, because the load accumulates invisibly.

Same warning as §1.1, from the code side.

---

## 3. The adjacent domains that solved our model

### 3.1 Clinical trials: anchor, offset, window

A protocol defines every visit as an offset from an anchor date, such as day 1, randomization, or the last dose. The system holds a target date and an actual date. A visit outside its window is a protocol deviation. See [the CRC Toolbox visit window calculator](https://crctoolbox.com/visit-calculator) and [PharmaSUG 2011, keeping patients on schedule, the art of visit windows](https://pharmasug.org/proceedings/2011/TT/PharmaSUG-2011-TT07.pdf).

[A USPTO filing on visit scheduling](https://image-ppubs.uspto.gov/dirsearch-public/print/downloadPdf/11545240) describes a model where one visit carries a time-based anchor to a fixed origin and a sequence-based anchor to the previous visit, at the same time.

That is §3.2 of our design. The offset from the target date of the drop is the time-based anchor. The gap to the item above is the sequence-based anchor. We show both and store the offset, which is the cheaper choice. The filing shows the richer model is a real design if we ever need it.

They hold one thing we do not: a tolerance window per step, such as day 28 plus or minus 3 days. Our `due inside 7 days` is one global number, and it warns rather than judges. If somebody says a 5-day fit approval and a 60-day transit must not warn the same way, a per-type window is the shape of the answer. §5 already marks the number as a v1 constant.

### 3.2 Legal docketing: the same engine with a calendar

Deadline engines compute from a trigger date plus an offset, and they treat court holidays, weekends, and the difference between business days and calendar days as core. They also keep the rule citation beside each computed date, for the audit. See [Filevine on legal deadline calculators](https://www.filevine.com/blog/legal-deadline-calculators-your-tool-for-timely-filing/) and [LawToolBox](https://lawtoolbox.com/legal-deadline-calculator/).

§11 excludes working days, and the reason in §6 is sound: the lead times of the trade already absorb them. Keep the decision. The risk sits in the short steps, such as the 5 days from the PPS to the fit approval and the 7 days to ready for sale. A 5-day step that lands on a Saturday is the case a user will report.

---

## 4. Cross-cutting engineering notes

### 4.1 Derived state beats a stored status column

[FlowFuse, tracking instrument calibration with a digital dashboard](https://flowfuse.com/blog/2026/07/calibration-management-dashboard/) makes our §5 argument in one sentence. Whatever writes a stored status column can fall behind, and the dashboard then reports a compliant system while the due dates quietly pass. Their fix is to compute overdue and due-soon from the dates.

The cost side is a query with a date predicate on every read. [thoughtbot, modeling state transitions in Postgres](https://thoughtbot.com/blog/modeling-state-transitions-in-postgres) covers the index work that keeps such reads cheap, including a covering index that turns the scan into an index-only scan.

### 4.2 Today comes from the business timezone

The failure pattern repeats across the posts: any time-dependent logic that assumes the server timezone equals the business timezone breaks in a UTC environment. See [handling timezone issues in cron jobs](https://dev.to/cronmonitor/handling-timezone-issues-in-cron-jobs-2025-guide-52ii).

[Oracle CX recomputes overdue on every task edit, and again daily at midnight of the chosen timezone](https://docs.oracle.com/cd/E80480_01/help/en/user/136745.htm). Our derived state removes that job. Keep one function that returns today in the business timezone, and test it.

### 4.3 The audit trail wants the request, not only the field

The common field-level pattern is one row per changed field: the table, the record id, the field name, the old value, the new value, the operation, the actor, the timestamp, and the request that caused the change. See [audit log paradigms and Postgres design patterns](https://dev.to/akkaraponph/comprehensive-research-audit-log-paradigms-gopostgresqlgorm-design-patterns-1jmm) and [the audit trail, building a system that remembers](https://dev.to/itxshakil/the-audit-trail-building-a-system-that-remembers-4bh0).

§9 lists actions and not fields. The request id is the missing piece. It is what groups the 15 rows of a drop shift into one event that a user can read.

### 4.4 The bulk action bar

The shipped pattern for §8.2 is a contextual toolbar that appears when a selection exists. Jira and ClickUp float it, and Gmail docks it. See [PatternFly bulk selection](https://www.patternfly.org/patterns/bulk-selection/), [Helios table multi-select](https://helios.hashicorp.design/patterns/table-multi-select), [eBay bulk editing](https://playbook.ebay.com/design-system/patterns/bulk-editing), and [Eleken, 8 guidelines for bulk action UX](https://www.eleken.co/blog-posts/bulk-actions-ux).

Both eBay and Basis say the same thing: bulk edit does not replace the single-row edit. It is a fast path for a few common actions. That matches §7.2, which offers exactly two.

---

## 5. What this suggests for the design

Nothing in the research argues against a decision the design already made. The baseline, the freeze on a fact date, the confirm dialog per edit, and the computed state all carry shipped precedent.

Three additions are worth a look:

1. **§9.** Record the request that caused a batch of writes. A drop shift then reads as one event with 15 lines.
2. **§11.** Say that owners are the known next constraint, and that the apparel literature names them. It protects the decision instead of leaving it to look like an oversight.
3. **§5.** Say the 7-day window is global on purpose, and that a per-type window is the known next step. The clinical trial visit window is the precedent.
