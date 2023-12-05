package main

import (
	"encoding/csv"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/google/go-querystring/query"
	gozaim "github.com/s-sasaki-0529/go-zaim"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

type Payment struct {
	CategoryID int    `url:"category_id"`
	GenreID    int    `url:"genre_id"`
	Amount     int    `url:"amount"`
	Date       string `url:"date"`
	AccountID  int    `url:"from_account_id"`
	Comment    string `url:"comment"`
}

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

		amount, _ := strconv.Atoi(strings.Replace(row[5], ",", "", -1))
		payment := Payment{
			CategoryID: categoryID,
			GenreID:    genreID,
			Amount:     amount,
			Date:       strings.Replace(row[0], "/", "-", -1),
			AccountID:  accountID,
			Comment:    strings.Replace(row[2], "?", "ー", -1),
		}

		params, _ := query.Values(payment)

		_, err = zaim.CreatePayment(params)
		if err != nil {
			log.Fatal(err)
		}
	}
}
