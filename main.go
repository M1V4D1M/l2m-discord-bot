package main

import (
	"bytes"
	"discord-bot/discord"
	"discord-bot/notion"
	"discord-bot/openai"
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
	OpenAIClient  *openai.Client
	PublicKey     string
}

func main() {
	godotenv.Load()
	rand.Seed(time.Now().UnixNano())

	bot := &Bot{
		DiscordClient: discord.NewClient(os.Getenv("DISCORD_BOT_TOKEN"), os.Getenv("DISCORD_APP_ID")),
		NotionClient:  notion.NewClient(os.Getenv("NOTION_API_TOKEN"), os.Getenv("NOTION_DATABASE_ID")),
		OpenAIClient:  openai.NewClient(os.Getenv("OPENAI_API_TOKEN")),
		PublicKey:     os.Getenv("DISCORD_PUBLIC_KEY"),
	}

	r := gin.Default()

	r.POST("/interactions", bot.HandleInteractions)
	r.StaticFile("/", "./test.html")
	r.POST("/webhook/bot", func(c *gin.Context) {
		target := c.PostForm("target")
		fileHeader, err := c.FormFile("file0")
		if err != nil {
			log.Printf("Error receiving file: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "No file received"})
			return
		}

		file, err := fileHeader.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open file"})
			return
		}
		defer file.Close()

		fileBytes, err := io.ReadAll(file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
			return
		}

		log.Printf("Received webhook request: target=%s, filename=%s, size=%d bytes", target, fileHeader.Filename, len(fileBytes))

		// 1. Get item name from OpenAI
		itemName, err := bot.OpenAIClient.GetItemNameFromImage(fileBytes)
		if err != nil {
			log.Printf("OpenAI error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "OpenAI processing failed"})
			return
		}
		log.Printf("Item name detected: %s", itemName)

		// 2. Determine channel and role based on target
		channelID := os.Getenv("CHANNEL_OTHER")
		roleID := os.Getenv("ROLE_OTHER")
		if target == "ducks" {
			channelID = os.Getenv("CHANNEL_DUCKS")
			roleID = os.Getenv("ROLE_DUCKS")
		}

		// 3. Send message to Discord with image
		content := fmt.Sprintf("<@&%s> %s", roleID, itemName)
		msg, err := bot.DiscordClient.CreateMessage(channelID, content, "", "item.png", fileBytes)
		if err != nil {
			log.Printf("Discord message error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send Discord message"})
			return
		}

		// 4. Create thread on the message
		thread, err := bot.DiscordClient.CreateThread(channelID, msg.ID, itemName)
		if err != nil {
			log.Printf("Discord thread error: %v", err)
		} else {
			// 5. Send first message to thread
			_, err = bot.DiscordClient.CreateMessage(thread.ID, "В коллекцию", "", "", nil)
			if err != nil {
				log.Printf("Discord thread message error: %v", err)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"item_name": itemName,
		})
	})

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
		_, _ = b.DiscordClient.CreateMessage(interaction.ChannelID, fmt.Sprintf("📊 **Отчет /scrolls**\nБаза данных Notion обновлена. Обработано участников: %d.", count), "", "", nil)
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

		// 2. Get users with scrolls from Notion
		scrollOwners, err := b.NotionClient.GetUsersWithScrolls()
		if err != nil {
			b.DiscordClient.EditInteractionResponse(interaction.Token, "Error querying Notion: "+err.Error())
			return
		}

		// 3. Find intersection and store message IDs for replies
		type winnerInfo struct {
			userID    string
			userName  string
			messageID string
		}
		var eligible []winnerInfo

		// Map to ensure we pick unique users but store one of their message IDs
		processedUsers := make(map[string]bool)

		for _, msg := range messages {
			if len(msg.Attachments) > 0 && scrollOwners[msg.Author.ID] && !processedUsers[msg.Author.ID] {
				displayName := msg.Author.Username
				if msg.Author.GlobalName != "" {
					displayName = msg.Author.GlobalName
				}
				if msg.Member != nil && msg.Member.Nick != nil && *msg.Member.Nick != "" {
					displayName = *msg.Member.Nick
				}

				eligible = append(eligible, winnerInfo{
					userID:    msg.Author.ID,
					userName:  displayName,
					messageID: msg.ID,
				})
				processedUsers[msg.Author.ID] = true
			}
		}

		if len(eligible) == 0 {
			msg := "Не найдено подходящих участников (нужен скриншот в этом треде и наличие свитка в таблице)."
			b.DiscordClient.EditInteractionResponse(interaction.Token, msg)
			_, _ = b.DiscordClient.CreateMessage(interaction.ChannelID, msg, "", "", nil)
			log.Printf("Roll command completed. No eligible participants found.")
			return
		}

		// 4. Randomly select
		winner := eligible[rand.Intn(len(eligible))]

		log.Printf("Roll command: found %d eligible participants. Selected winner: %s", len(eligible), winner.userName)

		resultMsg := fmt.Sprintf("🎲 Розыгрыш завершен!\nПобедитель: <@%s> (%s)\nЭтот человек получит предмет!", winner.userID, winner.userName)
		b.DiscordClient.EditInteractionResponse(interaction.Token, resultMsg)
		_, err = b.DiscordClient.CreateMessage(interaction.ChannelID, resultMsg, winner.messageID, "", nil)
		if err != nil {
			log.Printf("Error sending message to Discord channel %s: %v", interaction.ChannelID, err)
		}

		// Добавляем реакцию :pig: на изначальное сообщение (стартер треда)
		channelInfo, err := b.DiscordClient.GetChannel(interaction.ChannelID)
		if err == nil && channelInfo.ParentID != "" {
			err = b.DiscordClient.AddReaction(channelInfo.ParentID, interaction.ChannelID, "🐷")
			if err != nil {
				log.Printf("Error adding reaction: %v", err)
			}
		}

		log.Printf("Roll command completed. Winner: %s", winner.userName)
	}()
}
