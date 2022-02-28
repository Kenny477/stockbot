// https://discord.com/oauth2/authorize?client_id=943281411299880970&permissions=8&scope=bot

package main

import (
	"bytes"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	"github.com/piquette/finance-go/chart"
	"github.com/piquette/finance-go/datetime"
	"github.com/piquette/finance-go/equity"
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

func getApproximation(num int) string {
	log := math.Log10(float64(num))
	logInt := int(log)

	newNum := float64(num) / math.Pow(10, float64(logInt))

	if logInt >= 12 {
		return fmt.Sprintf("%.2fT", newNum)
	}
	if logInt >= 9 {
		return fmt.Sprintf("%.2fB", newNum)
	}
	if logInt >= 6 {
		return fmt.Sprintf("%.2fM", newNum)
	}
	if logInt >= 3 {
		return fmt.Sprintf("%.2fK", newNum)
	}

	return fmt.Sprintf("%d", num)
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
		eq, err := equity.Get(symbol)
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

		prevClose := make([]float64, len(x))
		for i := range prevClose {
			prevClose[i] = q.RegularMarketPreviousClose
		}

		prevCloseSeries := &gchart.TimeSeries{
			Name:    "Previous Close",
			XValues: x,
			YValues: prevClose,
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
				gchart.AnnotationSeries{
					Annotations: []gchart.Value2{
						{XValue: gchart.TimeToFloat64(x[len(x)-1]), YValue: y[len(y)-1], Label: fmt.Sprintf("%s: %.2f", q.Symbol, y[len(y)-1])},
						{XValue: gchart.TimeToFloat64(x[len(x)-1]), YValue: prevClose[len(prevClose)-1], Label: fmt.Sprintf("Previous Close: %.2f", prevClose[len(prevClose)-1])},
					},
					Style: gchart.Style{
						StrokeColor: gchart.ColorBlack,
					},
				},
				&gchart.MinSeries{
					InnerSeries: prevCloseSeries,
					Style: gchart.Style{
						StrokeColor:     gchart.ColorAlternateGray,
						StrokeDashArray: []float64{5.0, 5.0},
					},
				},
			},
		}

		buffer := bytes.NewBuffer([]byte{})
		err = graph.Render(gchart.PNG, buffer)

		reader := bytes.NewReader(buffer.Bytes())
		embedImg := &discordgo.File{Name: fmt.Sprintf("%s.png", q.Symbol), ContentType: "image/png", Reader: reader}
		embedImage := &discordgo.MessageEmbedImage{URL: "attachment://" + fmt.Sprintf("%s.png", q.Symbol)}

		fields := []*discordgo.MessageEmbedField{
			{
				Name:   "Price",
				Value:  fmt.Sprintf("%.2f %s", q.RegularMarketPrice, q.CurrencyID),
				Inline: true,
			},
			{
				Name:   "Volume",
				Value:  getApproximation(q.RegularMarketVolume),
				Inline: true,
			},
			{
				Name:   "Percent Change Today",
				Value:  fmt.Sprintf("%.2f%%", q.RegularMarketChangePercent),
				Inline: false,
			},
			{
				Name:   "Previous Close",
				Value:  fmt.Sprintf("%.2f %s", q.RegularMarketPreviousClose, q.CurrencyID),
				Inline: false,
			},
			{
				Name:   "Open",
				Value:  fmt.Sprintf("%.2f", q.RegularMarketOpen),
				Inline: true,
			},
			{
				Name:   "High",
				Value:  fmt.Sprintf("%.2f", q.RegularMarketDayHigh),
				Inline: true,
			},
			{
				Name:   "Low",
				Value:  fmt.Sprintf("%.2f", q.RegularMarketDayLow),
				Inline: true,
			},
			{
				Name:   "Market Cap",
				Value:  getApproximation(int(eq.MarketCap)),
				Inline: true,
			},
			{
				Name:   "P/E Ratio",
				Value:  fmt.Sprintf("%.2f", eq.TrailingPE),
				Inline: true,
			},
			{
				Name:   "Dividend Yield",
				Value:  fmt.Sprintf("%.2f%%", eq.TrailingAnnualDividendYield*100),
				Inline: true,
			},
			{
				Name:   "EPS",
				Value:  fmt.Sprintf("%.2f", eq.EpsTrailingTwelveMonths),
				Inline: true,
			},
			{
				Name:   "52 Week High",
				Value:  fmt.Sprintf("%.2f", q.FiftyTwoWeekHigh),
				Inline: true,
			},
			{
				Name:   "52 Week Low",
				Value:  fmt.Sprintf("%.2f", q.FiftyTwoWeekLow),
				Inline: true,
			},
		}

		embed := &discordgo.MessageEmbed{Title: fmt.Sprintf("%s (%s)", q.ShortName, q.Symbol), Timestamp: timestamp, Image: embedImage, Fields: fields}

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
