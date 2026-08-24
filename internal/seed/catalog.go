package seed

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"
	"time"

	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/platform/date"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/media"
)

// seasonName is the one season the demo dataset builds.
const seasonName = "27SPRING"

// The calendar the demo runs on, in days from the day the seed loads it.
const (
	seasonStartsIn = -60
	firstDropIn    = 180
	dropsApart     = 60
)

// design is one sketch of the season.
type design struct {
	article string

	// colors is how many colorways the design is made in.
	colors int
}

type drop struct {
	name    string
	designs []design
}

// plan is the demo season.
var plan = []drop{
	{
		name: "Дроп 1",
		designs: []design{
			{"TSH-1001", 4},
			{"TSH-1002", 4},
			{"HOD-1003", 4},
			{"TOP-1004", 3},
			{"SHR-1005", 3},
			{"LNG-1006", 3},
			{"TRS-1007", 3},
			{"JNS-1008", 3},
			{"TSH-1009", 2},
			{"TOP-1010", 2},
			{"SHR-1011", 2},
			{"LNG-1012", 2},
			{"HOD-1013", 2},
			{"TRS-1014", 2},
			{"JNS-1015", 2},
			{"SKT-1016", 2},
			{"DRS-1017", 2},
			{"DRS-1018", 1},
			{"ACC-1019", 1},
			{"UND-1020", 1},
		},
	},
	{
		name: "Дроп 2",
		designs: []design{
			{"TSH-2001", 4},
			{"TOP-2002", 4},
			{"JNS-2003", 4},
			{"TSH-2004", 3},
			{"SHR-2005", 3},
			{"LNG-2006", 3},
			{"HOD-2007", 3},
			{"TRS-2008", 3},
			{"DRS-2009", 3},
			{"TSH-2010", 2},
			{"TOP-2011", 2},
			{"SHR-2012", 2},
			{"LNG-2013", 2},
			{"HOD-2014", 2},
			{"TRS-2015", 2},
			{"JNS-2016", 2},
			{"SKT-2017", 2},
			{"DRS-2018", 1},
			{"JKT-2019", 1},
			{"ACC-2020", 1},
			{"UND-2021", 1},
		},
	},
	{
		name: "Дроп 3",
		designs: []design{
			{"HOD-3001", 4},
			{"JNS-3002", 4},
			{"TSH-3003", 3},
			{"SWT-3004", 3},
			{"TRS-3005", 3},
			{"OUT-3006", 3},
			{"TOP-3007", 2},
			{"SHR-3008", 2},
			{"LNG-3009", 2},
			{"HOD-3010", 2},
			{"SWT-3011", 2},
			{"TRS-3012", 2},
			{"JKT-3013", 2},
			{"OUT-3014", 2},
			{"DRS-3015", 2},
			{"SKT-3016", 1},
			{"DRS-3017", 1},
			{"ACC-3018", 1},
			{"UND-3019", 1},
		},
	},
}

// plannedDesigns is how many sketches the demo season holds.
func plannedDesigns() int {
	total := 0
	for _, drop := range plan {
		total += len(drop.designs)
	}
	return total
}

// plannedModels is how many rows those sketches make, one per color.
func plannedModels() int {
	total := 0
	for _, drop := range plan {
		for _, d := range drop.designs {
			total += d.colors
		}
	}
	return total
}

// Counts is what one run did to one kind of row.
type Counts struct {
	// Created counts the rows this run wrote.
	Created int

	// Skipped counts the rows an earlier run already wrote.
	Skipped int
}

// CatalogReport is what the catalog part of one run wrote.
type CatalogReport struct {
	Seasons Counts
	Drops   Counts
	Models  Counts
	Photos  int
}

// catalogRun is one pass over the catalog part of the dataset.
type catalogRun struct {
	deps  Deps
	today time.Time

	// limit is how many models this pass plans, counted over the season in drop
	// order.
	limit int

	// drawn holds each placeholder panel the pass painted, keyed by the color
	// and the position it was painted for.
	drawn map[panelKey][]byte

	report CatalogReport
}

// seedCatalog writes the season, its drops, models, and the placeholder
// photos of the models. It returns the drops of the season, in plan order,
// because the calendars are written onto the models of those drops.
func seedCatalog(ctx context.Context, deps Deps, opts Options) (CatalogReport, []catalog.Drop, error) {
	limit := opts.Models
	if limit == 0 {
		limit = plannedModels()
	}

	run := &catalogRun{
		deps:  deps,
		today: deps.Clock.Today(),
		limit: limit,
		drawn: make(map[panelKey][]byte),
	}

	season, err := run.season(ctx)
	if err != nil {
		return run.report, nil, err
	}
	drops, err := run.drops(ctx, season)
	if err != nil {
		return run.report, nil, err
	}
	if err := run.models(ctx, drops); err != nil {
		return run.report, nil, err
	}
	return run.report, drops, nil
}

// season returns the demo season, and writes it when no run wrote it yet.
func (r *catalogRun) season(ctx context.Context) (catalog.Season, error) {
	held, err := r.deps.Catalog.ListSeasons(ctx)
	if err != nil {
		return catalog.Season{}, err
	}
	for _, season := range held {
		if strings.EqualFold(season.Name, seasonName) {
			r.report.Seasons.Skipped++
			return season, nil
		}
	}

	season, err := r.deps.Catalog.CreateSeason(ctx, catalog.SeasonCreateParams{
		Name:      seasonName,
		StartDate: date.AddDays(r.today, seasonStartsIn),
	})
	if err != nil {
		return catalog.Season{}, fmt.Errorf("seed: creating season %s: %w", seasonName, err)
	}
	r.report.Seasons.Created++
	return season, nil
}

// drops returns every drop of the plan, in plan order, and writes the ones no
// run wrote yet.
func (r *catalogRun) drops(ctx context.Context, season catalog.Season) ([]catalog.Drop, error) {
	held, err := r.deps.Catalog.ListDrops(ctx, season.ID)
	if err != nil {
		return nil, err
	}

	rows := make([]catalog.Drop, len(plan))
	for i, drop := range plan {
		if at := indexOfDrop(held, drop.name); at >= 0 {
			rows[i] = held[at]
			r.report.Drops.Skipped++
			continue
		}

		written, err := r.deps.Catalog.CreateDrop(ctx, catalog.DropCreateParams{
			SeasonID:   season.ID,
			Name:       drop.name,
			TargetDate: date.AddDays(r.today, firstDropIn+i*dropsApart),
		})
		if err != nil {
			return nil, fmt.Errorf("seed: creating drop %s: %w", drop.name, err)
		}
		rows[i] = written
		r.report.Drops.Created++
	}
	return rows, nil
}

// indexOfDrop is where the name sits in held, or -1 when no drop holds it. The
// table is unique over the lower-cased name inside one season.
func indexOfDrop(held []catalog.Drop, name string) int {
	for i, drop := range held {
		if strings.EqualFold(drop.Name, name) {
			return i
		}
	}
	return -1
}

// models writes the designs of every drop of the plan into that drop.
func (r *catalogRun) models(ctx context.Context, drops []catalog.Drop) error {
	planned := 0

	for i, drop := range drops {
		held, err := r.articleCounts(ctx, drop.ID)
		if err != nil {
			return err
		}

		for _, d := range plan[i].designs {
			for nth := range d.colors {
				if planned == r.limit {
					return nil
				}
				index := planned
				planned++

				if held[d.article] > nth {
					r.report.Models.Skipped++
					continue
				}
				if err := r.model(ctx, drop, d.article, index); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// model writes one row of a design, and the photo strip that goes with it.
func (r *catalogRun) model(ctx context.Context, drop catalog.Drop, article string, index int) error {
	model, err := r.deps.Catalog.CreateModel(ctx, catalog.ModelCreateParams{
		DropID:  drop.ID,
		Article: article,
	})
	if err != nil {
		return fmt.Errorf("seed: creating model %s: %w", article, err)
	}
	r.report.Models.Created++

	return r.photos(ctx, model, index)
}

// articleCounts is how many models the drop already holds, per article.
func (r *catalogRun) articleCounts(ctx context.Context, dropID ids.ID) (map[string]int, error) {
	held, err := r.deps.Catalog.ListModels(ctx, catalog.ModelListParams{DropID: dropID})
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int, len(held))
	for _, model := range held {
		counts[model.Article]++
	}
	return counts, nil
}

// ---------------------------------------------------------------- photos

// The placeholder photo.
const (
	photoWidth  = 360
	photoHeight = 480

	bandFirstTop = 64
	bandApart    = 96
	bandHeight   = 40
)

// palette is what a placeholder is painted in.
var palette = []color.RGBA{
	{0x1c, 0x1c, 0x1e, 0xff}, // black
	{0xf2, 0xef, 0xe9, 0xff}, // off white
	{0x3b, 0x4a, 0x6b, 0xff}, // navy
	{0x8c, 0x7a, 0x66, 0xff}, // taupe
	{0x6b, 0x8f, 0x71, 0xff}, // sage
	{0xa8, 0x4b, 0x4b, 0xff}, // brick
	{0xd9, 0xc2, 0xa3, 0xff}, // sand
	{0x4f, 0x4f, 0x55, 0xff}, // graphite
}

// photoStrip is how many placeholder images the model at index gets. Every
// seventh row gets none, so the list also shows a model that carries no
// thumbnail.
func photoStrip(index int) int {
	if index%7 == 6 {
		return 0
	}
	return 1 + index%3
}

// photos stores the strip of one model.
func (r *catalogRun) photos(ctx context.Context, model catalog.Model, index int) error {
	for position := range photoStrip(index) {
		content, err := r.panel(index, position)
		if err != nil {
			return err
		}

		stored, err := r.deps.Media.Put(ctx, media.Blob{
			Kind:    media.Image,
			Name:    fmt.Sprintf("%s-%d.png", model.Article, position+1),
			Content: bytes.NewReader(content),
		})
		if err != nil {
			return fmt.Errorf("seed: storing a photo of model %s: %w", model.Article, err)
		}

		if _, err := r.deps.Catalog.AddPhoto(ctx, model.ID, stored.Key); err != nil {
			return fmt.Errorf("seed: attaching a photo to model %s: %w", model.Article, err)
		}
		r.report.Photos++
	}
	return nil
}

// panelKey names one placeholder panel: the palette entry it is painted in, and
// the place in the strip it is painted for.
type panelKey struct {
	color    int
	position int
}

// panel returns the placeholder painted for a row.
func (r *catalogRun) panel(index, position int) ([]byte, error) {
	at := panelKey{color: index % len(palette), position: position}
	if held, painted := r.drawn[at]; painted {
		return held, nil
	}

	content, err := paint(palette[at.color], position)
	if err != nil {
		return nil, err
	}
	r.drawn[at] = content
	return content, nil
}

// paint draws one demo photo: a flat panel in the color of the row, with a band
// that sits where the photo sits in the strip.
func paint(base color.RGBA, position int) ([]byte, error) {
	panel := image.NewRGBA(image.Rect(0, 0, photoWidth, photoHeight))
	draw.Draw(panel, panel.Bounds(), &image.Uniform{C: base}, image.Point{}, draw.Src)

	top := bandFirstTop + position*bandApart
	band := image.Rect(0, top, photoWidth, top+bandHeight)
	draw.Draw(panel, band, &image.Uniform{C: shade(base)}, image.Point{}, draw.Src)

	var out bytes.Buffer
	if err := png.Encode(&out, panel); err != nil {
		return nil, fmt.Errorf("seed: drawing a placeholder photo: %w", err)
	}
	return out.Bytes(), nil
}

// shade is the band color of a base: a light panel gets a darker band, and a
// dark panel gets a lighter one, so the band shows on either.
func shade(base color.RGBA) color.RGBA {
	const step = 0x30

	if luminance(base) > 0x80 {
		return color.RGBA{R: down(base.R, step), G: down(base.G, step), B: down(base.B, step), A: 0xff}
	}
	return color.RGBA{R: up(base.R, step), G: up(base.G, step), B: up(base.B, step), A: 0xff}
}

// luminance is how bright a color reads, on the same 0 to 255 scale as its
// components.
func luminance(c color.RGBA) int {
	return (299*int(c.R) + 587*int(c.G) + 114*int(c.B)) / 1000
}

func up(v, step uint8) uint8 {
	if int(v)+int(step) > 0xff {
		return 0xff
	}
	return v + step
}

func down(v, step uint8) uint8 {
	if int(v)-int(step) < 0 {
		return 0
	}
	return v - step
}
