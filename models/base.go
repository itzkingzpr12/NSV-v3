package models

// Channel struct
type Channel struct {
	ID string `json:"id"`
}

// Command struct
type Command struct {
	Name string `json:"name"`
}

// Message struct
type Message struct {
	ID      string  `json:"id"`
	Channel Channel `json:"channel"`
}

// Reaction struct
type Reaction struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// Server struct
type Server struct {
	ID        uint64 `json:"id"`
	NitradoID int64  `json:"nitrado_id"`
	Name      string `json:"name"`
}

// Step struct
type Step struct {
	Name string `json:"name"`
}

// User struct
type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Guild struct
type Guild struct {
	ID string `json:"id"`
}
