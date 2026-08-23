package audit

// Activation names the action an `active` flag move makes.
func Activation(active bool) string {
	if active {
		return ActionReactivated
	}
	return ActionDeactivated
}
