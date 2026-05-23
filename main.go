package main

import (
	"encoding/csv"
	"io"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-querystring/query"
	gozaim "github.com/s-sasaki-0529/go-zaim"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
	"golang.org/x/text/width"
)

type Payment struct {
	CategoryID    int    `url:"category_id"`
	GenreID       int    `url:"genre_id"`
	Amount        int    `url:"amount"`
	Date          string `url:"date"`
	FromAccountID int    `url:"from_account_id"`
	Place         string `url:"place"`
}

type Money struct {
	StartDate     string `url:"start_date,omitempty"`
	EndData       string `url:"end_date,omitempty"`
	Mode          string `url:"mode,omitempty"`
	FromAccountID int    `url:"from_account_id,omitempty"`
	Limit         int    `url:"limit,omitempty"`
	Page          int    `url:"page,omitempty"`
}

var zaim *gozaim.Client

// Zaim API は短時間に大量のリクエストを送ると 401 Unauthorized で
// throttling してくる（公式ドキュメントには記載なし、実測値）。
// 数分の cooldown で復帰するので、401 が出たら指数バックオフでリトライする。
var retryBackoffs = []time.Duration{
	30 * time.Second,
	60 * time.Second,
	120 * time.Second,
	240 * time.Second,
	480 * time.Second,
}

func isThrottled(err error) bool {
	return err != nil && strings.Contains(err.Error(), "401 Unauthorized")
}

func fetchMoneyWithRetry(params url.Values) (gozaim.MoneySlice, error) {
	for i := 0; ; i++ {
		ms, err := zaim.FetchMoney(params)
		if err == nil || !isThrottled(err) || i >= len(retryBackoffs) {
			return ms, err
		}
		log.Printf("FetchMoney throttled (401), retrying in %v (attempt %d/%d)", retryBackoffs[i], i+1, len(retryBackoffs))
		time.Sleep(retryBackoffs[i])
	}
}

func createPaymentWithRetry(params url.Values) (bool, error) {
	for i := 0; ; i++ {
		ok, err := zaim.CreatePayment(params)
		if err == nil || !isThrottled(err) || i >= len(retryBackoffs) {
			return ok, err
		}
		log.Printf("CreatePayment throttled (401), retrying in %v (attempt %d/%d)", retryBackoffs[i], i+1, len(retryBackoffs))
		time.Sleep(retryBackoffs[i])
	}
}

func main() {
	zaim = gozaim.NewClient(
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

	var amex *os.File

	if len(os.Args) > 1 {
		amex, err = os.Open(os.Args[1])
		if err != nil {
			log.Fatal(err)
		}
	} else {
		amex = os.Stdin
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

		// 口座振替分は支払い元の銀行口座での支払いと重複するので除外
		if row[2] == "前回分口座振替金額" {
			continue
		}

		amount, _ := strconv.Atoi(strings.Replace(row[5], ",", "", -1))
		payment := &Payment{
			CategoryID:    categoryID,
			GenreID:       genreID,
			Amount:        amount,
			Date:          strings.Replace(row[0], "/", "-", -1),
			FromAccountID: accountID,
			Place:         convertPlace(row[2]),
		}

		duplicated, err := payment.Duplicated()
		if err != nil {
			log.Fatalf("Duplicated check failed for %s %d %q: %v", payment.Date, payment.Amount, payment.Place, err)
		}
		if duplicated {
			continue
		}

		err = payment.SetCategoryAndGenre()
		if err != nil {
			log.Fatal(err)
		}

		params, _ := query.Values(payment)
		_, err = createPaymentWithRetry(params)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Posted %#v\n", payment)
	}
}

func (p *Payment) Duplicated() (bool, error) {
	money := Money{
		StartDate:     p.Date,
		EndData:       p.Date,
		Mode:          "payment",
		FromAccountID: p.FromAccountID,
		Limit:         100,
	}

	params, _ := query.Values(money)

	payments, err := fetchMoneyWithRetry(params)
	if err != nil {
		return false, err
	}

	for _, payment := range payments {
		if payment.Amount == p.Amount && (strings.Contains(payment.Comment, p.Place) || strings.Contains(payment.Place, p.Place)) {
			return true, nil
		}
	}

	return false, nil
}

func convertPlace(place string) string {
	// CSVファイル上で長音記号が?になってしまっているので変換
	place = strings.Replace(place, "?", "ー", -1)
	place = width.Fold.String(place)
	place = strings.TrimSpace(place)

	return place
}

func (p *Payment) SetCategoryAndGenre() error {
	date, err := time.Parse("2006-01-02", p.Date)
	if err != nil {
		return err
	}

	date = date.AddDate(0, -3, 0)

	for page := 1; ; page++ {
		money := Money{
			StartDate:     date.Format("2006-01-02"),
			Mode:          "payment",
			FromAccountID: p.FromAccountID,
			Limit:         100,
			Page:          page,
		}

		params, err := query.Values(money)
		if err != nil {
			return err
		}

		payments, err := fetchMoneyWithRetry(params)
		if err != nil {
			return err
		}

		for _, payment := range payments {
			if strings.Contains(payment.Comment, p.Place) || strings.Contains(payment.Place, p.Place) {
				p.CategoryID = payment.CategoryID
				p.GenreID = payment.GenreID
				return nil
			}
		}

		if len(payments) < 100 {
			return nil
		}
	}
}
