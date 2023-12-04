package main

import (
	"encoding/csv"
	"io"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"

	gozaim "github.com/s-sasaki-0529/go-zaim"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

func main() {
	zaim := gozaim.NewClient(
		os.Getenv("ZAIM_CONSUMER_ID"),
		os.Getenv("ZAIM_CONSUMER_SECRET"),
		os.Getenv("ZAIM_ACCESS_TOKEN"),
		os.Getenv("ZAIM_ACCESS_TOKEN_SECRET"),
	)

	accounts, err := zaim.FetchAccounts()
	if err != nil {
		log.Fatal(err)
	}

	var accountID int
	for _, account := range accounts {
		if account.Name == os.Getenv("ZAIM_ACCOUNT_NAME") {
			accountID = account.ID
		}
	}

	if accountID == 0 {
		log.Fatalf("Could not get account: %s", os.Getenv("ZAIM_ACCOUNT_NAME"))
	}

	categories, err := zaim.FetchCategories()
	if err != nil {
		log.Fatal(err)
	}

	var categoryID int
	for _, category := range categories {
		if category.Name == os.Getenv("ZAIM_CATEGORY") {
			categoryID = category.ID
		}
	}

	if categoryID == 0 {
		log.Fatalf("Could not get category: %s", os.Getenv("ZAIM_CATEGORY"))
	}

	genres, err := zaim.FetchGenres()
	if err != nil {
		log.Fatal(err)
	}

	var genreID int
	for _, genre := range genres {
		if genre.Name == os.Getenv("ZAIM_GENRE") {
			genreID = genre.ID
		}
	}

	if genreID == 0 {
		log.Fatalf("Could not get genre: %s", os.Getenv("ZAIM_GENRE"))
	}

	amex, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	reader := csv.NewReader(transform.NewReader(amex, japanese.ShiftJIS.NewDecoder()))
	reader.Read() // Skip header

	for {
		row, err := reader.Read()

		if err == io.EOF {
			break
		}

		if err != nil {
			log.Fatal(err)
		}

		params := url.Values{}
		params.Set("category_id", strconv.Itoa(categoryID))
		params.Set("genre_id", strconv.Itoa(genreID))
		params.Set("amount", strings.Replace(row[5], ",", "", -1))
		params.Set("date", strings.Replace(row[0], "/", "-", -1))
		params.Set("from_account_id", strconv.Itoa(accountID))
		params.Set("comment", row[2])

		_, err = zaim.CreatePayment(params)
		if err != nil {
			log.Fatal(err)
		}
	}
}
