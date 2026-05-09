package main

import (
	"bytes"
	"discord-bot/discord"
	"discord-bot/notion"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

type Bot struct {
	DiscordClient *discord.Client
	NotionClient  *notion.Client
	PublicKey     string
}

func main() {
	godotenv.Load()

	bot := &Bot{
		DiscordClient: discord.NewClient(os.Getenv("DISCORD_BOT_TOKEN"), os.Getenv("DISCORD_APP_ID")),
		NotionClient:  notion.NewClient(os.Getenv("NOTION_API_TOKEN"), os.Getenv("NOTION_DATABASE_ID")),
		PublicKey:     os.Getenv("DISCORD_PUBLIC_KEY"),
	}

	r := gin.Default()

	r.POST("/interactions", bot.HandleInteractions)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on port %s", port)
	r.Run(":" + port)
}

func (b *Bot) HandleInteractions(c *gin.Context) {
	signature := c.GetHeader("X-Signature-Ed25519")
	timestamp := c.GetHeader("X-Signature-Timestamp")

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	// Restore body for further use
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	if os.Getenv("SKIP_SIGNATURE_VERIFICATION") != "true" {
		if !discord.VerifySignature(b.PublicKey, signature, timestamp, string(body)) {
			c.Status(http.StatusUnauthorized)
			return
		}
	}

	var interaction discord.Interaction
	if err := c.ShouldBindJSON(&interaction); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	if interaction.Type == discord.InteractionPing {
		c.JSON(http.StatusOK, discord.InteractionResponse{
			Type: discord.InteractionCallbackPong,
		})
		return
	}

	if interaction.Type == discord.InteractionApplicationCommand {
		switch interaction.Data.Name {
		case "scrolls":
			b.HandleScrolls(c, interaction)
		case "roll":
			b.HandleRoll(c, interaction)
		default:
			c.JSON(http.StatusOK, discord.InteractionResponse{
				Type: discord.InteractionCallbackChannelMessageWithSource,
				Data: &discord.InteractionResponseData{
					Content: "Unknown command",
				},
			})
		}
	}
}

func (b *Bot) HandleScrolls(c *gin.Context, interaction discord.Interaction) {
	// Defer response
	c.JSON(http.StatusOK, discord.InteractionResponse{
		Type: discord.InteractionCallbackDeferredChannelMessageWithSource,
	})

	go func() {
		messages, err := b.DiscordClient.FetchThreadMessages(interaction.ChannelID)
		if err != nil {
			b.DiscordClient.EditInteractionResponse(interaction.Token, "Error fetching messages: "+err.Error())
			return
		}

		// Filter unique users with attachments
		uniqueUsers := make(map[string]string) // discordID -> display name
		for _, msg := range messages {
			if len(msg.Attachments) > 0 {
				displayName := msg.Author.Username
				if msg.Author.GlobalName != "" {
					displayName = msg.Author.GlobalName
				}
				if msg.Member != nil && msg.Member.Nick != nil && *msg.Member.Nick != "" {
					displayName = *msg.Member.Nick
				}
				uniqueUsers[msg.Author.ID] = displayName
			}
		}

		// Clear Notion DB
		err = b.NotionClient.ClearDatabase()
		if err != nil {
			b.DiscordClient.EditInteractionResponse(interaction.Token, "Error clearing Notion database: "+err.Error())
			return
		}

		// Add to Notion
		count := 0
		for id, name := range uniqueUsers {
			err = b.NotionClient.AddEntry(name, id)
			if err != nil {
				log.Printf("Error adding entry to Notion for %s: %v", name, err)
				continue
			}
			count++
		}

		b.DiscordClient.EditInteractionResponse(interaction.Token, fmt.Sprintf("✅ Обработано %d пользователей. Таблица Notion обновлена.", count))
		b.DiscordClient.CreateMessage(interaction.ChannelID, fmt.Sprintf("📊 **Отчет /scrolls**\nБаза данных Notion обновлена. Обработано участников: %d.", count))
		log.Printf("Scrolls command completed: %d users added", count)
	}()
}

func (b *Bot) HandleRoll(c *gin.Context, interaction discord.Interaction) {
	// Defer response
	c.JSON(http.StatusOK, discord.InteractionResponse{
		Type: discord.InteractionCallbackDeferredChannelMessageWithSource,
	})

	go func() {
		// 1. Fetch messages from current thread to see who posted screenshots
		messages, err := b.DiscordClient.FetchThreadMessages(interaction.ChannelID)
		if err != nil {
			b.DiscordClient.EditInteractionResponse(interaction.Token, "Error fetching messages: "+err.Error())
			return
		}

		threadParticipants := make(map[string]string) // discordID -> display name
		for _, msg := range messages {
			if len(msg.Attachments) > 0 {
				displayName := msg.Author.Username
				if msg.Author.GlobalName != "" {
					displayName = msg.Author.GlobalName
				}
				if msg.Member != nil && msg.Member.Nick != nil && *msg.Member.Nick != "" {
					displayName = *msg.Member.Nick
				}
				threadParticipants[msg.Author.ID] = displayName
			}
		}

		// 2. Get users with scrolls from Notion
		scrollOwners, err := b.NotionClient.GetUsersWithScrolls()
		if err != nil {
			b.DiscordClient.EditInteractionResponse(interaction.Token, "Error querying Notion: "+err.Error())
			return
		}

		// 3. Find intersection
		var eligible []string
		for id := range threadParticipants {
			if scrollOwners[id] {
				eligible = append(eligible, id)
			}
		}

		if len(eligible) == 0 {
			msg := "Не найдено подходящих участников (нужен скриншот в этом треде и наличие свитка в таблице)."
			b.DiscordClient.EditInteractionResponse(interaction.Token, msg)
			b.DiscordClient.CreateMessage(interaction.ChannelID, msg)
			log.Printf("Roll command completed. No eligible participants found.")
			return
		}

		// 4. Randomly select
		rand.Seed(time.Now().UnixNano())
		winnerID := eligible[rand.Intn(len(eligible))]
		winnerName := threadParticipants[winnerID]

		resultMsg := fmt.Sprintf("🎲 Розыгрыш завершен!\nПобедитель: <@%s> (%s)\nЭтот человек получит предмет!", winnerID, winnerName)
		b.DiscordClient.EditInteractionResponse(interaction.Token, resultMsg)
		err = b.DiscordClient.CreateMessage(interaction.ChannelID, resultMsg)
		if err != nil {
			log.Printf("Error sending message to Discord channel %s: %v", interaction.ChannelID, err)
		}
		log.Printf("Roll command completed. Winner: %s", winnerName)
	}()
}
