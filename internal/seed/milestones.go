package seed

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/milestones"
	"github.com/paveltessman/pilam/internal/platform/date"
	"github.com/paveltessman/pilam/internal/platform/ids"
)

// The one template the demo brand runs on. A second template changes the transit
// time alone, and the design leaves it out of the dataset.
const (
	templateName        = "Импорт, ж/д"
	templateDescription = "Основной критический путь: производство в Азии и доставка железной дорогой."
)

// step is one milestone type of the critical path, with the days it runs from
// the target date of the drop.
type step struct {
	name        string
	description string

	// offset is days from the target date of the drop. Zero or negative.
	offset int
}

// criticalPath is the chain every model of the demo brand runs through, in the
// order the steps run. The offsets are the lead times the workbook holds inside
// cell formulas: 270 days from the first tech pack to the day the goods go on
// sale, and 60 of them the rail transit.
var criticalPath = []step{
	{"ТП на 1-й образец", "Технический пакет на первый образец передан на фабрику.", -270},
	{"1-й образец получен", "Первый образец пришёл с фабрики и готов к примерке.", -240},
	{"ТП на 2-й образец", "Технический пакет с правками после первой примерки передан на фабрику.", -230},
	{"2-й образец получен", "Второй образец пришёл с фабрики и готов к примерке.", -200},
	{"ТП на ППО", "Технический пакет на предпроизводственный образец передан на фабрику.", -190},
	{"ППО получен", "Предпроизводственный образец пришёл с фабрики.", -160},
	{"Посадка утверждена", "Посадку изделия утвердили конструктор и технолог.", -155},
	{"ТП на производство", "Технический пакет на массовое производство передан на фабрику.", -150},
	{"Производство начато", "Фабрика запустила изделие в массовое производство.", -140},
	{"Готово к инспекции", "Партия дошита и готова к инспекции качества.", -95},
	{"Инспекция пройдена", "Инспекция качества пройдена, партия принята.", -88},
	{"Фотообразец передан", "Фотообразец передан на съёмку для карточки товара.", -88},
	{"Отгружено", "Партия отгружена с фабрики и вышла в путь.", -74},
	{"Прибыло на склад", "Партия принята на склад и разнесена по остаткам.", -14},
	{"Готово к продаже", "Карточка товара собрана, изделие готово к выкладке.", -7},
}

// MilestonesReport is what the milestone part of one run wrote.
type MilestonesReport struct {
	Types     Counts
	Templates Counts

	// Steps counts the items of the template.
	Steps Counts

	// Calendars counts the models the template was applied to.
	Calendars Counts

	// Facts counts the steps this run stamped a fact date on.
	Facts int
}

// milestoneRun is one pass over the milestone part of the dataset.
type milestoneRun struct {
	deps  Deps
	today time.Time

	// templateID is the demo template every calendar is built from.
	templateID ids.ID

	report MilestonesReport
}

// seedMilestones writes the milestone types of the critical path, the template
// that orders them, and the calendar of every model of the drops given.
func seedMilestones(ctx context.Context, deps Deps, drops []catalog.Drop) (MilestonesReport, error) {
	run := &milestoneRun{deps: deps, today: deps.Clock.Today()}

	types, err := run.types(ctx)
	if err != nil {
		return run.report, err
	}
	template, err := run.template(ctx, types)
	if err != nil {
		return run.report, err
	}
	run.templateID = template.ID

	if err := run.calendars(ctx, drops); err != nil {
		return run.report, err
	}
	return run.report, nil
}

// types returns the milestone type of every step of the critical path, in path
// order, and writes the ones no run wrote yet.
func (r *milestoneRun) types(ctx context.Context) ([]milestones.Type, error) {
	held, err := r.deps.Milestones.ListTypes(ctx)
	if err != nil {
		return nil, err
	}

	rows := make([]milestones.Type, len(criticalPath))
	for i, one := range criticalPath {
		if at := indexOfType(held, one.name); at >= 0 {
			rows[i] = held[at]
			r.report.Types.Skipped++
			continue
		}

		written, err := r.deps.Milestones.CreateType(ctx, milestones.TypeCreateParams{
			Name:        one.name,
			Description: one.description,
		})
		if err != nil {
			return nil, fmt.Errorf("seed: creating milestone type %s: %w", one.name, err)
		}
		rows[i] = written
		r.report.Types.Created++
	}
	return rows, nil
}

// indexOfType is where the short name sits in held, or -1 when no type holds
// it. The table is unique over the lower-cased name.
func indexOfType(held []milestones.Type, name string) int {
	for i, one := range held {
		if strings.EqualFold(one.Name, name) {
			return i
		}
	}
	return -1
}

// template returns the demo template, with one item per step of the critical
// path, and writes what no run wrote yet.
//
// types is the type of every step, in path order.
func (r *milestoneRun) template(ctx context.Context, types []milestones.Type) (milestones.Template, error) {
	template, err := r.namedTemplate(ctx)
	if err != nil {
		return milestones.Template{}, err
	}

	held, err := r.deps.Milestones.TemplateItems(ctx, template.ID)
	if err != nil {
		return milestones.Template{}, err
	}
	onTemplate := make(map[ids.ID]bool, len(held))
	for _, item := range held {
		onTemplate[item.TypeID] = true
	}

	// An item lands at the end of the list, so a run over an empty template
	// writes the path in path order.
	for i, one := range criticalPath {
		if onTemplate[types[i].ID] {
			r.report.Steps.Skipped++
			continue
		}
		if _, err := r.deps.Milestones.AddTemplateItem(ctx, template.ID, types[i].ID, one.offset); err != nil {
			return milestones.Template{}, fmt.Errorf("seed: adding step %s to the template: %w", one.name, err)
		}
		r.report.Steps.Created++
	}
	return template, nil
}

// namedTemplate returns the demo template, and writes it when no run wrote it
// yet. The template starts as the default one, which is what a new model takes.
func (r *milestoneRun) namedTemplate(ctx context.Context) (milestones.Template, error) {
	held, err := r.deps.Milestones.ListTemplates(ctx)
	if err != nil {
		return milestones.Template{}, err
	}
	for _, template := range held {
		if strings.EqualFold(template.Name, templateName) {
			r.report.Templates.Skipped++
			return template, nil
		}
	}

	template, err := r.deps.Milestones.CreateTemplate(ctx, milestones.TemplateCreateParams{
		Name:        templateName,
		Description: templateDescription,
		Default:     true,
	})
	if err != nil {
		return milestones.Template{}, fmt.Errorf("seed: creating template %s: %w", templateName, err)
	}
	r.report.Templates.Created++
	return template, nil
}

// ---------------------------------------------------------------- calendars

// late is one model that runs behind: where it sits in its drop, and how many
// of its past steps hold no fact date.
type late struct {
	at   int
	open int
}

// lateAt names the models that run late, per drop of the plan.
//
// A step reads as late once its plan date has passed with no fact date on it, so
// only the drops that go on sale first hold a model that can be late at all. The
// spread is uneven on purpose: the milestone list has to open on a handful of
// rows across the season, not on one row per drop.
var lateAt = [][]late{
	{{2, 1}, {5, 2}, {11, 1}, {18, 3}, {27, 1}, {34, 2}, {41, 1}},
	{{7, 1}, {23, 1}, {38, 1}},
	{},
}

// jitter is the days a step landed away from the day the calendar promised. The
// model and the step pick one, so a seeded season holds steps that ran early,
// steps that landed on the day, and steps that ran late.
var jitter = []int{0, -1, 3, 0, 7, -2, 0, 14, 1, 0, -3, 5}

// reasons is what somebody wrote in the note when a plan date moved.
var reasons = []string{
	"Фабрика сдвинула сроки: линия занята другим заказом.",
	"Ткань пришла на фабрику позже плана.",
	"Замечания по посадке, потребовался повторный образец.",
	"Ждали утверждение цвета от дизайнера.",
	"Фабрика закрывалась на национальные праздники.",
}

// calendars writes the calendar of every model of every drop, and stamps the
// facts a real season would already hold.
//
// drops arrives in plan order, which is the order lateAt names the drops in.
func (r *milestoneRun) calendars(ctx context.Context, drops []catalog.Drop) error {
	for i, drop := range drops {
		models, err := r.deps.Catalog.ListModels(ctx, catalog.ModelListParams{DropID: drop.ID})
		if err != nil {
			return err
		}
		for index, model := range models {
			if err := r.calendar(ctx, model, index, openSteps(i, index)); err != nil {
				return err
			}
		}
	}
	return nil
}

// openSteps is how many of the past steps of one model hold no fact date. It is
// zero for every model that is not on the late list.
func openSteps(dropAt, index int) int {
	if dropAt >= len(lateAt) {
		return 0
	}
	for _, one := range lateAt[dropAt] {
		if one.at == index {
			return one.open
		}
	}
	return 0
}

// calendar applies the template to one model and stamps what the model already
// lived through. index is where the model sits in its drop.
//
// A model that already holds a calendar takes nothing: an earlier run wrote it,
// with the facts that went with it.
func (r *milestoneRun) calendar(ctx context.Context, model catalog.Model, index, open int) error {
	written, err := r.deps.Milestones.ApplyTemplate(ctx, model.ID, r.templateID)
	if err != nil {
		return fmt.Errorf("seed: building the calendar of model %s: %w", model.Article, err)
	}
	if len(written) == 0 {
		r.report.Calendars.Skipped++
		return nil
	}
	r.report.Calendars.Created++

	return r.facts(ctx, model, written, index, open)
}

// facts stamps the steps of one calendar that a season already ran through:
// every step whose plan date has passed, less the open ones a late model
// carries at the end of that run.
//
// written arrives in path order, which is plan date order, because the offsets
// of the template run forwards.
func (r *milestoneRun) facts(
	ctx context.Context,
	model catalog.Model,
	written []milestones.Milestone,
	index, open int,
) error {
	var past []milestones.Milestone
	for _, one := range written {
		if date.Before(one.Plan, r.today) {
			past = append(past, one)
		}
	}

	done := max(len(past)-open, 0)
	for step, one := range past[:done] {
		fact := r.landedOn(one, index+step)

		// A step that ran late moved the plan date to the day it really
		// happened, and somebody wrote why. A step that landed on the day or
		// ran early kept the date the calendar promised.
		plan, note := one.Baseline, ""
		if date.After(fact, one.Baseline) {
			plan = fact
			note = reasons[(index+step)%len(reasons)]
		}

		// The plan date is a write of its own, and the shift stays off: the
		// seed states the date of every step itself.
		if !date.Equal(plan, one.Plan) {
			if _, err := r.deps.Milestones.MovePlan(ctx, one.ID, plan, false); err != nil {
				return fmt.Errorf("seed: moving the plan date of a step of model %s: %w", model.Article, err)
			}
		}

		err := r.deps.Milestones.UpdateMilestone(ctx, one.ID, milestones.MilestoneUpdateParams{
			Fact:   fact,
			Note:   note,
			Active: true,
		})
		if err != nil {
			return fmt.Errorf("seed: stamping a step of model %s: %w", model.Article, err)
		}
		r.report.Facts++
	}
	return nil
}

// landedOn is the day one step really happened: the day the calendar promised,
// moved by the jitter the model and the step pick.
//
// A fact date is never later than today, because the app refuses one.
func (r *milestoneRun) landedOn(one milestones.Milestone, pick int) time.Time {
	fact := date.AddDays(one.Baseline, jitter[pick%len(jitter)])
	if date.After(fact, r.today) {
		return r.today
	}
	return fact
}
