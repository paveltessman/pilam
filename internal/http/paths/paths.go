// Package paths holds every route path of the transport layer.
//
// It is the leaf of the http tree: it imports nothing from internal/http, so
// that a screen package and the shared layout can both name a path without an
// import cycle between them.
//
// A path with a {name} in it is a route pattern. A path without one is also a
// link the browser can follow.
package paths

// Login is the sign-in screen. Logout ends the session.
const (
	Login  = "/login"
	Logout = "/logout"
)

// ChangePassword is the screen a user with an expired password reaches.
const ChangePassword = "/password"

// Root carries no screen of its own. It sends the browser to Models.
const Root = "/{$}"

// The catalog screens.
const (
	Models   = "/models"
	ModelNew = Models + "/new"
	Model    = Models + "/{id}"

	ModelEdit       = Model + "/edit"
	ModelPhotos     = Model + "/photos"
	ModelPhotosEdit = ModelPhotos + "/edit"
	ModelOrder      = ModelPhotos + "/order"
	ModelPhoto      = ModelPhotos + "/{photoID}/remove"

	// The calendar section of the model card. A milestone carries no screen of
	// its own: every one of these posts back to the card.
	ModelMilestones     = Model + "/milestones"
	ModelMilestonesEdit = ModelMilestones + "/edit"
	ModelTemplate       = ModelMilestones + "/template"
	ModelMilestone      = ModelMilestones + "/{milestoneID}"
	ModelMilestoneEdit  = ModelMilestone + "/edit"

	// The plan date of one step carries the steps after it. This reads what a
	// date would carry, so that the dialog states the rule before it writes.
	ModelMilestonePlan = ModelMilestone + "/plan"

	Seasons   = "/seasons"
	SeasonNew = Seasons + "/new"
	Season    = Seasons + "/{id}"

	Drops   = "/drops"
	DropNew = Drops + "/new"
	Drop    = Drops + "/{id}"
)

// The users section. A member reaches none of it.
const (
	Users    = "/users"
	UserNew  = Users + "/new"
	User     = Users + "/{id}"
	UserPass = User + "/password"
)

// The milestone screens. The section screen lists the templates and the types;
// each of the two has its own create screen and its own card.
const (
	Milestones = "/milestones"

	// MilestoneList is the late work of a whole season. It is the one milestone
	// screen a member reaches, so it does not sit under the root guard.
	MilestoneList = Milestones + "/list"

	MilestoneTypes   = Milestones + "/types"
	MilestoneTypeNew = MilestoneTypes + "/new"
	MilestoneType    = MilestoneTypes + "/{id}"

	MilestoneTemplates   = Milestones + "/templates"
	MilestoneTemplateNew = MilestoneTemplates + "/new"
	MilestoneTemplate    = MilestoneTemplates + "/{id}"

	// The items of one template. They carry no screen of their own: every one
	// of these posts back to the template card.
	MilestoneItems      = MilestoneTemplate + "/items"
	MilestoneItemsOrder = MilestoneItems + "/order"
	MilestoneItem       = MilestoneItems + "/{itemID}"
	MilestoneItemRemove = MilestoneItem + "/remove"
)
