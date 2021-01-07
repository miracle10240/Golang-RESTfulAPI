package helpers

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"golang.org/x/crypto/bcrypt"
	"math"
	"math/rand"
	"reflect"
	"regexp"
	"strings"
)


func Hash(password string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
}

func RandomString(n int) string {
	var letter = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	b := make([]rune, n)
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))]
	}
	return string(b)
}

func NextPageCal(page int, totalPage int) int {
	if page == totalPage {
		return page
	}
	return page + 1
}

func PrevPageCal(page int) int {
	if page > 1 {
		return page - 1
	}
	return page
}

func TotalPage(count int64, limit int) int {
	return int(math.Ceil(float64(count) / float64(limit)))
}

func OffsetCal(page int, limit int) int {
	return (page - 1) * limit
}

func GetMD5Hash(text string) string {
	hasher := md5.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}

func InArray(val interface{}, array interface{}) (exists bool) {
	exists = false
	switch reflect.TypeOf(array).Kind() {
	case reflect.Slice:
		s := reflect.ValueOf(array)

		for i := 0; i < s.Len(); i++ {
			if reflect.DeepEqual(val, s.Index(i).Interface()) == true {
				exists = true
				return
			}
		}
	}
	return
}

func InArrayDuplicate(val interface{}, array interface{}) (number int) {
	number = 0
	switch reflect.TypeOf(array).Kind() {
	case reflect.Slice:
		s := reflect.ValueOf(array)
		for i := 0; i < s.Len(); i++ {
			if reflect.DeepEqual(val, s.Index(i).Interface()) == true {
				number++
			}
		}

	}
	return
}

func TagControl(tag, key string, s interface{}, defaultV string) (d string) {

	d = defaultV
	if tag == "" {
		return
	}

	rt := reflect.TypeOf(s)
	if rt.Kind() != reflect.Struct {
		panic("bad type")
	}
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		v := strings.Split(f.Tag.Get(key), ",")[0] // use split to ignore tag "options"
		if v == tag {
			return f.Tag.Get(key)
		}
	}

	return
}

func ScoreCalculate(likeCount float32, dislikeCount float32) float32 {
	return 5 + (likeCount-dislikeCount)*5/(likeCount+dislikeCount+1)
}

func ScoreCalculateWithShare(likeCount float32, dislikeCount float32, share float32) float32 {
	likeCount += 5 * share
	return 5 + (likeCount-dislikeCount)*5/(likeCount+dislikeCount+1)
}

func PercentCalculate(share int, denominator int) int {
	if denominator == 0 {
		denominator = 1
	}
	return int((100 * share) / denominator)
}

func RemoveDuplicateValues(intSlice []uint) []uint {
	keys := make(map[uint]bool)
	list := []uint{}
	for _, entry := range intSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func ComputeHmacSha1(message string, secret string) string {
	key := []byte(secret)
	h := hmac.New(sha1.New, key)
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

func ClearNonAlphanumericalCharacters(val string) (string, error) {
	reg, err := regexp.Compile("[^a-zA-Z0-9]+")
	if err != nil {
		return "",err
	}
	return reg.ReplaceAllString(val, "") , nil
}


type MonthlyEnum string

const (
	January   MonthlyEnum = "January"
	February  MonthlyEnum = "February"
	March     MonthlyEnum = "March"
	April     MonthlyEnum = "April"
	May       MonthlyEnum = "May"
	June      MonthlyEnum = "June"
	July      MonthlyEnum = "July"
	August    MonthlyEnum = "August"
	September MonthlyEnum = "September"
	October   MonthlyEnum = "October"
	November  MonthlyEnum = "November"
	December  MonthlyEnum = "December"
)

type MonthlyInformation struct {
	Month    MonthlyEnum
	FullName string
	MouthID  int
}

func (m MonthlyEnum) GetMonthFullName() string {
	return Months[m].FullName
}

func (m MonthlyEnum) GetMonthId() int {
	return Months[m].MouthID
}

func GetMonthNameWithId(id int) string {
	for _, m  := range Months {
		if m.MouthID == id {
			return m.FullName
		}
	}
	return ""
}

var Months = map[MonthlyEnum]MonthlyInformation{
	January:   {Month: January, FullName: "January", MouthID: 1},
	February:  {Month: February, FullName: "February", MouthID: 2},
	March:     {Month: March, FullName: "March", MouthID: 3},
	April:     {Month: April, FullName: "April", MouthID: 4},
	May:       {Month: May, FullName: "May", MouthID: 5},
	June:      {Month: June, FullName: "June", MouthID: 6},
	July:      {Month: July, FullName: "July", MouthID: 7},
	August:    {Month: August, FullName: "August", MouthID: 8},
	September: {Month: September, FullName: "September", MouthID: 9},
	October:   {Month: October, FullName: "October", MouthID: 10},
	November:  {Month: November, FullName: "November", MouthID: 11},
	December:  {Month: December, FullName: "December", MouthID: 12},
}
