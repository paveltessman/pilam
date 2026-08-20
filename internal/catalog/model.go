package catalog

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

// Model is one article.
type Model struct {
	ID      ids.ID
	DropID  ids.ID
	Article string
	Active  bool
}

// Models is the store of model rows. ByID returns ErrNoModel when nothing
// matches.
type Models interface {
	ByID(ctx context.Context, id ids.ID) (Model, error)

	// List returns the models the filter keeps, ordered by article.
	List(ctx context.Context, filter ModelListParams) ([]Model, error)

	Create(ctx context.Context, model Model) error
	Update(ctx context.Context, model Model) error
}

// ModelListParams is the arguments for ListModels.
type ModelListParams struct {
	SeasonID ids.ID
	DropID   ids.ID

	// Active keeps the active rows when it points at true, the deactivated
	// rows when it points at false, and both when it is nil.
	Active *bool
}

// ModelCreateParams is the arguments for CreateModel.
type ModelCreateParams struct {
	DropID  ids.ID
	Article string
}

// ModelUpdateParams is the arguments for UpdateModel.
type ModelUpdateParams struct {
	Article string
	Active  bool
}

// Model returns one model.
func (s *Service) Model(ctx context.Context, modelID ids.ID) (Model, error) {
	model, err := s.models.ByID(ctx, modelID)
	if err != nil {
		return Model{}, fmt.Errorf("catalog: loading model %s: %w", modelID, err)
	}
	return model, nil
}

// ListModels returns the models the filter keeps, ordered by article.
func (s *Service) ListModels(ctx context.Context, filter ModelListParams) ([]Model, error) {
	models, err := s.models.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("catalog: listing models: %w", err)
	}
	return models, nil
}

// CreateModel writes a new model inside a drop that already exists.
func (s *Service) CreateModel(ctx context.Context, in ModelCreateParams) (Model, error) {
	article := strings.TrimSpace(in.Article)

	var v validate.Validator
	v.Required(FieldArticle, article)
	v.Check(in.DropID != ids.Nil, FieldDrop, validate.Required)
	if err := v.Err(); err != nil {
		return Model{}, err
	}

	if _, err := s.drops.ByID(ctx, in.DropID); err != nil {
		return Model{}, fmt.Errorf("catalog: loading drop %s: %w", in.DropID, err)
	}

	model := Model{
		ID:      s.ids.New(),
		DropID:  in.DropID,
		Article: article,
		Active:  true,
	}

	write := func(ctx context.Context) error {
		if err := s.models.Create(ctx, model); err != nil {
			return err
		}
		return s.record(ctx, change(audit.EntityModel, model.ID, audit.ActionCreated, "", "", model.Article))
	}
	if err := s.atomic.InTx(ctx, write); err != nil {
		return Model{}, err
	}
	return model, nil
}

// UpdateModel writes the edited fields and records one trail entry per field
// that moved. It writes nothing at all when nothing moved.
func (s *Service) UpdateModel(ctx context.Context, modelID ids.ID, in ModelUpdateParams) error {
	article := strings.TrimSpace(in.Article)

	var v validate.Validator
	v.Required(FieldArticle, article)
	if err := v.Err(); err != nil {
		return err
	}

	model, err := s.models.ByID(ctx, modelID)
	if err != nil {
		return fmt.Errorf("catalog: loading model %s: %w", modelID, err)
	}

	var changes []audit.Change
	if article != model.Article {
		changes = append(changes, change(audit.EntityModel, model.ID, audit.ActionChanged, FieldArticle, model.Article, article))
		model.Article = article
	}
	if in.Active != model.Active {
		changes = append(changes, change(audit.EntityModel, model.ID, activation(in.Active), FieldActive,
			strconv.FormatBool(model.Active), strconv.FormatBool(in.Active)))
		model.Active = in.Active
	}

	if len(changes) == 0 {
		return nil
	}
	return s.atomic.InTx(ctx, func(ctx context.Context) error {
		if err := s.models.Update(ctx, model); err != nil {
			return err
		}
		return s.record(ctx, changes...)
	})
}
