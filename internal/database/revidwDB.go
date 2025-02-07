package database

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

// imageToBase64 принимает путь к изображению и возвращает строку Base64 или ошибку
func imageToBase64(imagePath string) (string, error) {
	// Открываем файл
	file, err := os.Open(imagePath)
	if err != nil {
		return "", fmt.Errorf("не удалось открыть файл: %v", err)
	}
	defer file.Close()

	// Читаем содержимое файла
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("не удалось прочитать файл: %v", err)
	}

	// Кодируем содержимое файла в Base64
	base64String := base64.StdEncoding.EncodeToString(fileBytes)
	return base64String, nil
}

func (repo *MyRepository) GroupReviewNewOnesFirstSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, ads_id int) (err error) {
	type Review_type struct {
		Review_id          int    `json:"Review_id"`
		User_id            int    `json:"User_id"`
		Review_name        string `json:"Review_name"`
		Review_avatar      string `json:"Review_avatar"`
		Review_avatar_path string `json:"Review_avatar_path"`
		Updated_at_comment int    `json:"Updated_at_comment"`
		Rating             int    `json:"Rating"`
		Comment            string `json:"Comment"`
		Typee              int    `json:"Type"`

		Rep_id      int    `json:"Rep_id"`
		Rep_comment string `json:"Rep_comment"`
		Rep_date    int    `json:"Rep_date"`

		Rep_owner_id   int    `json:"Rep_owner_id"`
		Rep_owner_name string `json:"Rep_owner_name"`
		Rep_owner_ava  string `json:"Rep_owner_ava"`
	}

	type Reviews struct {
		Review_list  []Review_type `json:"Review_list"`
		Rating_num   float32       `json:"Rating_num"`
		Star_five    int           `json:"Star_five"`
		Star_four    int           `json:"Star_four"`
		Star_thre    int           `json:"Star_thre"`
		Star_two     int           `json:"Star_two"`
		Star_one     int           `json:"Star_one"`
		Review_count int           `json:"Review_count"`
	}
	mass := Reviews{}
	prod := []Review_type{}

	var Updated_at_time time.Time
	var Rep_date_time time.Time
	var Owner_ava_pwd string

	request, err := rep.Query(ctx,
		`
		WITH reply AS (
			SELECT DISTINCT
				reply_to_user.id AS rep_id,	
				reply_to_user.comment AS rep_comment,
				COALESCE(reply_to_user.updated_at, reply_to_user.created_at) AS rep_date,
				reply_to_user.review_id,

				ads.id AS ads_id,
				ads.owner_id,
				COALESCE(individual_user.name, company_user.name_of_company) AS owner_name,
				users.avatar_path
			FROM ads.reply_to_user
			JOIN ads.reviews ON reviews.id = reply_to_user.review_id AND reviews.state != 3
			JOIN orders.bookings ON bookings.order_id = reviews.order_id
			JOIN ads.ads ON bookings.ads_id = $1
			LEFT JOIN users.individual_user ON individual_user.user_id = ads.owner_id
			LEFT JOIN users.company_user ON company_user.user_id = ads.owner_id
			LEFT JOIN users.users ON users.id = ads.owner_id
			WHERE reply_to_user.state != 3 AND ads.id = $1
		)
		SELECT DISTINCT
			reviews.id AS reviews_id,
			users.id AS users_id,
			COALESCE(individual_user.name, company_user.name_of_company) AS name,
			users.avatar_path,
			COALESCE(reviews.updated_at, reviews.created_at) AS updated_at,
			reviews.rating,
			reviews.type,
			COALESCE(reviews.comment, 'Пустой комментарий') AS comment,

			COALESCE((SELECT rep_id FROM reply WHERE review_id = reviews.id), 0),
			COALESCE((SELECT rep_comment FROM reply WHERE review_id = reviews.id), 'No comment'),
			COALESCE((SELECT rep_date FROM reply WHERE review_id = reviews.id), NOW()),
			COALESCE((SELECT owner_id FROM reply WHERE review_id = reviews.id), 0),
			COALESCE((SELECT owner_name FROM reply WHERE review_id = reviews.id), 'No name'),
			COALESCE((SELECT avatar_path FROM reply WHERE review_id = reviews.id), '/home/')
		FROM
			ads.reviews
		JOIN
			orders.orders ON reviews.order_id = orders.id
		JOIN
			orders.bookings ON bookings.order_id = orders.id
		JOIN
			finance.transactions ON bookings.transaction_id = transactions.id
		JOIN
			finance.wallets ON transactions.wallet_id = wallets.id
		JOIN
			users.users ON wallets.user_id = users.id
		LEFT JOIN
			users.individual_user ON users.id = individual_user.user_id
		LEFT JOIN
			users.company_user ON users.id = company_user.user_id
		WHERE
			bookings.ads_id = $1
		ORDER BY reviews_id ASC
		`, ads_id)
	errorr(err)

	for request.Next() {
		rev := Review_type{}
		err := request.Scan(
			&rev.Review_id,
			&rev.User_id,
			&rev.Review_name,
			&rev.Review_avatar_path,
			&Updated_at_time,
			&rev.Rating,
			&rev.Typee,
			&rev.Comment,
			&rev.Rep_id,
			&rev.Rep_comment,
			&Rep_date_time,

			&rev.Rep_owner_id,
			&rev.Rep_owner_name,
			&Owner_ava_pwd,
		)
		errorr(err)

		rev.Updated_at_comment = int(Updated_at_time.Unix())
		rev.Rep_date = int(Rep_date_time.Unix())

		if len(rev.Review_avatar_path) > 0 {
			rev.Review_avatar = ServeSpecificMediaBase64(rw, r, rev.Review_avatar_path)
			rev.Review_avatar_path = "/home/"

			rev.Rep_owner_ava = ServeSpecificMediaBase64(rw, r, Owner_ava_pwd)
		}
		prod = append(prod, rev)
	}

	mass.Review_list = prod

	request, err = rep.Query(ctx,
		`
		WITH stars AS (
			SELECT DISTINCT
				reviews.id,
				reviews.rating
			FROM ads.reviews
			JOIN orders.bookings ON bookings.order_id = reviews.order_id
			JOIN ads.ads ON bookings.ads_id = $1
			WHERE reviews.state != $1
		)
		SELECT DISTINCT
			users.rating,
			(SELECT COUNT(*) FROM stars WHERE rating = 5) AS five,
			(SELECT COUNT(*) FROM stars WHERE rating = 4) AS four,
			(SELECT COUNT(*) FROM stars WHERE rating = 3) AS thre,
			(SELECT COUNT(*) FROM stars WHERE rating = 2) AS two,
			(SELECT COUNT(*) FROM stars WHERE rating = 1) AS one
			FROM users.users, ads.ads
			WHERE ads.id = $1 AND ads.owner_id = users.id
		`, ads_id)
	errorr(err)

	for request.Next() {
		err := request.Scan(
			&mass.Rating_num,
			&mass.Star_five,
			&mass.Star_four,
			&mass.Star_thre,
			&mass.Star_two,
			&mass.Star_one,
		)
		errorr(err)

	}
	mass.Review_count = mass.Star_one + mass.Star_two + mass.Star_thre + mass.Star_four + mass.Star_five

	type Response struct {
		Status  string  `json:"status"`
		Data    Reviews `json:"data,omitempty"`
		Message string  `json:"message"`
	}

	if len(prod) == 0 || err != nil || request == nil {
		response := Response{
			Status:  "fatal",
			Message: "Объявление не найдено",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return err
	}

	fmt.Println(mass)

	response := Response{
		Status:  "success",
		Data:    mass,
		Message: "Объявление показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) GroupReviewOldOnesFirstSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, ads_id int) (err error) {
	type Review_type struct {
		Review_id          int    `json:"Review_id"`
		User_id            int    `json:"User_id"`
		Review_name        string `json:"Review_name"`
		Review_avatar      string `json:"Review_avatar"`
		Review_avatar_path string `json:"Review_avatar_path"`
		Updated_at_comment int    `json:"Updated_at_comment"`
		Rating             int    `json:"Rating"`
		Comment            string `json:"Comment"`
		Typee              int    `json:"Type"`

		Rep_id      int    `json:"Rep_id"`
		Rep_comment string `json:"Rep_comment"`
		Rep_date    int    `json:"Rep_date"`

		Rep_owner_id   int    `json:"Rep_owner_id"`
		Rep_owner_name string `json:"Rep_owner_name"`
		Rep_owner_ava  string `json:"Rep_owner_ava"`
	}

	type Reviews struct {
		Review_list  []Review_type `json:"Review_list"`
		Rating_num   float32       `json:"Rating_num"`
		Star_five    int           `json:"Star_five"`
		Star_four    int           `json:"Star_four"`
		Star_thre    int           `json:"Star_thre"`
		Star_two     int           `json:"Star_two"`
		Star_one     int           `json:"Star_one"`
		Review_count int           `json:"Review_count"`
	}
	mass := Reviews{}
	prod := []Review_type{}

	var Updated_at_time time.Time
	var Rep_date_time time.Time
	var Owner_ava_pwd string

	request, err := rep.Query(ctx,
		`
		WITH reply AS (
			SELECT DISTINCT
				reply_to_user.id AS rep_id,	
				reply_to_user.comment AS rep_comment,
				COALESCE(reply_to_user.updated_at, reply_to_user.created_at) AS rep_date,
				reply_to_user.review_id,

				ads.id AS ads_id,
				ads.owner_id,
				COALESCE(individual_user.name, company_user.name_of_company) AS owner_name,
				users.avatar_path
			FROM ads.reply_to_user
			JOIN ads.reviews ON reviews.id = reply_to_user.review_id AND reviews.state != 3
			JOIN orders.bookings ON bookings.order_id = reviews.order_id
			JOIN ads.ads ON bookings.ads_id = $1
			LEFT JOIN users.individual_user ON individual_user.user_id = ads.owner_id
			LEFT JOIN users.company_user ON company_user.user_id = ads.owner_id
			LEFT JOIN users.users ON users.id = ads.owner_id
			WHERE reply_to_user.state != 3 AND ads.id = $1
		)
		SELECT DISTINCT
			reviews.id AS reviews_id,
			users.id AS users_id,
			COALESCE(individual_user.name, company_user.name_of_company) AS name,
			users.avatar_path,
			COALESCE(reviews.updated_at, reviews.created_at) AS updated_at,
			reviews.rating,
			reviews.type,
			COALESCE(reviews.comment, 'Пустой комментарий') AS comment,

			COALESCE((SELECT rep_id FROM reply WHERE review_id = reviews.id), 0),
			COALESCE((SELECT rep_comment FROM reply WHERE review_id = reviews.id), 'No comment'),
			COALESCE((SELECT rep_date FROM reply WHERE review_id = reviews.id), NOW()),
			COALESCE((SELECT owner_id FROM reply WHERE review_id = reviews.id), 0),
			COALESCE((SELECT owner_name FROM reply WHERE review_id = reviews.id), 'No name'),
			COALESCE((SELECT avatar_path FROM reply WHERE review_id = reviews.id), '/home/')
		FROM
			ads.reviews
		JOIN
			orders.orders ON reviews.order_id = orders.id
		JOIN
			orders.bookings ON bookings.order_id = orders.id
		JOIN
			finance.transactions ON bookings.transaction_id = transactions.id
		JOIN
			finance.wallets ON transactions.wallet_id = wallets.id
		JOIN
			users.users ON wallets.user_id = users.id
		LEFT JOIN
			users.individual_user ON users.id = individual_user.user_id
		LEFT JOIN
			users.company_user ON users.id = company_user.user_id
		WHERE
			bookings.ads_id = $1
		ORDER BY reviews_id DESC
		`, ads_id)
	errorr(err)

	for request.Next() {
		rev := Review_type{}
		err := request.Scan(
			&rev.Review_id,
			&rev.User_id,
			&rev.Review_name,
			&rev.Review_avatar_path,
			&Updated_at_time,
			&rev.Rating,
			&rev.Typee,
			&rev.Comment,
			&rev.Rep_id,
			&rev.Rep_comment,
			&Rep_date_time,

			&rev.Rep_owner_id,
			&rev.Rep_owner_name,
			&Owner_ava_pwd,
		)
		errorr(err)

		rev.Updated_at_comment = int(Updated_at_time.Unix())
		rev.Rep_date = int(Rep_date_time.Unix())

		if len(rev.Review_avatar_path) > 0 {
			rev.Review_avatar = ServeSpecificMediaBase64(rw, r, rev.Review_avatar_path)
			rev.Review_avatar_path = "/home/"

			rev.Rep_owner_ava = ServeSpecificMediaBase64(rw, r, Owner_ava_pwd)
		}
		prod = append(prod, rev)
	}

	mass.Review_list = prod

	request, err = rep.Query(ctx,
		`
		WITH stars AS (
			SELECT DISTINCT
				reviews.id,
				reviews.rating
			FROM ads.reviews
			JOIN orders.bookings ON bookings.order_id = reviews.order_id
			JOIN ads.ads ON bookings.ads_id = $1
			WHERE reviews.state != $1
		)
		SELECT DISTINCT
			users.rating,
			(SELECT COUNT(*) FROM stars WHERE rating = 5) AS five,
			(SELECT COUNT(*) FROM stars WHERE rating = 4) AS four,
			(SELECT COUNT(*) FROM stars WHERE rating = 3) AS thre,
			(SELECT COUNT(*) FROM stars WHERE rating = 2) AS two,
			(SELECT COUNT(*) FROM stars WHERE rating = 1) AS one
			FROM users.users, ads.ads
			WHERE ads.id = $1 AND ads.owner_id = users.id
		`, ads_id)
	errorr(err)

	for request.Next() {
		err := request.Scan(
			&mass.Rating_num,
			&mass.Star_five,
			&mass.Star_four,
			&mass.Star_thre,
			&mass.Star_two,
			&mass.Star_one,
		)
		errorr(err)

	}
	mass.Review_count = mass.Star_one + mass.Star_two + mass.Star_thre + mass.Star_four + mass.Star_five

	type Response struct {
		Status  string  `json:"status"`
		Data    Reviews `json:"data,omitempty"`
		Message string  `json:"message"`
	}

	if len(prod) == 0 || err != nil || request == nil {
		response := Response{
			Status:  "fatal",
			Message: "Объявление не найдено",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return err
	}

	fmt.Println(mass)

	response := Response{
		Status:  "success",
		Data:    mass,
		Message: "Объявление показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) GroupReviewLowRatOnesFirstSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, ads_id int) (err error) {
	type Review_type struct {
		Review_id          int    `json:"Review_id"`
		User_id            int    `json:"User_id"`
		Review_name        string `json:"Review_name"`
		Review_avatar      string `json:"Review_avatar"`
		Review_avatar_path string `json:"Review_avatar_path"`
		Updated_at_comment int    `json:"Updated_at_comment"`
		Rating             int    `json:"Rating"`
		Comment            string `json:"Comment"`
		Typee              int    `json:"Type"`

		Rep_id      int    `json:"Rep_id"`
		Rep_comment string `json:"Rep_comment"`
		Rep_date    int    `json:"Rep_date"`

		Rep_owner_id   int    `json:"Rep_owner_id"`
		Rep_owner_name string `json:"Rep_owner_name"`
		Rep_owner_ava  string `json:"Rep_owner_ava"`
	}

	type Reviews struct {
		Review_list  []Review_type `json:"Review_list"`
		Rating_num   float32       `json:"Rating_num"`
		Star_five    int           `json:"Star_five"`
		Star_four    int           `json:"Star_four"`
		Star_thre    int           `json:"Star_thre"`
		Star_two     int           `json:"Star_two"`
		Star_one     int           `json:"Star_one"`
		Review_count int           `json:"Review_count"`
	}
	mass := Reviews{}
	prod := []Review_type{}

	var Updated_at_time time.Time
	var Rep_date_time time.Time
	var Owner_ava_pwd string

	request, err := rep.Query(ctx,
		`
		WITH reply AS (
			SELECT DISTINCT
				reply_to_user.id AS rep_id,	
				reply_to_user.comment AS rep_comment,
				COALESCE(reply_to_user.updated_at, reply_to_user.created_at) AS rep_date,
				reply_to_user.review_id,

				ads.id AS ads_id,
				ads.owner_id,
				COALESCE(individual_user.name, company_user.name_of_company) AS owner_name,
				users.avatar_path
			FROM ads.reply_to_user
			JOIN ads.reviews ON reviews.id = reply_to_user.review_id AND reviews.state != 3
			JOIN orders.bookings ON bookings.order_id = reviews.order_id
			JOIN ads.ads ON bookings.ads_id = $1
			LEFT JOIN users.individual_user ON individual_user.user_id = ads.owner_id
			LEFT JOIN users.company_user ON company_user.user_id = ads.owner_id
			LEFT JOIN users.users ON users.id = ads.owner_id
			WHERE reply_to_user.state != 3 AND ads.id = $1
		)
		SELECT DISTINCT
			reviews.id AS reviews_id,
			users.id AS users_id,
			COALESCE(individual_user.name, company_user.name_of_company) AS name,
			users.avatar_path,
			COALESCE(reviews.updated_at, reviews.created_at) AS updated_at,
			reviews.rating,
			reviews.type,
			COALESCE(reviews.comment, 'Пустой комментарий') AS comment,

			COALESCE((SELECT rep_id FROM reply WHERE review_id = reviews.id), 0),
			COALESCE((SELECT rep_comment FROM reply WHERE review_id = reviews.id), 'No comment'),
			COALESCE((SELECT rep_date FROM reply WHERE review_id = reviews.id), NOW()),
			COALESCE((SELECT owner_id FROM reply WHERE review_id = reviews.id), 0),
			COALESCE((SELECT owner_name FROM reply WHERE review_id = reviews.id), 'No name'),
			COALESCE((SELECT avatar_path FROM reply WHERE review_id = reviews.id), '/home/')
		FROM
			ads.reviews
		JOIN
			orders.orders ON reviews.order_id = orders.id
		JOIN
			orders.bookings ON bookings.order_id = orders.id
		JOIN
			finance.transactions ON bookings.transaction_id = transactions.id
		JOIN
			finance.wallets ON transactions.wallet_id = wallets.id
		JOIN
			users.users ON wallets.user_id = users.id
		LEFT JOIN
			users.individual_user ON users.id = individual_user.user_id
		LEFT JOIN
			users.company_user ON users.id = company_user.user_id
		WHERE
			bookings.ads_id = $1
		ORDER BY rating ASC
		`, ads_id)
	errorr(err)

	for request.Next() {
		rev := Review_type{}
		err := request.Scan(
			&rev.Review_id,
			&rev.User_id,
			&rev.Review_name,
			&rev.Review_avatar_path,
			&Updated_at_time,
			&rev.Rating,
			&rev.Typee,
			&rev.Comment,
			&rev.Rep_id,
			&rev.Rep_comment,
			&Rep_date_time,

			&rev.Rep_owner_id,
			&rev.Rep_owner_name,
			&Owner_ava_pwd,
		)
		errorr(err)

		rev.Updated_at_comment = int(Updated_at_time.Unix())
		rev.Rep_date = int(Rep_date_time.Unix())

		if len(rev.Review_avatar_path) > 0 {
			rev.Review_avatar = ServeSpecificMediaBase64(rw, r, rev.Review_avatar_path)
			rev.Review_avatar_path = "/home/"

			rev.Rep_owner_ava = ServeSpecificMediaBase64(rw, r, Owner_ava_pwd)
		}
		prod = append(prod, rev)
	}

	mass.Review_list = prod

	request, err = rep.Query(ctx,
		`
		WITH stars AS (
			SELECT DISTINCT
				reviews.id,
				reviews.rating
			FROM ads.reviews
			JOIN orders.bookings ON bookings.order_id = reviews.order_id
			JOIN ads.ads ON bookings.ads_id = $1
			WHERE reviews.state != $1
		)
		SELECT DISTINCT
			users.rating,
			(SELECT COUNT(*) FROM stars WHERE rating = 5) AS five,
			(SELECT COUNT(*) FROM stars WHERE rating = 4) AS four,
			(SELECT COUNT(*) FROM stars WHERE rating = 3) AS thre,
			(SELECT COUNT(*) FROM stars WHERE rating = 2) AS two,
			(SELECT COUNT(*) FROM stars WHERE rating = 1) AS one
			FROM users.users, ads.ads
			WHERE ads.id = $1 AND ads.owner_id = users.id
		`, ads_id)
	errorr(err)

	for request.Next() {
		err := request.Scan(
			&mass.Rating_num,
			&mass.Star_five,
			&mass.Star_four,
			&mass.Star_thre,
			&mass.Star_two,
			&mass.Star_one,
		)
		errorr(err)

	}
	mass.Review_count = mass.Star_one + mass.Star_two + mass.Star_thre + mass.Star_four + mass.Star_five

	type Response struct {
		Status  string  `json:"status"`
		Data    Reviews `json:"data,omitempty"`
		Message string  `json:"message"`
	}

	if len(prod) == 0 || err != nil || request == nil {
		response := Response{
			Status:  "fatal",
			Message: "Объявление не найдено",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return err
	}

	fmt.Println(mass)

	response := Response{
		Status:  "success",
		Data:    mass,
		Message: "Объявление показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) GroupReviewHigRatOnesFirstSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, ads_id int) (err error) {
	type Review_type struct {
		Review_id          int    `json:"Review_id"`
		User_id            int    `json:"User_id"`
		Review_name        string `json:"Review_name"`
		Review_avatar      string `json:"Review_avatar"`
		Review_avatar_path string `json:"Review_avatar_path"`
		Updated_at_comment int    `json:"Updated_at_comment"`
		Rating             int    `json:"Rating"`
		Comment            string `json:"Comment"`
		Typee              int    `json:"Type"`

		Rep_id      int    `json:"Rep_id"`
		Rep_comment string `json:"Rep_comment"`
		Rep_date    int    `json:"Rep_date"`

		Rep_owner_id   int    `json:"Rep_owner_id"`
		Rep_owner_name string `json:"Rep_owner_name"`
		Rep_owner_ava  string `json:"Rep_owner_ava"`
	}

	type Reviews struct {
		Review_list  []Review_type `json:"Review_list"`
		Rating_num   float32       `json:"Rating_num"`
		Star_five    int           `json:"Star_five"`
		Star_four    int           `json:"Star_four"`
		Star_thre    int           `json:"Star_thre"`
		Star_two     int           `json:"Star_two"`
		Star_one     int           `json:"Star_one"`
		Review_count int           `json:"Review_count"`
	}
	mass := Reviews{}
	prod := []Review_type{}

	var Updated_at_time time.Time
	var Rep_date_time time.Time
	var Owner_ava_pwd string

	request, err := rep.Query(ctx,
		`
		WITH reply AS (
			SELECT DISTINCT
				reply_to_user.id AS rep_id,	
				reply_to_user.comment AS rep_comment,
				COALESCE(reply_to_user.updated_at, reply_to_user.created_at) AS rep_date,
				reply_to_user.review_id,

				ads.id AS ads_id,
				ads.owner_id,
				COALESCE(individual_user.name, company_user.name_of_company) AS owner_name,
				users.avatar_path
			FROM ads.reply_to_user
			JOIN ads.reviews ON reviews.id = reply_to_user.review_id AND reviews.state != 3
			JOIN orders.bookings ON bookings.order_id = reviews.order_id
			JOIN ads.ads ON bookings.ads_id = $1
			LEFT JOIN users.individual_user ON individual_user.user_id = ads.owner_id
			LEFT JOIN users.company_user ON company_user.user_id = ads.owner_id
			LEFT JOIN users.users ON users.id = ads.owner_id
			WHERE reply_to_user.state != 3 AND ads.id = $1
		)
		SELECT DISTINCT
			reviews.id AS reviews_id,
			users.id AS users_id,
			COALESCE(individual_user.name, company_user.name_of_company) AS name,
			users.avatar_path,
			COALESCE(reviews.updated_at, reviews.created_at) AS updated_at,
			reviews.rating,
			reviews.type,
			COALESCE(reviews.comment, 'Пустой комментарий') AS comment,

			COALESCE((SELECT rep_id FROM reply WHERE review_id = reviews.id), 0),
			COALESCE((SELECT rep_comment FROM reply WHERE review_id = reviews.id), 'No comment'),
			COALESCE((SELECT rep_date FROM reply WHERE review_id = reviews.id), NOW()),
			COALESCE((SELECT owner_id FROM reply WHERE review_id = reviews.id), 0),
			COALESCE((SELECT owner_name FROM reply WHERE review_id = reviews.id), 'No name'),
			COALESCE((SELECT avatar_path FROM reply WHERE review_id = reviews.id), '/home/')
		FROM
			ads.reviews
		JOIN
			orders.orders ON reviews.order_id = orders.id
		JOIN
			orders.bookings ON bookings.order_id = orders.id
		JOIN
			finance.transactions ON bookings.transaction_id = transactions.id
		JOIN
			finance.wallets ON transactions.wallet_id = wallets.id
		JOIN
			users.users ON wallets.user_id = users.id
		LEFT JOIN
			users.individual_user ON users.id = individual_user.user_id
		LEFT JOIN
			users.company_user ON users.id = company_user.user_id
		WHERE
			bookings.ads_id = $1
		ORDER BY rating DESC
		`, ads_id)
	errorr(err)

	for request.Next() {
		rev := Review_type{}
		err := request.Scan(
			&rev.Review_id,
			&rev.User_id,
			&rev.Review_name,
			&rev.Review_avatar_path,
			&Updated_at_time,
			&rev.Rating,
			&rev.Typee,
			&rev.Comment,
			&rev.Rep_id,
			&rev.Rep_comment,
			&Rep_date_time,

			&rev.Rep_owner_id,
			&rev.Rep_owner_name,
			&Owner_ava_pwd,
		)
		errorr(err)

		rev.Updated_at_comment = int(Updated_at_time.Unix())
		rev.Rep_date = int(Rep_date_time.Unix())

		if len(rev.Review_avatar_path) > 0 {
			rev.Review_avatar = ServeSpecificMediaBase64(rw, r, rev.Review_avatar_path)
			rev.Review_avatar_path = "/home/"

			rev.Rep_owner_ava = ServeSpecificMediaBase64(rw, r, Owner_ava_pwd)
		}
		prod = append(prod, rev)
	}

	mass.Review_list = prod

	request, err = rep.Query(ctx,
		`
		WITH stars AS (
			SELECT DISTINCT
				reviews.id,
				reviews.rating
			FROM ads.reviews
			JOIN orders.bookings ON bookings.order_id = reviews.order_id
			JOIN ads.ads ON bookings.ads_id = $1
			WHERE reviews.state != $1
		)
		SELECT DISTINCT
			users.rating,
			(SELECT COUNT(*) FROM stars WHERE rating = 5) AS five,
			(SELECT COUNT(*) FROM stars WHERE rating = 4) AS four,
			(SELECT COUNT(*) FROM stars WHERE rating = 3) AS thre,
			(SELECT COUNT(*) FROM stars WHERE rating = 2) AS two,
			(SELECT COUNT(*) FROM stars WHERE rating = 1) AS one
			FROM users.users, ads.ads
			WHERE ads.id = $1 AND ads.owner_id = users.id
		`, ads_id)
	errorr(err)

	for request.Next() {
		err := request.Scan(
			&mass.Rating_num,
			&mass.Star_five,
			&mass.Star_four,
			&mass.Star_thre,
			&mass.Star_two,
			&mass.Star_one,
		)
		errorr(err)

	}
	mass.Review_count = mass.Star_one + mass.Star_two + mass.Star_thre + mass.Star_four + mass.Star_five

	type Response struct {
		Status  string  `json:"status"`
		Data    Reviews `json:"data,omitempty"`
		Message string  `json:"message"`
	}

	if len(prod) == 0 || err != nil || request == nil {
		response := Response{
			Status:  "fatal",
			Message: "Объявление не найдено",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return err
	}

	fmt.Println(mass)

	response := Response{
		Status:  "success",
		Data:    mass,
		Message: "Объявление показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return err
}
