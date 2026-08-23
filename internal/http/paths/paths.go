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
	Models      = "/models"
	ModelNew    = Models + "/new"
	Model       = Models + "/{id}"
	ModelPhotos = Model + "/photos"
	ModelOrder  = ModelPhotos + "/order"
	ModelPhoto  = ModelPhotos + "/{photoID}/remove"

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

// The milestone screens.
const (
	MilestoneTypes   = "/milestone-types"
	MilestoneTypeNew = MilestoneTypes + "/new"
	MilestoneType    = MilestoneTypes + "/{id}"
)
