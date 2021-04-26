package discordapi

import (
	"time"

	"github.com/bwmarrin/discordgo"
)

// EmbeddableParams struct
type EmbeddableParams struct {
	Title        string
	Description  string
	Color        int
	TitleURL     string
	Footer       string
	ThumbnailURL string
}

// EmbeddableField interface
type EmbeddableField interface {
	ConvertToEmbedField() (*discordgo.MessageEmbedField, *Error)
}

// MaxEmbedFields const
const MaxEmbedFields = 23 // actually 25

// MaxEmbedCharCount const
const MaxEmbedCharCount = 5600 // actually 6000

// MaxEmbedFieldCharCount const
const MaxEmbedFieldCharCount = 950 // actually 1000

// CreateEmbeds func
func CreateEmbeds(embedParams EmbeddableParams, embedableFields []EmbeddableField) []discordgo.MessageEmbed {
	var embeds []discordgo.MessageEmbed

	embed := &discordgo.MessageEmbed{
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Executed",
		},
		Color:       embedParams.Color,
		Description: embedParams.Description,
		Fields:      []*discordgo.MessageEmbedField{},
		Timestamp:   time.Now().Format(time.RFC3339), // Discord wants ISO8601; RFC3339 is an extension of ISO8601 and should be completely compatible.
		Title:       embedParams.Title,
		URL:         embedParams.TitleURL,
	}

	if embedParams.Footer != "" {
		embed.Footer.Text = embedParams.Footer
	}

	if embedParams.ThumbnailURL != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
			URL: embedParams.ThumbnailURL,
		}
	}

	embedCharCount := 0
	for i := 0; i < len(embedableFields); i++ {
		field, err := embedableFields[i].ConvertToEmbedField()

		if err != nil {
			continue
		}

		if len(field.Name)+len(field.Value)+embedCharCount >= MaxEmbedCharCount || len(embed.Fields) >= MaxEmbedFields {
			embeds = append(embeds, *embed)
			embed.Fields = []*discordgo.MessageEmbedField{}
			embedCharCount = 0
		}

		embedCharCount += len(field.Name) + len(field.Value)
		embed.Fields = append(embed.Fields, field)
	}

	if len(embed.Fields) != 0 || len(embeds) == 0 {
		embeds = append(embeds, *embed)
	}

	return embeds
}
