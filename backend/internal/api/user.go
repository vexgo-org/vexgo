package api

// UserSummary is the only user shape a post, comment or page response carries:
// enough to attribute content, nothing else. Avatar is filled where the caller
// already preloaded it (comments); elsewhere it stays empty.
type UserSummary struct {
	ID       uint   `json:"id" example:"42"`
	Username string `json:"username" example:"alice"`
	Avatar   string `json:"avatar,omitempty" example:"/api/assets/01a15240-3847-71a4-b8bb-f8448ef3a952/original"`
}
