package sql

import (
	"testing"
	"time"

	"gorm.io/gorm"
)

type MtBook struct {
	EventDate time.Time
	Source    string
	Symbol    string
	Datafeed  string
	EventTime int64
	Asks      string
	Bids      string
	Spread    float64
	Digits    int8
}

func (m *MtBook) TableName() string {
	return "mt_book"
}

func TestDB(t *testing.T) {
	db, err := Open(&Config{
		Dialect: "clickhouse",
		URL:     "tcp://clickhouse.gonit.codes:9001?username=dealer&password=abcd1234&database=dealer",
	})

	if err != nil {
		t.Fatal(err)
	}
	query := db.Model(&MtBook{}).
		Scopes(
			func(d *gorm.DB) *gorm.DB {
				d.Where("source = ? ", "dcaisa").
					Where("event_date between ? and ?",
						time.Now().Add(-10*time.Second-8*time.Hour).Format(TimeFormat),
						time.Now().Add(-8*time.Hour).Format(TimeFormat)).
					Where("symbol = ?", "AUDCHFp")
				return d
			})
	rows, err := query.Order("event_date").Rows()
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	result := &MtBook{}
	for rows.Next() {
		err = db.ScanRows(rows, result)
		if err != nil {
			t.Fatal(err)
		}
		t.Log(result)
	}
}

// time format
const (
	TimeFormat = "2006-01-02 15:04:05"
)
