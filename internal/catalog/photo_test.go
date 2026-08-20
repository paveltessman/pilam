package catalog

import (
	"errors"
	"slices"
	"testing"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/media"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

func TestAddPhotoAppendsToTheStripAndRecordsOnTheModel(t *testing.T) {
	svc, _, trail := newTestService(t)
	ctx := signedIn(t)
	_, _, model := spine(t, svc, ctx)
	trail.entries = nil

	for _, key := range []media.Key{firstKey, secondKey} {
		if _, err := svc.AddPhoto(ctx, model.ID, key); err != nil {
			t.Fatalf("AddPhoto: %v", err)
		}
	}

	photos, err := svc.ListPhotos(ctx, model.ID)
	if err != nil {
		t.Fatalf("ListPhotos: %v", err)
	}
	if len(photos) != 2 || photos[0].Position != 0 || photos[1].Position != 1 {
		t.Fatalf("The strip = %+v, want positions 0 and 1", photos)
	}
	if photos[0].MediaKey != firstKey {
		t.Errorf("The thumbnail is %q, want the first photo %q", photos[0].MediaKey, firstKey)
	}

	if want := []string{audit.ActionPhotoAdded, audit.ActionPhotoAdded}; !slices.Equal(trail.actions(), want) {
		t.Errorf("actions = %v, want %v", trail.actions(), want)
	}
	if entry := trail.entries[0]; entry.Entity != audit.EntityModel || entry.EntityID != model.ID {
		t.Errorf("entry = %s %s, want the model %s", entry.Entity, entry.EntityID, model.ID)
	}
}

func TestAddPhotoRefusesAKeyMediaDidNotIssue(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := signedIn(t)
	_, _, model := spine(t, svc, ctx)

	_, err := svc.AddPhoto(ctx, model.ID, media.Key("images/not-a-key.jpg"))
	rejects(t, err, FieldPhoto, validate.NotAllowed)
}

func TestAddPhotoRefusesAModelThatIsNotThere(t *testing.T) {
	svc, _, _ := newTestService(t)

	missing := ids.MustParse("01912345-6789-7abc-def0-000000000004")
	if _, err := svc.AddPhoto(signedIn(t), missing, firstKey); !errors.Is(err, ErrNoModel) {
		t.Errorf("AddPhoto error = %v, want %v", err, ErrNoModel)
	}
}

func TestRemovePhotoClosesTheGapItLeaves(t *testing.T) {
	svc, _, trail := newTestService(t)
	ctx := signedIn(t)
	_, _, model := spine(t, svc, ctx)

	var strip []Photo
	for _, key := range []media.Key{firstKey, secondKey, thirdKey} {
		photo, err := svc.AddPhoto(ctx, model.ID, key)
		if err != nil {
			t.Fatalf("AddPhoto: %v", err)
		}
		strip = append(strip, photo)
	}
	trail.entries = nil

	if err := svc.RemovePhoto(ctx, model.ID, strip[0].ID); err != nil {
		t.Fatalf("RemovePhoto: %v", err)
	}

	photos, err := svc.ListPhotos(ctx, model.ID)
	if err != nil {
		t.Fatalf("ListPhotos: %v", err)
	}
	if want := []media.Key{secondKey, thirdKey}; !slices.Equal(keysOf(photos), want) {
		t.Fatalf("The strip = %v, want %v", keysOf(photos), want)
	}
	if photos[0].Position != 0 || photos[1].Position != 1 {
		t.Errorf("positions = %d and %d, want 0 and 1", photos[0].Position, photos[1].Position)
	}

	entry := trail.only(t)
	if entry.Action != audit.ActionPhotoRemoved || entry.Old != string(firstKey) {
		t.Errorf("entry = %s %q, want %s %q", entry.Action, entry.Old, audit.ActionPhotoRemoved, firstKey)
	}
}

func TestRemovePhotoReportsAPhotoOfAnotherModel(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := signedIn(t)
	_, drop, model := spine(t, svc, ctx)

	other, err := svc.CreateModel(ctx, ModelCreateParams{DropID: drop.ID, Article: "A-200"})
	if err != nil {
		t.Fatalf("CreateModel: %v", err)
	}
	photo, err := svc.AddPhoto(ctx, other.ID, firstKey)
	if err != nil {
		t.Fatalf("AddPhoto: %v", err)
	}

	if err := svc.RemovePhoto(ctx, model.ID, photo.ID); !errors.Is(err, ErrNoPhoto) {
		t.Errorf("RemovePhoto error = %v, want %v", err, ErrNoPhoto)
	}
}

func TestReorderPhotosMakesTheNamedPhotoTheThumbnail(t *testing.T) {
	svc, _, trail := newTestService(t)
	ctx := signedIn(t)
	_, _, model := spine(t, svc, ctx)

	var strip []Photo
	for _, key := range []media.Key{firstKey, secondKey, thirdKey} {
		photo, err := svc.AddPhoto(ctx, model.ID, key)
		if err != nil {
			t.Fatalf("AddPhoto: %v", err)
		}
		strip = append(strip, photo)
	}
	trail.entries = nil

	order := []ids.ID{strip[2].ID, strip[0].ID, strip[1].ID}
	if err := svc.ReorderPhotos(ctx, model.ID, order); err != nil {
		t.Fatalf("ReorderPhotos: %v", err)
	}

	photos, err := svc.ListPhotos(ctx, model.ID)
	if err != nil {
		t.Fatalf("ListPhotos: %v", err)
	}
	if want := []media.Key{thirdKey, firstKey, secondKey}; !slices.Equal(keysOf(photos), want) {
		t.Errorf("The strip = %v, want %v", keysOf(photos), want)
	}

	entry := trail.only(t)
	if entry.Action != audit.ActionPhotoReordered {
		t.Errorf("action = %q, want %q", entry.Action, audit.ActionPhotoReordered)
	}
	if entry.Old == entry.New {
		t.Errorf("The entry records the same order on both sides: %q", entry.Old)
	}
}

func TestReorderPhotosRefusesAListThatIsNotTheStrip(t *testing.T) {
	svc, _, trail := newTestService(t)
	ctx := signedIn(t)
	_, _, model := spine(t, svc, ctx)

	var strip []Photo
	for _, key := range []media.Key{firstKey, secondKey} {
		photo, err := svc.AddPhoto(ctx, model.ID, key)
		if err != nil {
			t.Fatalf("AddPhoto: %v", err)
		}
		strip = append(strip, photo)
	}
	trail.entries = nil

	stranger := ids.MustParse("01912345-6789-7abc-def0-000000000005")
	for name, order := range map[string][]ids.ID{
		"one photo left out": {strip[0].ID},
		"one photo twice":    {strip[0].ID, strip[0].ID},
		"a photo elsewhere":  {strip[0].ID, stranger},
	} {
		t.Run(name, func(t *testing.T) {
			if err := svc.ReorderPhotos(ctx, model.ID, order); !errors.Is(err, ErrPhotoOrder) {
				t.Errorf("ReorderPhotos error = %v, want %v", err, ErrPhotoOrder)
			}
		})
	}

	photos, err := svc.ListPhotos(ctx, model.ID)
	if err != nil {
		t.Fatalf("ListPhotos: %v", err)
	}
	if want := []media.Key{firstKey, secondKey}; !slices.Equal(keysOf(photos), want) {
		t.Errorf("The strip moved to %v, want the unchanged %v", keysOf(photos), want)
	}
	if len(trail.entries) != 0 {
		t.Errorf("The trail holds %d entries, want 0", len(trail.entries))
	}
}

func TestReorderPhotosWritesNothingWhenTheOrderStands(t *testing.T) {
	svc, _, trail := newTestService(t)
	ctx := signedIn(t)
	_, _, model := spine(t, svc, ctx)

	var strip []ids.ID
	for _, key := range []media.Key{firstKey, secondKey} {
		photo, err := svc.AddPhoto(ctx, model.ID, key)
		if err != nil {
			t.Fatalf("AddPhoto: %v", err)
		}
		strip = append(strip, photo.ID)
	}
	trail.entries = nil

	if err := svc.ReorderPhotos(ctx, model.ID, strip); err != nil {
		t.Fatalf("ReorderPhotos: %v", err)
	}
	if len(trail.entries) != 0 {
		t.Errorf("The trail holds %d entries, want 0", len(trail.entries))
	}
}

// The list asks for a whole page of covers at once, so the service passes the
// identifiers through and reads the store once.
func TestThumbnailsReadTheStoreOnce(t *testing.T) {
	svc, rows, _ := newTestService(t)
	ctx := signedIn(t)
	_, drop, model := spine(t, svc, ctx)

	if _, err := svc.AddPhoto(ctx, model.ID, firstKey); err != nil {
		t.Fatalf("AddPhoto: %v", err)
	}
	if _, err := svc.AddPhoto(ctx, model.ID, secondKey); err != nil {
		t.Fatalf("AddPhoto: %v", err)
	}

	bare, err := svc.CreateModel(ctx, ModelCreateParams{DropID: drop.ID, Article: "A-200"})
	if err != nil {
		t.Fatalf("CreateModel: %v", err)
	}

	covers, err := svc.Thumbnails(ctx, model.ID, bare.ID)
	if err != nil {
		t.Fatalf("Thumbnails: %v", err)
	}
	if len(covers) != 1 {
		t.Fatalf("Thumbnails returned %d covers, want 1: %+v", len(covers), covers)
	}
	if got := covers[model.ID].MediaKey; got != firstKey {
		t.Errorf("cover = %q, want the first photo %q", got, firstKey)
	}

	// A page with no rows on it asks the store nothing at all.
	rows.failWith = errors.New("the store was asked")
	if covers, err := svc.Thumbnails(ctx); err != nil || len(covers) != 0 {
		t.Errorf("Thumbnails of no models = %+v, %v, want an empty map and no error", covers, err)
	}
}
