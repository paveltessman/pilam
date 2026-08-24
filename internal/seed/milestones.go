package seed

import (
	"context"
	"fmt"
	"strings"

	"github.com/paveltessman/pilam/internal/milestones"
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
}

// milestoneRun is one pass over the milestone part of the dataset.
type milestoneRun struct {
	deps   Deps
	report MilestonesReport
}

// seedMilestones writes the milestone types of the critical path and the
// template that orders them.
func seedMilestones(ctx context.Context, deps Deps) (MilestonesReport, error) {
	run := &milestoneRun{deps: deps}

	types, err := run.types(ctx)
	if err != nil {
		return run.report, err
	}
	if _, err := run.template(ctx, types); err != nil {
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
