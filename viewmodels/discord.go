package viewmodels

// GetAllGuildsResponse struct
type GetAllGuildsResponse struct {
	Message string        `json:"message"`
	Count   int           `json:"count"`
	Guilds  []*SmallGuild `json:"guilds"`
}

type SmallGuild struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	OwnerID     string `json:"owner_id"`
	MemberCount int    `json:"member_count"`
}

// VerifySubscriberGuildsResponse struct
type VerifySubscriberGuildsResponse struct {
	Message         string                `json:"message"`
	Count           int                   `json:"count"`
	VerifiedCount   int                   `json:"verified_count"`
	UnverifiedCount int                   `json:"unverified_count"`
	Guilds          []*VerifiedSmallGuild `json:"guilds"`
}

type VerifiedSmallGuild struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	OwnerID       string `json:"owner_id"`
	MemberCount   int    `json:"member_count"`
	NSMSubscriber bool   `json:"nsm_subscriber"`
}
