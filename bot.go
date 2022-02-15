// https://discord.com/oauth2/authorize?client_id=943281411299880970&permissions=8&scope=bot

package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

func goDotEnvVariable(key string) string {

	// load .env file
	err := godotenv.Load(".env")

	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	return os.Getenv(key)
}

func main() {
	fmt.Println("Hello world")
	fmt.Println(os.Getenv("BOT_TOKEN"))
}
