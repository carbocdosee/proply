package domain

import (
	"encoding/json"
	"time"
)

// Plan represents the user subscription plan.
type Plan string

const (
	PlanFree Plan = "free"
	PlanPro  Plan = "pro"
	PlanTeam Plan = "team"
)

// ProposalStatus represents the lifecycle state of a proposal.
type ProposalStatus string

const (
	StatusDraft    ProposalStatus = "draft"
	StatusSent     ProposalStatus = "sent"
	StatusOpened   ProposalStatus = "opened"
	StatusApproved ProposalStatus = "approved"
	StatusRejected ProposalStatus = "rejected"
)

// BlockType represents the type of a proposal block.
type BlockType string

const (
	BlockTypeText        BlockType = "text"
	BlockTypePriceTable  BlockType = "price_table"
	BlockTypeCaseStudy   BlockType = "case_study"
	BlockTypeTeamMember  BlockType = "team_member"
	BlockTypeTerms       BlockType = "terms"
)

// User represents an authenticated user/agency.
type User struct {
	ID                  string     `json:"id"`
	Email               string     `json:"email"`
	Name                string     `json:"name"`
	PasswordHash        *string    `json:"-"`
	GoogleID            *string    `json:"-"`
	EmailVerifiedAt     *time.Time `json:"email_verified_at,omitempty"`
	Plan                Plan       `json:"plan"`
	Language            string     `json:"language"`
	LogoURL             *string    `json:"logo_url,omitempty"`
	PrimaryColor        string     `json:"primary_color"`
	AccentColor         string     `json:"accent_color"`
	HideProplyFooter    bool       `json:"hide_proply_footer"`
	DataRetentionMonths int        `json:"data_retention_months"`
	StripeCustomerID    *string    `json:"-"`
	CreatedAt           time.Time  `json:"created_at"`
	DeletedAt           *time.Time `json:"-"`
}

// Block represents a single content block in a proposal.
type Block struct {
	ID    string          `json:"id"`
	Type  BlockType       `json:"type"`
	Order int             `json:"order"`
	Data  json.RawMessage `json:"data"`
}

// Proposal represents a commercial proposal document.
type Proposal struct {
	ID            string         `json:"id"`
	UserID        string         `json:"user_id"`
	Title         string         `json:"title"`
	ClientName    string         `json:"client_name"`
	ClientEmail   *string        `json:"client_email,omitempty"`
	Status        ProposalStatus `json:"status"`
	Slug          *string        `json:"slug,omitempty"`
	SlugActive    bool           `json:"slug_active"`
	PasswordHash  *string        `json:"-"`
	Blocks        []Block        `json:"blocks"`
	TemplateID    *string        `json:"template_id,omitempty"`
	FirstOpenedAt *time.Time     `json:"first_opened_at,omitempty"`
	LastOpenedAt  *time.Time     `json:"last_opened_at,omitempty"`
	OpenCount     int            `json:"open_count"`
	ApprovedAt    *time.Time     `json:"approved_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     *time.Time     `json:"-"`
}

// TrackingEvent represents an analytics event for a proposal.
type TrackingEvent struct {
	ID          string     `json:"id"`
	ProposalID  string     `json:"proposal_id"`
	EventType   string     `json:"event_type"` // open | block_time | approve
	BlockID     *string    `json:"block_id,omitempty"`
	DurationMs  *int       `json:"duration_ms,omitempty"`
	Country     *string    `json:"country,omitempty"`
	UserAgent   string     `json:"user_agent"`
	Fingerprint string     `json:"-"`
	CreatedAt   time.Time  `json:"created_at"`
}

// JobQueue represents a background job entry.
type JobQueue struct {
	ID          string          `json:"id"`
	JobType     string          `json:"job_type"`
	Payload     json.RawMessage `json:"payload"`
	Status      string          `json:"status"`
	Attempts    int             `json:"attempts"`
	MaxAttempts int             `json:"max_attempts"`
	ScheduledAt time.Time       `json:"scheduled_at"`
	ProcessedAt *time.Time      `json:"processed_at,omitempty"`
	Error       *string         `json:"error,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

// Subscription represents a billing subscription.
type Subscription struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	Provider         string    `json:"provider"` // stripe | paddle
	ExternalID       string    `json:"external_id"`
	Plan             Plan      `json:"plan"`
	Status           string    `json:"status"` // active | cancelled | past_due
	CurrentPeriodEnd time.Time `json:"current_period_end"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// PublicProposal is the proposal data returned for the public viewer.
type PublicProposal struct {
	Title              string  `json:"title"`
	ClientName         string  `json:"client_name"`
	AgencyName         string  `json:"agency_name"`
	LogoURL            *string `json:"logo_url,omitempty"`
	PrimaryColor       string  `json:"primary_color"`
	AccentColor        string  `json:"accent_color"`
	HideProplyFooter   bool    `json:"hide_proply_footer"`
	Language           string  `json:"language"`
	Blocks             []Block `json:"blocks"`
	Status             ProposalStatus `json:"status"`
	ApprovedAt         *time.Time     `json:"approved_at,omitempty"`
	PasswordProtected  bool    `json:"password_protected"`
}
