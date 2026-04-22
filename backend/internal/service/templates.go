package service

import (
	"crypto/rand"
	"encoding/json"
	"fmt"

	"proply/internal/domain"
)

// TemplateMeta describes a proposal template for the picker UI.
type TemplateMeta struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	BlockTypes  []string `json:"block_types"` // ordered list for preview
}

// catalogTemplates is the static list of available templates.
var catalogTemplates = []TemplateMeta{
	{
		ID:          "web",
		Name:        "Web Development",
		Description: "Website or web-app project with scope, pricing, and portfolio.",
		BlockTypes:  []string{"text", "price_table", "case_study", "terms"},
	},
	{
		ID:          "seo",
		Name:        "SEO Optimization",
		Description: "Search engine optimization services with deliverables and timeline.",
		BlockTypes:  []string{"text", "price_table", "terms"},
	},
	{
		ID:          "smm",
		Name:        "Social Media Marketing",
		Description: "Content strategy, community management, and paid social.",
		BlockTypes:  []string{"text", "price_table", "terms"},
	},
	{
		ID:          "design",
		Name:        "Design & Branding",
		Description: "Identity design, UI/UX, or brand refresh with portfolio examples.",
		BlockTypes:  []string{"text", "price_table", "case_study", "terms"},
	},
	{
		ID:          "consulting",
		Name:        "Consulting",
		Description: "Strategy or advisory engagement with team bios and deliverables.",
		BlockTypes:  []string{"text", "price_table", "team_member", "terms"},
	},
}

// ListTemplates returns the full template catalog.
func ListTemplates() []TemplateMeta {
	return catalogTemplates
}

// blocksForTemplate returns pre-filled blocks for the given template ID.
// Returns nil if templateID is unknown (blank proposal).
func blocksForTemplate(templateID string) ([]domain.Block, error) {
	switch templateID {
	case "web":
		return buildBlocks([]rawBlock{
			{Type: "text", Data: map[string]any{
				"html": "<h2>Project overview</h2><p>We are excited to present this proposal for your website project. Our team will deliver a modern, performant web solution tailored to your goals.</p>",
			}},
			{Type: "price_table", Data: map[string]any{
				"rows": []map[string]any{
					{"service": "UI/UX Design", "qty": 1, "price": 0},
					{"service": "Frontend development", "qty": 1, "price": 0},
					{"service": "Backend & API", "qty": 1, "price": 0},
					{"service": "QA & Testing", "qty": 1, "price": 0},
				},
			}},
			{Type: "case_study", Data: map[string]any{
				"title":       "Recent project",
				"description": "Describe a similar project you delivered and the impact it had.",
				"image_url":   nil,
			}},
			{Type: "terms", Data: map[string]any{
				"items": []string{
					"50% deposit due before work begins; remaining 50% on delivery.",
					"Client will provide all copy, logos, and brand assets within 5 business days.",
					"Up to two rounds of revisions are included; additional rounds billed at hourly rate.",
					"Delivery timeline starts after receipt of deposit and all required assets.",
				},
			}},
		})

	case "seo":
		return buildBlocks([]rawBlock{
			{Type: "text", Data: map[string]any{
				"html": "<h2>SEO strategy overview</h2><p>We will improve your search visibility through technical SEO, content optimisation, and authoritative link building to drive sustainable organic growth.</p>",
			}},
			{Type: "price_table", Data: map[string]any{
				"rows": []map[string]any{
					{"service": "Technical SEO audit", "qty": 1, "price": 0},
					{"service": "On-page optimisation (per page)", "qty": 10, "price": 0},
					{"service": "Monthly link building", "qty": 1, "price": 0},
					{"service": "Monthly reporting", "qty": 1, "price": 0},
				},
			}},
			{Type: "terms", Data: map[string]any{
				"items": []string{
					"Initial audit and strategy delivered within 10 business days.",
					"Results are measured monthly; organic ranking improvements typically visible within 90 days.",
					"Client provides full access to Google Search Console and website CMS.",
					"Contract is month-to-month with 30-day cancellation notice.",
				},
			}},
		})

	case "smm":
		return buildBlocks([]rawBlock{
			{Type: "text", Data: map[string]any{
				"html": "<h2>Social media strategy</h2><p>We will manage and grow your social media presence across chosen platforms through consistent content, community engagement, and paid campaigns.</p>",
			}},
			{Type: "price_table", Data: map[string]any{
				"rows": []map[string]any{
					{"service": "Content calendar & copywriting", "qty": 1, "price": 0},
					{"service": "Graphic design (posts per month)", "qty": 12, "price": 0},
					{"service": "Community management", "qty": 1, "price": 0},
					{"service": "Paid social management", "qty": 1, "price": 0},
				},
			}},
			{Type: "terms", Data: map[string]any{
				"items": []string{
					"Content calendar submitted for approval 5 days before the start of each month.",
					"Ad spend budget is separate and managed directly by the client.",
					"Three rounds of creative revisions included per month.",
					"Monthly performance report delivered within 5 business days of month end.",
				},
			}},
		})

	case "design":
		return buildBlocks([]rawBlock{
			{Type: "text", Data: map[string]any{
				"html": "<h2>Design proposal</h2><p>We will create a compelling visual identity and user experience that reflects your brand values and resonates with your target audience.</p>",
			}},
			{Type: "price_table", Data: map[string]any{
				"rows": []map[string]any{
					{"service": "Brand discovery & moodboard", "qty": 1, "price": 0},
					{"service": "Logo & identity system", "qty": 1, "price": 0},
					{"service": "UI design (screens)", "qty": 5, "price": 0},
					{"service": "Brand guidelines document", "qty": 1, "price": 0},
				},
			}},
			{Type: "case_study", Data: map[string]any{
				"title":       "Portfolio highlight",
				"description": "Showcase a past branding or design project that demonstrates your style and expertise.",
				"image_url":   nil,
			}},
			{Type: "terms", Data: map[string]any{
				"items": []string{
					"All original source files (Figma / AI / SVG) delivered upon final payment.",
					"Two logo concepts presented; client selects one for refinement.",
					"Up to three revision rounds per deliverable are included.",
					"Client retains full ownership of final assets after project completion.",
				},
			}},
		})

	case "consulting":
		return buildBlocks([]rawBlock{
			{Type: "text", Data: map[string]any{
				"html": "<h2>Consulting engagement</h2><p>Our experienced team will analyse your current situation, identify opportunities, and provide a clear strategic roadmap to help you achieve your business goals.</p>",
			}},
			{Type: "price_table", Data: map[string]any{
				"rows": []map[string]any{
					{"service": "Discovery workshop (full day)", "qty": 1, "price": 0},
					{"service": "Current-state analysis", "qty": 1, "price": 0},
					{"service": "Strategic roadmap & presentation", "qty": 1, "price": 0},
					{"service": "Implementation support (days)", "qty": 5, "price": 0},
				},
			}},
			{Type: "team_member", Data: map[string]any{
				"name":      "Your Name",
				"role":      "Lead Consultant",
				"bio":       "Describe your background, expertise, and why you are the right person for this engagement.",
				"photo_url": nil,
			}},
			{Type: "terms", Data: map[string]any{
				"items": []string{
					"Engagement begins upon receipt of signed agreement and 50% deposit.",
					"Weekly status calls scheduled at project kickoff.",
					"All findings and recommendations are confidential and remain the client's property.",
					"Additional advisory hours available at agreed day rate beyond the scope above.",
				},
			}},
		})

	default:
		return nil, nil
	}
}

// rawBlock is an internal helper for building blocks from static data.
type rawBlock struct {
	Type string
	Data map[string]any
}

// buildBlocks serialises rawBlock entries into domain.Block values with generated IDs.
func buildBlocks(raws []rawBlock) ([]domain.Block, error) {
	blocks := make([]domain.Block, 0, len(raws))
	for i, rb := range raws {
		id, err := newBlockID()
		if err != nil {
			return nil, fmt.Errorf("templates: generate block id: %w", err)
		}
		dataJSON, err := json.Marshal(rb.Data)
		if err != nil {
			return nil, fmt.Errorf("templates: marshal block data: %w", err)
		}
		blocks = append(blocks, domain.Block{
			ID:    id,
			Type:  domain.BlockType(rb.Type),
			Order: i,
			Data:  dataJSON,
		})
	}
	return blocks, nil
}

// newBlockID generates a random hex ID for a block (16 bytes → 32 hex chars).
func newBlockID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}
