// https://discord.com/oauth2/authorize?client_id=943281411299880970&permissions=8&scope=bot

package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	"github.com/piquette/finance-go/chart"
	"github.com/piquette/finance-go/datetime"
	"github.com/piquette/finance-go/quote"
	gchart "github.com/wcharczuk/go-chart"
)

func goDotEnvVariable(key string) string {

	// load .env file
	err := godotenv.Load(".env")

	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	return os.Getenv(key)
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	// ignore all messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	if strings.HasPrefix(m.Content, "!ping") {
		s.ChannelMessageSend(m.ChannelID, "Pong!")
	}

	if strings.HasPrefix(m.Content, "!test") {
		args := strings.Split(m.Content, " ")
		if len(args) != 2 {
			s.ChannelMessageSend(m.ChannelID, "Usage: !test <symbol>")
			return
		}
		symbol := args[1]
		q, err := quote.Get(symbol)
		if err != nil {
			// Uh-oh.
			panic(err)
		}

		currentTime := time.Now()
		JavascriptISOString := "2006-01-02T15:04:05.999Z07:00"
		timestamp := fmt.Sprint(currentTime.Format(JavascriptISOString))

		p := &chart.Params{
			Symbol:   symbol,
			Start:    &datetime.Datetime{Month: 1, Day: 1, Year: 2017},
			End:      &datetime.Datetime{Month: 1, Day: 1, Year: 2018},
			Interval: datetime.OneDay}

		iter := chart.Get(p)
		// for iter.Next() {
		// 	b := iter.Bar()
		// 	fmt.Println(b)

		// 	// Meta-data for the iterator - (*finance.ChartMeta).
		// 	fmt.Println(iter.Meta())
		// }

		// Catch an error, if there was one.
		if iter.Err() != nil {
			// Uh-oh!
			panic(err)
		}

		graph := gchart.Chart{
			Series: []gchart.Series{
				gchart.ContinuousSeries{
					XValues: []float64{1.0, 2.0, 3.0, 4.0},
					YValues: []float64{1.0, 2.0, 3.0, 4.0},
				},
			},
		}
		buffer := bytes.NewBuffer([]byte{})
		err = graph.Render(gchart.PNG, buffer)
		f, err := os.Create("test.png")
		img, err := f.Write(buffer.Bytes())
		fmt.Printf("wrote %d bytes\n", img)

		embedImage := &discordgo.MessageEmbedImage{URL: "attachment://graph.png"}

		embed := &discordgo.MessageEmbed{Title: fmt.Sprintf("%s (%s)", q.ShortName, q.Symbol), Description: fmt.Sprintf("Price: %.2f %s\nPercent Change: %.2f%% today\nOpen %.2f High %.2f     Low %.2f", q.RegularMarketPrice, q.CurrencyID, q.RegularMarketChangePercent, q.RegularMarketOpen, q.RegularMarketDayHigh, q.RegularMarketDayLow), Timestamp: timestamp, Image: embedImage}

		s.ChannelMessageSendEmbed(m.ChannelID, embed)
	}
}

func main() {
	token := goDotEnvVariable("BOT_TOKEN")

	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("Error creating Discord session,", err)
		return
	}

	dg.AddHandler(messageCreate)

	dg.Identify.Intents = discordgo.IntentsGuildMessages

	err = dg.Open()
	if err != nil {
		fmt.Println("Error opening connection,", err)
		return
	}

	fmt.Println("Bot is now running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	dg.Close()
}
