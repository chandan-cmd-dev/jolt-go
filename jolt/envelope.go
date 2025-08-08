package jolt

type Meta struct {
    Type     string      `json:"type,omitempty"`
    Schema   string      `json:"schema,omitempty"`
    Version  string      `json:"version,omitempty"`
    Created  *Timestamp  `json:"createdAt,omitempty"`
    Features []string    `json:"features,omitempty"`
    Sig      any         `json:"sig,omitempty"`
}

type Envelope struct {
    Meta Meta `json:"$meta"`
    Body any  `json:"$body"`
}
