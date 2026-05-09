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
	Member    *Member         `json:"member"`
}

type InteractionData struct {
	Name string `json:"name"`
}

type User struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	GlobalName string `json:"global_name"`
}

type Member struct {
	Nick *string `json:"nick"` // Никнейм на сервере
	User User    `json:"user"`
}

type Message struct {
	ID          string  `json:"id"`
	Content     string  `json:"content"`
	Author      User    `json:"author"`
	Member      *Member `json:"member"`
	Attachments []struct {
		ID string `json:"id"`
	} `json:"attachments"`
}

type InteractionResponse struct {
	Type InteractionResponseDataType `json:"type"`
	Data *InteractionResponseData    `json:"data,omitempty"`
}

type InteractionResponseData struct {
	Content string `json:"content"`
}
