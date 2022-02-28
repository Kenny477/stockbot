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
	gchart "github.com/wcharczuk/go-chart/v2"
)

func goDotEnvVariable(key string) string {

	// load .env file
	err := godotenv.Load(".env")

	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	return os.Getenv(key)
}

func minmax(arr []float64) (float64, float64) {
	smallest, biggest := arr[0], arr[0]
	for _, v := range arr {
		if v > biggest {
			biggest = v
		}
		if v < smallest {
			smallest = v
		}
	}
	return smallest, biggest
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	// ignore all messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	if strings.HasPrefix(m.Content, "!ping") {
		s.ChannelMessageSend(m.ChannelID, "Pong!")
	}

	if strings.HasPrefix(m.Content, "!get") {
		args := strings.Split(m.Content, " ")
		if len(args) != 2 {
			s.ChannelMessageSend(m.ChannelID, "Usage: !get <symbol>")
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

		currentDay := currentTime.Day()
		currentMonth := int(currentTime.Month())
		currentYear := currentTime.Year()

		pastDay := currentDay
		pastMonth := currentMonth - 1
		pastYear := currentYear
		if pastMonth == 0 {
			pastMonth = 12
			pastYear--
		}

		p := &chart.Params{
			Symbol:   q.Symbol,
			Start:    &datetime.Datetime{Month: pastMonth, Day: pastDay, Year: pastYear},
			End:      &datetime.Datetime{Month: currentMonth, Day: currentDay, Year: currentYear},
			Interval: datetime.OneDay}

		x, y := make([]time.Time, 0), make([]float64, 0)
		iter := chart.Get(p)
		for iter.Next() {
			b := iter.Bar()

			timestamp := b.Timestamp
			close := b.Close
			x = append(x, time.Unix(int64(timestamp), 0))
			y = append(y, close.InexactFloat64())

			// fmt.Println(timestamp, close)
			// Meta-data for the iterator - (*finance.ChartMeta).
			// fmt.Println(iter.Meta())
		}
		min, max := minmax(y)
		r := max - min
		upperIQ := max + (r * 0.25)
		lowerIQ := min - (r * 0.25)
		// fmt.Println(x, y)

		// Catch an error, if there was one.
		if iter.Err() != nil {
			// Uh-oh!
			panic(err)
		}

		graph := gchart.Chart{
			XAxis: gchart.XAxis{
				TickPosition: gchart.TickPositionBetweenTicks,
			},
			YAxis: gchart.YAxis{
				Range: &gchart.ContinuousRange{
					Max: upperIQ,
					Min: lowerIQ,
				},
			},
			Series: []gchart.Series{
				gchart.TimeSeries{
					Name:    q.Symbol,
					XValues: x,
					YValues: y,
				},
			},
		}

		buffer := bytes.NewBuffer([]byte{})
		err = graph.Render(gchart.PNG, buffer)

		reader := bytes.NewReader(buffer.Bytes())
		embedImg := &discordgo.File{Name: fmt.Sprintf("%s.png", q.Symbol), ContentType: "image/png", Reader: reader}
		embedImage := &discordgo.MessageEmbedImage{URL: "attachment://" + fmt.Sprintf("%s.png", q.Symbol)}

		embed := &discordgo.MessageEmbed{Title: fmt.Sprintf("%s (%s)", q.ShortName, q.Symbol), Description: fmt.Sprintf("Price: %.2f %s\nPercent Change: %.2f%% today\nOpen %.2f High %.2f     Low %.2f", q.RegularMarketPrice, q.CurrencyID, q.RegularMarketChangePercent, q.RegularMarketOpen, q.RegularMarketDayHigh, q.RegularMarketDayLow), Timestamp: timestamp, Image: embedImage}

		msg := &discordgo.MessageSend{Embed: embed, Files: []*discordgo.File{embedImg}}

		s.ChannelMessageSendComplex(m.ChannelID, msg)
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
