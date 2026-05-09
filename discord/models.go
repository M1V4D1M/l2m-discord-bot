package discord

type InteractionType int

const (
	InteractionPing               InteractionType = 1
	InteractionApplicationCommand InteractionType = 2
)

type InteractionResponseDataType int

const (
	InteractionCallbackPong                        InteractionResponseDataType = 1
	InteractionCallbackChannelMessageWithSource    InteractionResponseDataType = 4
	InteractionCallbackDeferredChannelMessageWithSource InteractionResponseDataType = 5
)

type Interaction struct {
	Type      InteractionType `json:"type"`
	Token     string          `json:"token"`
	ID        string          `json:"id"`
	ChannelID string          `json:"channel_id"`
	GuildID   string          `json:"guild_id"`
	Data      InteractionData `json:"data"`
	Member    Member          `json:"member"`
}

type InteractionData struct {
	Name string `json:"name"`
}

type Member struct {
	User User `json:"user"`
}

type InteractionResponse struct {
	Type InteractionResponseDataType `json:"type"`
	Data *InteractionResponseData    `json:"data,omitempty"`
}

type InteractionResponseData struct {
	Content string `json:"content"`
}
