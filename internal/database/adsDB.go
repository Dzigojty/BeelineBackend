package database

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/rs/zerolog"
)

func ConvertToString(oldStr *string) string {
	if oldStr != nil {
		return *oldStr // разыменуем указатель и присвоим значение в str
	} else {
		return "" // можно использовать пустую строку или любое другое значение по умолчанию
	}
}

func (repo *MyRepository) ProductListSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, user_id *int, ads_list []int) (err error) {
	type Product struct {
		Ads_path    string //это фотки объявления
		Avatar_path string //это аватарка юзера

		Ads_photo string
		Avatar    string

		Title         string
		Hourly_rate   int
		Description   string
		Duration      string
		Created_at    time.Time
		Favorite_flag bool
		User_name     string
		Rating        float64
		Review_count  int

		Ads_id      int
		Owner_id    int
		Category_id int
	}

	var Favorite_flag_mass []bool
	var Review_count_mass []int

	products := []Product{}
	request, err := rep.Query(
		ctx,
		`
		WITH duration AS (
			WITH duration AS (
				SELECT
					ads.id,
					ads.owner_id,
					ARRAY[
						COALESCE(MAX(bookings.starts_at), '1900-01-01'),
						COALESCE(MAX(bookings.ends_at), '1900-01-01')
					] AS date_range
				FROM 
					ads.ads
				LEFT JOIN 
					orders.bookings
				ON 
					bookings.ads_id = ads.id
				WHERE 
					ads.id = ANY($2::INT[])
				GROUP BY 
					ads.id, ads.owner_id
			)
			SELECT * FROM duration
		)
		SELECT
			COALESCE(t1.File_path::TEXT, '/root/') AS Ads_path,
			t2.Title::TEXT,
			t2.Hourly_rate,
			t2.Description::TEXT,
			COALESCE((Duration(
				(SELECT d.date_range::date[] FROM duration d WHERE d.id = t2.id)
			))[1], 'Нет точной информации'),
			t2.Created_at,
			Favorite_flag($1, $2::INT[]),
			t4.Avatar_path::TEXT as User_avatar,
			COALESCE(t3.Name::TEXT, t5.name_of_company::TEXT) as User_name,
			t4.Rating,
			Review_count($2::INT[]),
			t2.Id as Ads_id,
			t2.Owner_id,
			t2.Category_id
		FROM
			ads.ads t2
		LEFT JOIN
			ads.ad_photos t1
			ON t2.id = t1.ad_id  -- Соединение на уровне объявления
		LEFT JOIN
			users.individual_user t3
			ON t3.user_id = t2.owner_id
		LEFT JOIN
			users.company_user t5
			ON t5.user_id = t2.owner_id
		LEFT JOIN
			users.users t4
			ON t4.id = t2.owner_id
		WHERE
			t2.status = true
			AND t2.id = ANY($2::INT[]);
		`,
		user_id,
		ads_list)
	errorr(err)

	for request.Next() {
		p := Product{}
		err := request.Scan(
			&p.Ads_path, //кладем сюда множество, длиною в три ӕлемента, с путями фоток
			&p.Title,
			&p.Hourly_rate,
			&p.Description,
			&p.Duration,
			&p.Created_at,
			&Favorite_flag_mass,
			&p.Avatar_path,
			&p.User_name,
			&p.Rating,
			&Review_count_mass,

			&p.Ads_id,
			&p.Owner_id,
			&p.Category_id,
		)
		if err != nil {
			fmt.Errorf("Error", err)
			continue
		}
		products = append(products, p)
	}

	for i := 0; i < len(products); i++ { //пока что у нас три объявления
		products[i].Favorite_flag = Favorite_flag_mass[i]
		products[i].Review_count = Review_count_mass[i]

		for j := 0; j < len(products[i].Ads_path); j++ {
			products[i].Ads_photo = ServeSpecificMediaBase64(rw, r, products[i].Ads_path)
		}

		products[i].Avatar = ServeSpecificMediaBase64(rw, r, products[i].Avatar_path)
	}

	type Response struct {
		Status  string    `json:"status"`
		Data    []Product `json:"data,omitempty"`
		Message string    `json:"message"`
	}

	if err != nil || len(products) == 0 {
		response := Response{
			Status:  "fatal",
			Message: "Не показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return err
	}

	response := Response{
		Status:  "success",
		Data:    products,
		Message: "Показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)
	return
}

type Review_type struct {
	Review_id          *int    `json:"Review_id"`
	User_id            *int    `json:"User_id"`
	Review_name        *string `json:"Review_name"`
	Review_avatar      *string `json:"Review_avatar"`
	Review_avatar_path *string `json:"Review_avatar_path"`
	Updated_at_comment *int    `json:"Updated_at_comment"`
	Rating             *int    `json:"Rating"`
	Comment            *string `json:"Comment"`
	State              *int    `json:"State"`
}

type ForPrintAds struct {
	Title        string        `json:"Title"`
	Images       []string      `json:"Imags"`
	File_path    []string      `json:"File_path"`
	Updated_at   int           `json:"Updated_at"`
	Description  string        `json:"Description"`
	Position     pgtype.Point  `json:"Position"`
	Reviews      []Review_type `json:"reviews"`
	Review_count int           `json:"Review_count"`
	Hourly_rate  int           `json:"Hourly_rate"`
	Daily_rate   int           `json:"Daily_rate"`
	Ads_id       int           `json:"Ads_id"`
	Owner_id     int           `json:"Owner_id"`
	Owner_name   string        `json:"Owner_name"`
	Avatar       string        `json:"Avatar"`
	Avatar_path  string        `json:"Avatar_path"`
	Owner_rating float32       `json:"Owner_rating"`
	Duration     string        `json:"Duration"`
	Ads_count    int           `json:"Ads_count"`
	Category_id  int           `json:"Category_id"`
}

func (repo *MyRepository) PrintAdsSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, ads_id int) (err error) {
	prod := []ForPrintAds{}

	request, err := rep.Query(ctx, `
		WITH CustomerReviews AS (
			SELECT DISTINCT
			reviews.id AS reviews_id,
			users.id AS user_id,
			COALESCE(individual_user.name, company_user.name_of_company) AS name,
			users.avatar_path,
			COALESCE(reviews.updated_at, reviews.created_at) AS updated_at,
			reviews.rating,
			reviews.state,
			COALESCE(reviews.comment, 'Пустой комментарий') AS comment
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
			),
			duration AS (
				SELECT 
				ads.id,
				ads.owner_id,
				ARRAY[
					MAX(bookings.starts_at),
					MAX(bookings.ends_at)
					] AS date_range
				FROM ads.ads, orders.bookings
				WHERE bookings.ads_id = ads.id
				GROUP BY ads.id, ads.owner_id
		),
		owner_info AS (
			SELECT 
				ads.owner_id AS owner_id
			FROM ads.ads
			WHERE ads.id = $1
		),
		ads_info AS (
			SELECT COUNT(*) AS ads_count
			FROM ads.ads
			WHERE ads.owner_id = (SELECT owner_id FROM owner_info)
		)
		SELECT
			t2.title,
			COALESCE(
			CASE WHEN t1.file_path IS NULL THEN ARRAY['/home/'] ELSE ARRAY[t1.file_path] END,
			ARRAY['/home/']
		) AS file_path,
		t2.updated_at,
		t2.description,
		t2.position::TEXT,

		ARRAY(SELECT cr.reviews_id FROM CustomerReviews cr LIMIT 2) AS id,
		ARRAY(SELECT cr.user_id FROM CustomerReviews cr LIMIT 2) AS id,
		ARRAY(SELECT cr.name FROM CustomerReviews cr LIMIT 2) AS name,
		ARRAY(SELECT cr.avatar_path FROM CustomerReviews cr LIMIT 2) AS avatar_path,
		ARRAY(SELECT cr.updated_at FROM CustomerReviews cr LIMIT 2) AS updated_at,
		ARRAY(SELECT cr.rating FROM CustomerReviews cr LIMIT 2) AS rating,
		ARRAY(SELECT COALESCE(cr.comment, 'Пустой коммент') FROM CustomerReviews cr LIMIT 2) AS comment,
		ARRAY(SELECT cr.state FROM CustomerReviews cr LIMIT 2) AS state,
		CARDINALITY(ARRAY_AGG(cr.state)) AS Review_count, 

		t2.hourly_rate,
		t2.daily_rate, 
		t2.id AS ads_id,
		t2.owner_id,
		COALESCE(ind_us.Name::TEXT, comp_us.name_of_company::TEXT) AS owner_name,
		t4.avatar_path AS Avatar_path,
		t4.rating,
		COALESCE(
		    (Duration(
		        (SELECT COALESCE(
		            NULLIF(array_remove(d.date_range, NULL), '{}'), -- Удаление NULL и проверка на пустой массив
		            ARRAY['2024-12-12'::date, '2024-12-14'::date]
		        ) 
		        FROM duration d 
		        WHERE d.id = t2.id)
		    ))[1],
		    'Нет точной информации'
		) AS Duration,
		ads_info.ads_count,
		t2.category_id
	FROM
		ads.ads t2
	LEFT JOIN
		ads.ad_photos t1 ON t2.id = t1.ad_id
	LEFT JOIN
		users.individual_user ind_us ON ind_us.user_id = t2.owner_id
	LEFT JOIN
		users.company_user comp_us ON comp_us.user_id = t2.owner_id
	INNER JOIN
		users.users t4 ON t2.owner_id = t4.id
	LEFT JOIN ads_info ON TRUE
	LEFT JOIN
		CustomerReviews cr ON TRUE
	WHERE
		t2.status = true AND t2.id = $1
	GROUP BY
		t2.id, t2.title, t2.updated_at, t2.description, t2.position::TEXT, t2.hourly_rate, t2.daily_rate, t2.owner_id, t4.rating, t4.avatar_path, ind_us.Name, comp_us.name_of_company, t1.file_path, ads_info.ads_count
	LIMIT 1;
		`, ads_id)
	errorr(err)

	var Avatar_cost []string
	var Updated_at_int time.Time

	for request.Next() {
		rev := ForPrintAds{}
		var Review_id *[]int
		var User_id *[]int
		var Name_cost *[]string
		var Avatar_path_cost *[]string
		var Updated_at_cost *[]time.Time
		var Rating_cost *[]int
		var Comment_cost *[]string
		var State_cost *[]int

		err := request.Scan(
			&rev.Title,
			&rev.File_path,
			&Updated_at_int,
			&rev.Description,
			&rev.Position,

			&Review_id,
			&User_id,
			&Name_cost,
			&Avatar_path_cost,
			&Updated_at_cost,
			&Rating_cost,
			&Comment_cost,
			&State_cost,

			&rev.Review_count,
			&rev.Hourly_rate,
			&rev.Daily_rate,
			&rev.Ads_id,
			&rev.Owner_id,
			&rev.Owner_name,
			&rev.Avatar_path,
			&rev.Owner_rating,
			&rev.Duration,
			&rev.Ads_count,
			&rev.Category_id,
		)

		rev.Updated_at = int(Updated_at_int.Unix())

		if Review_id == nil || len(*Review_id) == 0 {
			rev.Review_count = 0
		}

		if err != nil {
			return err
		}

		if len(rev.File_path) > 0 {
			for i := 0; i < len(rev.File_path); i++ {
				rev.Images = append(rev.Images, ServeSpecificMediaBase64(rw, r, rev.File_path[i]))
			}
		}
		if len(*Avatar_path_cost) > 0 {
			for i := 0; i < len(*Avatar_path_cost); i++ {
				Avatar_cost = append(Avatar_cost, ServeSpecificMediaBase64(rw, r, (*Avatar_path_cost)[i]))
			}
		}

		if len(*Review_id) > 0 && len(*User_id) > 0 && len(*Name_cost) > 0 &&
			len(*Avatar_path_cost) > 0 && len(*Updated_at_cost) > 0 &&
			len(*Rating_cost) > 0 && len(*Comment_cost) > 0 && len(*State_cost) > 0 {

			rev.Reviews = append(rev.Reviews, Review_type{
				Review_id:     &(*Review_id)[0],
				User_id:       &(*User_id)[0],
				Review_name:   &(*Name_cost)[0],
				Review_avatar: &Avatar_cost[0], // Проверьте, что Avatar_cost инициализирован
				Updated_at_comment: func(t time.Time) *int {
					i := int(t.Unix())
					return &i
				}((*Updated_at_cost)[0]),
				Rating:  &(*Rating_cost)[0],
				Comment: &(*Comment_cost)[0],
				State:   &(*State_cost)[0],
			})

			if len(*Review_id) > 1 && len(*User_id) > 1 && len(*Name_cost) > 1 &&
				len(*Avatar_path_cost) > 1 && len(*Updated_at_cost) > 1 &&
				len(*Rating_cost) > 1 && len(*Comment_cost) > 1 && len(*State_cost) > 1 {

				rev.Reviews = append(rev.Reviews, Review_type{
					Review_id:     &(*Review_id)[1],
					User_id:       &(*User_id)[1],
					Review_name:   &(*Name_cost)[1],
					Review_avatar: &Avatar_cost[1], // Проверьте, что Avatar_cost инициализирован
					Updated_at_comment: func(t time.Time) *int {
						i := int(t.Unix())
						return &i
					}((*Updated_at_cost)[1]),
					Rating:  &(*Rating_cost)[1],
					Comment: &(*Comment_cost)[1],
					State:   &(*State_cost)[1],
				})
			}
		}

		rev.Avatar = ServeSpecificMediaBase64(rw, r, rev.Avatar_path)

		prod = append(prod, rev)
	}

	type Response struct {
		Status  string      `json:"status"`
		Data    ForPrintAds `json:"data,omitempty"`
		Message string      `json:"message"`
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

	response := Response{
		Status:  "success",
		Data:    prod[0],
		Message: "Объявление показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) SortProductListDailyRateSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, list int, size int, category []int, lowNum, higNum int, lowDate, higDate time.Time, position []float64, location string, distance, rating int) (err error) {
	type Product struct {
		Id *int

		Ads_path  string
		Ads_photo string

		Daily_rate         *int
		Title              *string
		Description        *string
		Category_id        *int
		Name               *string
		Surname_or_ind_num *string
		Owner_id           *int
		Rating             *float32
		Location           string

		Avatar_path  string
		Avatar_photo string

		Review_count int
		Duration     string
	}
	products := []Product{}

	request, err := rep.Query(
		ctx,
		`
		WITH duration AS (
			SELECT
				ads.id,
				ads.owner_id,
				ARRAY[
					COALESCE(MAX(bookings.starts_at), '1900-01-01'),
					COALESCE(MAX(bookings.ends_at), '1900-01-01')
				] AS date_range
			FROM 
				ads.ads
			LEFT JOIN 
				orders.bookings
			ON 
				bookings.ads_id = ads.id
			GROUP BY 
				ads.id, ads.owner_id
		)
		SELECT DISTINCT ON (t2.Id)
			t2.Id,
			COALESCE(t1.File_path::TEXT, '/home/'),
			t2.Daily_rate,
			t2.Title::TEXT,
			t2.Description,
			t2.Category_id,
			COALESCE(t3.name::TEXT, t5.name_of_company::TEXT) AS Name,
			COALESCE(t3.surname::TEXT, t5.ind_num_taxp::TEXT) AS Surname_or_ind_num,
			t2.Owner_id,
			t4.Rating,
			t4.Avatar_path,
			(Review_count(ARRAY[t2.id]::INT[]))[1],
			COALESCE((Duration(
				(SELECT d.date_range::date[] FROM duration d WHERE d.id = t2.id)
			))[1], 'Нет точной информации'),
			t2.Location
		FROM
			ads.ads t2
		LEFT JOIN
			ads.ad_photos t1 ON t2.id = t1.ad_id
			AND t2.status = true
		LEFT JOIN
			users.individual_user t3 ON t3.user_id = t2.owner_id
		LEFT JOIN
			users.company_user t5 ON t5.user_id = t2.owner_id
		INNER JOIN
			users.users t4 ON t2.owner_id = t4.id
		WHERE
			t2.status = true AND
			(t2.Location ILIKE $10 OR t2.Location ILIKE '%' || $11 || '%' OR $11 ILIKE '%' || t2.Location || '%') AND
			(cardinality($1::INT[]) = 0 OR t2.category_id = ANY($1::INT[]))
			AND $2 <= t2.Daily_rate AND t2.Daily_rate <= $3
			AND EXISTS (
				WITH booking_array AS (
					SELECT 
						COALESCE(array_agg(bookings.starts_at::date), ARRAY[NULL::date]) AS starts_at_list,
						COALESCE(array_agg(bookings.ends_at::date), ARRAY[NULL::date]) AS ends_at_list,
						COALESCE(array_agg(bookings.id), ARRAY[NULL::int]) AS bookings_id_list,
						COALESCE(bookings.ads_id, 0) AS ads_id_list
					FROM 
						orders.orders AS orders
					LEFT JOIN 
						orders.bookings AS bookings
						ON orders.id = bookings.order_id AND bookings.ads_id = t2.id
					GROUP BY 
						bookings.ads_id
					UNION ALL
					SELECT 
						ARRAY[NULL::date],
						ARRAY[NULL::date],
						ARRAY[NULL::int],
						0
					WHERE NOT EXISTS (SELECT 1 FROM orders.orders)
				)
				SELECT
					CASE
						WHEN NOT EXISTS (SELECT 1 FROM orders.orders) THEN TRUE
						WHEN booking_array.ads_id_list IS NULL OR booking_array.ads_id_list = 0 THEN TRUE
						WHEN cardinality(array_remove(booking_array.starts_at_list, NULL)) = 0 THEN TRUE
						WHEN ($4::date >= ALL(booking_array.starts_at_list) 
							AND $5::date >= ALL(booking_array.ends_at_list))
						OR ($4::date <= ALL(booking_array.starts_at_list) 
							AND $5::date <= ALL(booking_array.ends_at_list))
						THEN TRUE 
						ELSE FALSE 
					END AS date_flag
				FROM 
					booking_array
							)
							AND ST_Distance(
									ST_SetSRID(ST_MakePoint($6::float8, $7::float8), 4326)::geography,
									ST_SetSRID(ST_MakePoint(t2.Position[0], t2.Position[1]), 4326)::geography
								) < $8
						-- сортировка по радиусу
						
						AND (COALESCE(t4.rating = $9, TRUE) OR COALESCE($9 = 0, TRUE))
						-- сортировка по рейтингу
						ORDER BY
							t2.Id;
		`,

		category,
		lowNum,
		higNum,
		lowDate,
		higDate,
		position[0],
		position[1],
		distance,
		rating,
		location+"%",
		location,
	)
	fmt.Println(err)

	for request.Next() {
		p := Product{}
		err := request.Scan(
			&p.Id,
			&p.Ads_path,
			&p.Daily_rate,
			&p.Title,
			&p.Description,
			&p.Category_id,
			&p.Name,
			&p.Surname_or_ind_num,
			&p.Owner_id,
			&p.Rating,
			&p.Avatar_path,
			&p.Review_count,
			&p.Duration,
			&p.Location,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}

		products = append(products, p)
	}

	for i := 0; i < len(products); i++ { //пока что у нас три объявления
		products[i].Ads_photo = ServeSpecificMediaBase64(rw, r, products[i].Ads_path)

		products[i].Avatar_photo = ServeSpecificMediaBase64(rw, r, products[i].Avatar_path)

	}

	type Response struct {
		Status  string    `json:"status"`
		Lenght  int       `json:"Lenght"`
		Data    []Product `json:"data,omitempty"`
		Message string    `json:"message"`
	}

	if len(products) < list*size {

		if len(products) < (list*size)-size {
			response := Response{
				Status:  "fatal",
				Message: "Диапозон выходит за рамки ",
			}

			rw.WriteHeader(http.StatusOK)
			json.NewEncoder(rw).Encode(response)

			return err
		}

		response := Response{
			Status:  "success",
			Lenght:  len(products),
			Data:    products[(list*size)-size:],
			Message: "Показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return
	}

	if err == nil && request != nil && len(products) != 0 {
		response := Response{
			Status:  "success",
			Lenght:  len(products),
			Data:    products[(list*size)-size : (list * size)],
			Message: "Показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return
	}

	response := Response{
		Status:  "fatal",
		Message: "Не показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) SortProductListHourlyRateSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, list int, size int, category []int, lowNum, higNum int, lowDate, higDate time.Time, position []float64, distance, rating int, location string) (err error) {
	type Product struct {
		Id *int

		Ads_path  string
		Ads_photo string

		Hourly_rate        *int
		Title              *string
		Description        *string
		Category_id        *int
		Name               *string
		Surname_or_ind_num *string
		Owner_id           *int
		Rating             *float32
		Location           string

		Avatar_path  string
		Avatar_photo string

		Review_count int
		Duration     string
	}
	products := []Product{}

	request, err := rep.Query(
		ctx,
		`
		WITH duration AS (
			SELECT
				ads.id,
				ads.owner_id,
				ARRAY[
					COALESCE(MAX(bookings.starts_at), '1900-01-01'),
					COALESCE(MAX(bookings.ends_at), '1900-01-01')
				] AS date_range
			FROM 
				ads.ads
			LEFT JOIN 
				orders.bookings
			ON 
				bookings.ads_id = ads.id
			GROUP BY 
				ads.id, ads.owner_id
		)
		SELECT DISTINCT ON (t2.Id)
			t2.Id,
			COALESCE(t1.File_path::TEXT, '/home/'),
			t2.Hourly_rate,
			t2.Title::TEXT,
			t2.Description,
			t2.Category_id,
			COALESCE(t3.name::TEXT, t5.name_of_company::TEXT) AS Name,
			COALESCE(t3.surname::TEXT, t5.ind_num_taxp::TEXT) AS Surname_or_ind_num,
			t2.Owner_id,
			t4.Rating,
			t4.Avatar_path,
			(Review_count(ARRAY[t2.id]::INT[]))[1],
			COALESCE((Duration(
				(SELECT d.date_range::date[] FROM duration d WHERE d.id = t2.id)
			))[1], 'Нет точной информации'),
			t2.Location
		FROM
			ads.ads t2
		LEFT JOIN
			ads.ad_photos t1 ON t2.id = t1.ad_id
			AND t2.status = true
		LEFT JOIN
			users.individual_user t3 ON t3.user_id = t2.owner_id
		LEFT JOIN
			users.company_user t5 ON t5.user_id = t2.owner_id
		INNER JOIN
			users.users t4 ON t2.owner_id = t4.id
		WHERE
			t2.status = true AND
			(t2.Location ILIKE $10 OR t2.Location ILIKE '%' || $11 || '%' OR $11 ILIKE '%' || t2.Location || '%') AND
			(cardinality($1::INT[]) = 0 OR t2.category_id = ANY($1::INT[]))
			AND $2 <= t2.Hourly_rate AND t2.Hourly_rate <= $3
			AND EXISTS (
				WITH booking_array AS (
					SELECT 
						COALESCE(array_agg(bookings.starts_at::date), ARRAY[NULL::date]) AS starts_at_list,
						COALESCE(array_agg(bookings.ends_at::date), ARRAY[NULL::date]) AS ends_at_list,
						COALESCE(array_agg(bookings.id), ARRAY[NULL::int]) AS bookings_id_list,
						COALESCE(bookings.ads_id, 0) AS ads_id_list
					FROM 
						orders.orders AS orders
					LEFT JOIN 
						orders.bookings AS bookings
						ON orders.id = bookings.order_id AND bookings.ads_id = t2.id
					GROUP BY 
						bookings.ads_id
					UNION ALL
					SELECT 
						ARRAY[NULL::date],
						ARRAY[NULL::date],
						ARRAY[NULL::int],
						0
					WHERE NOT EXISTS (SELECT 1 FROM orders.orders)
				)
			SELECT
				CASE
					WHEN NOT EXISTS (SELECT 1 FROM orders.orders) THEN TRUE
					WHEN booking_array.ads_id_list IS NULL OR booking_array.ads_id_list = 0 THEN TRUE
					WHEN cardinality(array_remove(booking_array.starts_at_list, NULL)) = 0 THEN TRUE
					WHEN ($4::date >= ALL(booking_array.starts_at_list) 
						AND $5::date >= ALL(booking_array.ends_at_list))
					OR ($4::date <= ALL(booking_array.starts_at_list) 
						AND $5::date <= ALL(booking_array.ends_at_list))
					THEN TRUE 
					ELSE FALSE 
				END AS date_flag
			FROM booking_array)
					AND ST_Distance(
							ST_SetSRID(ST_MakePoint($6::float8, $7::float8), 4326)::geography,
							ST_SetSRID(ST_MakePoint(t2.Position[0], t2.Position[1]), 4326)::geography
						) < $8
				-- сортировка по радиусу
				
				AND (COALESCE(t4.rating = $9, TRUE) OR COALESCE($9 = 0, TRUE))
				-- сортировка по рейтингу
				ORDER BY
					t2.Id;
		`,

		category,
		lowNum,
		higNum,
		lowDate,
		higDate,
		position[0],
		position[1],
		distance,
		rating,
		location+"%",
		location,
	)

	for request.Next() {
		p := Product{}
		err := request.Scan(
			&p.Id,
			&p.Ads_path,
			&p.Hourly_rate,
			&p.Title,
			&p.Description,
			&p.Category_id,
			&p.Name,
			&p.Surname_or_ind_num,
			&p.Owner_id,
			&p.Rating,
			&p.Avatar_path,
			&p.Review_count,
			&p.Duration,
			&p.Location,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}

		products = append(products, p)
	}

	for i := 0; i < len(products); i++ { //пока что у нас три объявления
		products[i].Ads_photo = ServeSpecificMediaBase64(rw, r, products[i].Ads_path)

		products[i].Avatar_photo = ServeSpecificMediaBase64(rw, r, products[i].Avatar_path)

	}

	type Response struct {
		Status  string    `json:"status"`
		Lenght  int       `json:"Lenght"`
		Data    []Product `json:"data,omitempty"`
		Message string    `json:"message"`
	}

	if len(products) < list*size {

		if len(products) < (list*size)-size {
			response := Response{
				Status:  "fatal",
				Message: "Диапозон выходит за рамки ",
			}

			rw.WriteHeader(http.StatusOK)
			json.NewEncoder(rw).Encode(response)

			return err
		}

		response := Response{
			Status:  "success",
			Lenght:  len(products),
			Data:    products[(list*size)-size:],
			Message: "Показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return
	}

	if err == nil && request != nil && len(products) != 0 {
		response := Response{
			Status:  "success",
			Lenght:  len(products),
			Data:    products[(list*size)-size : (list * size)],
			Message: "Показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return
	}

	response := Response{
		Status:  "fatal",
		Message: "Не показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) SignupAdsSQL(
	ctx context.Context,
	rw http.ResponseWriter,
	rep *pgxpool.Pool,
	title,
	description string,
	hourly_rate,
	daily_rate,
	owner_id,
	category_id int,
	PositionX,
	PositionY float64,
	Location string,
	updated_at time.Time,
	images []string,
	pwd_mass []string,
	pwd string) (err error) {

	request, err := rep.Query(ctx, `
			WITH i AS (
				INSERT INTO Ads.ads (title, description, hourly_rate, daily_rate, owner_id, category_id, position, updated_at, location) 
				VALUES ($1, $2, $3, $4, $5, $6, POINT($7, $8), $9, $10) 
				RETURNING id
			),
			p AS (
				INSERT INTO Ads.Ad_photos (ad_id, file_path, status)
				SELECT i.id, UNNEST($11::text[]), $12
				FROM i
				RETURNING ad_id
			)
			SELECT i.id FROM i;
		`,
		title,
		description,
		hourly_rate,
		daily_rate,
		owner_id,
		category_id,
		PositionX,
		PositionY,
		updated_at,
		Location,

		pwd_mass,
		false,
	)
	errorr(err)

	var ad_id int
	for request.Next() {
		err := request.Scan(
			&ad_id,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err != nil || ad_id <= 0 {
		flag, file_err := DeleteImageMass(rw, pwd, images)
		if flag {
			response := Response{
				Status:  "fatal",
				Message: "Проблемы с удалением фото: " + file_err.Error(),
			}

			rw.WriteHeader(http.StatusOK)
			json.NewEncoder(rw).Encode(response)

			return err
		}

		response := Response{
			Status:  "fatal",
			Message: "Объявление не зарегистирровано",
		}

		json.NewEncoder(rw).Encode(response)

		return err
	}

	response := Response{
		Status:  "success",
		Data:    ad_id,
		Message: "Объявление показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) EditAdsListSQL(
	ctx context.Context,
	rw http.ResponseWriter,
	rep *pgxpool.Pool,
	ad_id,
	owner_id int,
) (err error) {
	request, err := rep.Query(ctx,
		`
		WITH i AS (
			SELECT id, title, description, hourly_rate, daily_rate, category_id, location, position
			FROM Ads.ads
			WHERE id = $1 AND owner_id = $2
		),
		j AS (
			SELECT ad_id, file_path, uploaded_at
			FROM ads.ad_photos
			WHERE ad_id IN (SELECT id FROM i)-- Используем подзапрос, чтобы получить id ads
		)
		SELECT i.title, i.description, i.hourly_rate, i.daily_rate, i.category_id, i.location, i.position, j.ad_id, j.file_path, j.uploaded_at
		FROM i
		LEFT JOIN j ON j.ad_id = i.id;
		`,

		ad_id,
		owner_id,
	)
	errorr(err)

	type Ads struct {
		Images      []string     `json:"Images"`
		Title       string       `json:"Title"`
		Description string       `json:"Description"`
		Hourly_rate int          `json:"Hourly_rate"`
		Daily_rate  int          `json:"Daily_rate"`
		Category_id int          `json:"Category_id"`
		Location    string       `json:"Location"`
		Point       pgtype.Point `json:"Point"`

		Ads_id      int       `json:"Ads_id"`
		File_path   []string  `json:"File_path"`
		Uploaded_at time.Time `json:"Uploaded_at"`
	}

	mass := []Ads{}
	for request.Next() {
		p := Ads{}
		err := request.Scan(
			&p.Title,
			&p.Description,
			&p.Hourly_rate,
			&p.Daily_rate,
			&p.Category_id,
			&p.Location,
			&p.Point,

			&p.Ads_id,
			&p.File_path,
			&p.Uploaded_at,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}

		mass = append(mass, p)
	}

	type Response struct {
		Status  string `json:"status"`
		Data    Ads    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	for i := range len(mass[0].File_path) {
		image, err := DownloadFile(mass[0].File_path[i])
		if err != nil {
			rw.WriteHeader(http.StatusOK)
			json.NewEncoder(rw).Encode(Response{
				Status:  "fatal",
				Message: "Фото не найденно",
			})
			return err
		}

		mass[0].Images = append(mass[0].Images, image)
	}

	if err != nil && len(mass) == 0 {
		response := Response{
			Status:  "fatal",
			Message: "Объявление не изменено",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	}

	response := Response{
		Status: "success",
		Data: Ads{
			Images:      mass[0].Images,
			Title:       mass[0].Title,
			Description: mass[0].Description,
			Hourly_rate: mass[0].Hourly_rate,
			Daily_rate:  mass[0].Daily_rate,
			Category_id: mass[0].Category_id,
			Location:    mass[0].Location,
			Point:       mass[0].Point,
			Ads_id:      mass[0].Ads_id,
			Uploaded_at: mass[0].Uploaded_at,
		},
		Message: "Объявление показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)
	return err
}

func (repo *MyRepository) UpdAdsAddImgSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, file_path string, ad_id, owner_id int) (err error) {
	request, err := rep.Query(ctx,
		`
			INSERT INTO ads.ad_photos (ad_id, file_path)
			VALUES ($1, $2)
			RETURNING ad_id;
			`,
		ad_id,
		file_path,
	)

	var req_ad_id int
	for request.Next() {
		err := request.Scan(
			&req_ad_id,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}

		type Response struct {
			Status  string `json:"status"`
			Data    int    `json:"data,omitempty"`
			Message string `json:"message"`
		}

		if err != nil && req_ad_id == 0 {
			response := Response{
				Status:  "fatal",
				Message: "Объявление не изменено",
			}

			json.NewEncoder(rw).Encode(response)
			return err
		}

		response := Response{
			Status:  "success",
			Data:    req_ad_id,
			Message: "Фото изменено",
		}

		json.NewEncoder(rw).Encode(response)
		return err
	}

	return
}

func (repo *MyRepository) UpdAdsDelImgSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, img_id int) (err error) {
	request, err := rep.Query(ctx,
		`
			UPDATE Ads.ad_photos
				SET status = false,
				removed_at = NOW()
			WHERE id = $2
			RETURNING id;
			`,
		img_id,
	)

	var req_ad_id int
	for request.Next() {
		err := request.Scan(
			&req_ad_id,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}

		type Response struct {
			Status  string `json:"status"`
			Data    int    `json:"data,omitempty"`
			Message string `json:"message"`
		}

		if err != nil {
			response := Response{
				Status:  "fatal",
				Message: "Объявление не изменено",
			}

			json.NewEncoder(rw).Encode(response)
			return err
		}

		response := Response{
			Status:  "success",
			Data:    req_ad_id,
			Message: "Объявление показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	}

	return
}

func (repo *MyRepository) UpdAdsTitleSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, title string, ad_id, owner_id int) (err error) {
	request, err := rep.Query(ctx,
		`
			UPDATE Ads.ads
				SET title = $1,
				updated_at = NOW()
			WHERE id = $2 AND owner_id = $3
			RETURNING title;
			`,
		title,

		ad_id,
		owner_id,
	)

	var req_title string
	for request.Next() {
		err := request.Scan(
			&req_title,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}

		type Response struct {
			Status  string `json:"status"`
			Data    string `json:"data,omitempty"`
			Message string `json:"message"`
		}

		if err != nil && ad_id == 0 {
			response := Response{
				Status:  "fatal",
				Message: "Объявление не изменено",
			}

			json.NewEncoder(rw).Encode(response)
			return err
		}

		response := Response{
			Status:  "success",
			Data:    req_title,
			Message: "Объявление показано",
		}

		json.NewEncoder(rw).Encode(response)
		return err
	}

	return
}

func (repo *MyRepository) UpdAdsDescriptionSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, description string, ad_id, owner_id int) (err error) {
	request, err := rep.Query(ctx,
		`
		UPDATE Ads.ads
			SET description = $1,
			updated_at = NOW()
		WHERE id = $2 AND owner_id = $3
		RETURNING description;
		`,
		description,

		ad_id,
		owner_id,
	)

	var req_description string
	for request.Next() {
		err := request.Scan(
			&req_description,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}

		type Response struct {
			Status  string `json:"status"`
			Data    string `json:"data,omitempty"`
			Message string `json:"message"`
		}

		if err != nil && ad_id == 0 {
			response := Response{
				Status:  "fatal",
				Message: "Объявление не изменено",
			}

			json.NewEncoder(rw).Encode(response)
			return err
		}

		response := Response{
			Status:  "success",
			Data:    req_description,
			Message: "Объявление показано",
		}

		json.NewEncoder(rw).Encode(response)
		return err
	}

	return
}

func (repo *MyRepository) UpdAdsHourly_rateSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, hourly_rate, ad_id, owner_id int) (err error) {
	request, err := rep.Query(ctx,
		`
		UPDATE Ads.ads
			SET hourly_rate = $1,
			updated_at = NOW()
		WHERE id = $2 AND owner_id = $3
		RETURNING id, title;
		`,
		hourly_rate,

		ad_id,
		owner_id,
	)

	var req_hourly_rate int
	for request.Next() {
		err := request.Scan(
			&req_hourly_rate,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}

		type Response struct {
			Status  string `json:"status"`
			Data    int    `json:"data,omitempty"`
			Message string `json:"message"`
		}

		if err != nil && ad_id == 0 {
			response := Response{
				Status:  "fatal",
				Message: "Объявление не изменено",
			}

			json.NewEncoder(rw).Encode(response)
			return err
		}

		response := Response{
			Status:  "success",
			Data:    req_hourly_rate,
			Message: "Объявление показано",
		}

		json.NewEncoder(rw).Encode(response)
		return err
	}

	return
}

func (repo *MyRepository) UpdAdsDaily_rateSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, daily_rate, ad_id, owner_id int) (err error) {
	request, err := rep.Query(ctx,
		`
		UPDATE Ads.ads
			SET daily_rate = $1,
			updated_at = NOW()
		WHERE id = $2 AND owner_id = $3
		RETURNING daily_rate;
		`,
		daily_rate,

		ad_id,
		owner_id,
	)

	var req_daily_rate string
	for request.Next() {
		err := request.Scan(
			&req_daily_rate,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}

		type Response struct {
			Status  string `json:"status"`
			Data    string `json:"data,omitempty"`
			Message string `json:"message"`
		}

		if err != nil && ad_id == 0 {
			response := Response{
				Status:  "fatal",
				Message: "Объявление не изменено",
			}

			json.NewEncoder(rw).Encode(response)
			return err
		}

		response := Response{
			Status:  "success",
			Data:    req_daily_rate,
			Message: "Объявление показано",
		}

		json.NewEncoder(rw).Encode(response)
		return err
	}

	return
}

func (repo *MyRepository) UpdAdsCategory_idSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, category_id, ad_id, owner_id int) (err error) {
	request, err := rep.Query(ctx,
		`
		UPDATE Ads.ads
			SET category_id = $1,
			updated_at = NOW()
		WHERE id = $2 AND owner_id = $3
		RETURNING id, title;
		`,
		category_id,

		ad_id,
		owner_id,
	)

	var req_category_id int
	for request.Next() {
		err := request.Scan(
			&req_category_id,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}

		type Response struct {
			Status  string `json:"status"`
			Data    int    `json:"data,omitempty"`
			Message string `json:"message"`
		}

		if err != nil && ad_id == 0 {
			response := Response{
				Status:  "fatal",
				Message: "Объявление не изменено",
			}

			json.NewEncoder(rw).Encode(response)
			return err
		}

		response := Response{
			Status:  "success",
			Data:    req_category_id,
			Message: "Объявление показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	}

	return
}

func (repo *MyRepository) UpdAdsLocationSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, location string, ad_id, owner_id int) (err error) {
	request, err := rep.Query(ctx,
		`
			UPDATE Ads.ads
				SET location = $1,
				updated_at = NOW()
			WHERE id = $2 AND owner_id = $3
			RETURNING id, title;
			`,
		location,

		ad_id,
		owner_id,
	)

	var req_location string
	for request.Next() {
		err := request.Scan(
			&req_location,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}

		type Response struct {
			Status  string `json:"status"`
			Data    string `json:"data,omitempty"`
			Message string `json:"message"`
		}

		if err != nil && ad_id == 0 {
			response := Response{
				Status:  "fatal",
				Message: "Объявление не изменено",
			}

			json.NewEncoder(rw).Encode(response)
			return err
		}

		response := Response{
			Status:  "success",
			Data:    req_location,
			Message: "Объявление показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	}

	return
}

func (repo *MyRepository) UpdAdsPositionSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, position pgtype.Point, ad_id, owner_id int) (err error) {
	request, err := rep.Query(ctx,
		`
			UPDATE Ads.ads
				SET position = $1,
				updated_at = NOW()
			WHERE id = $2 AND owner_id = $3
			RETURNING position;
			`,
		position,

		ad_id,
		owner_id,
	)

	var req_position pgtype.Point
	for request.Next() {
		err := request.Scan(
			&req_position,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}

		type Response struct {
			Status  string       `json:"status"`
			Data    pgtype.Point `json:"data,omitempty"`
			Message string       `json:"message"`
		}

		if err != nil && req_position.Status == pgtype.Null {
			response := Response{
				Status:  "fatal",
				Message: "Позиция не изменена",
			}

			json.NewEncoder(rw).Encode(response)
			return err
		}

		response := Response{
			Status:  "success",
			Data:    req_position,
			Message: "Позиция изменена",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	}

	return
}

func (repo *MyRepository) DelAdsSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, ads_id int, owner_id int) (err error) {
	request, err := rep.Query(ctx,
		`UPDATE Ads.ads
		SET status = false
		WHERE id = $1 AND owner_id = $2
		RETURNING id;`, ads_id, owner_id)

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	var ad_id int
	for request.Next() {
		err := request.Scan(
			&ad_id,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	if err != nil || ad_id == 0 {
		err = fmt.Errorf("failed to exec data: %w", err)
		response := Response{
			Status:  "fatal",
			Message: "Объявление не удалено",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	} else if ad_id == ads_id {
		response := Response{
			Status:  "success",
			Data:    ad_id,
			Message: "Объявление удалено",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
	}

	return
}

type Int struct {
	Title  string `json:"Title"`
	Ads_id int    `json:"Ads_id"`
}

func (repo *MyRepository) SearchForTechSQL(ctx context.Context, title string, rw http.ResponseWriter, rep *pgxpool.Pool) (err error) {
	products := []Int{}

	request, err := rep.Query(ctx, `
			SELECT ads.title, ads.id FROM ads.ads WHERE ads.title ILIKE '%' || $1 || '%' AND status != false;
		`,
		title,
	)

	if err != nil {
		fmt.Println(err)
	}

	for request.Next() {
		p := Int{}
		err := request.Scan(
			&p.Title,
			&p.Ads_id,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
		products = append(products, p)
	}

	type Response struct {
		Status  string `json:"status"`
		Data    []Int  `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err != nil || len(products) <= 0 {
		response := Response{
			Status:  "fatal",
			Message: "Объявление не найдено",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return err
	} else {
		response := Response{
			Status:  "success",
			Data:    products,
			Message: "Объявление показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return err
	}
}

func (repo *MyRepository) SortProductListCategoriezSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, category []int) (err error) {
	type Product struct {
		Id          *int
		File_path   string
		Photo       string
		Hourly_rate *int
		Daily_rate  *int
		Title       *string
		Description *string
		Category_id *int
		Name        interface{}
		Owner_id    *int
		Ads_rating  *int
	}
	products := []Product{}

	if err != nil {
		err = fmt.Errorf("failed to exec data: %w", err)

		return
	}

	request, err := rep.Query(
		ctx,
		`
		SELECT
			t2.Id,
			MIN(COALESCE(t1.File_path, '/home/')) AS File_path,
			MIN(t2.Hourly_rate) AS Hourly_rate,
			MIN(t2.Daily_rate) AS Daily_rate,
			MIN(t2.Title) AS Title,
			MIN(t2.description) AS description,
			MIN(t2.Category_id) AS Category_id,
			COALESCE(MIN(t3.Name), MIN(CAST(comp_user.ind_num_taxp AS text))) AS Name,
			MIN(t2.Owner_id) AS Owner_id,
			MIN(t4.rating) AS rating
		FROM
			ads.ads t2
		LEFT JOIN
			ads.ad_photos t1 ON t2.id = t1.ad_id
		LEFT JOIN
			users.individual_user t3 ON t3.user_id = t2.owner_id
		LEFT JOIN
			users.company_user comp_user ON comp_user.user_id = t2.owner_id
		INNER JOIN
			users.users t4 ON COALESCE(t3.user_id, comp_user.user_id) = t4.id
		WHERE
			t2.status = true
			AND (COALESCE($1, ARRAY[]::int[]) = ARRAY[]::int[]
			OR t2.category_id = ANY($1::int[]))
		GROUP BY t2.id
		ORDER BY t2.id;
		`,

		category,
	) //категория передается как массив
	fmt.Println(err)

	if err != nil {
		err = fmt.Errorf("failed to exec data: %w", err)

		return
	}

	for request.Next() {
		p := Product{}
		err := request.Scan(
			&p.Id,
			&p.File_path,
			&p.Hourly_rate,
			&p.Daily_rate,
			&p.Title,
			&p.Description,
			&p.Category_id,
			&p.Name,
			&p.Owner_id,
			&p.Ads_rating,
		)
		if err != nil {
			fmt.Println(err)

			continue

		}
		p.Photo = ServeSpecificMediaBase64(rw, r, p.File_path)

		products = append(products, p)
	}

	type Response struct {
		Status  string    `json:"status"`
		Data    []Product `json:"data,omitempty"`
		Message string    `json:"message"`
	}

	if err == nil && request != nil {
		response := Response{
			Status:  "success",
			Data:    products,
			Message: "Показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return
	}

	response := Response{
		Status:  "fatal",
		Message: "Не показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) SigDisputInChatSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, chat_id, user_id int) (err error) {
	request, err := rep.Query(
		ctx,
		`
		WITH disp AS (
			SELECT id, winner_id 
			FROM orders.disputes 
			WHERE chat_id = $1
		),
		i AS(
			INSERT INTO orders.disputes (chat_id)
			SELECT $1
			WHERE NOT EXISTS (
				SELECT 1
				FROM disp
				WHERE winner_id IS NULL
			)
			RETURNING id
		),
		j AS (
			UPDATE chat.chats 
			SET have_disput = true
			WHERE id = $1 AND (user_1_id = $2 OR user_2_id = $2) AND have_disput != true RETURNING id, have_disput
		)
		SELECT id FROM i;
		`,

		chat_id,
		user_id,
	)

	errorr(err)

	var id_chat int
	for request.Next() {
		err := request.Scan(
			&id_chat,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err != nil || id_chat <= 0 {
		response := Response{
			Status:  "fatal",
			Data:    0,
			Message: "Ошибка, спор по этому чату уже идёт",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	}

	response := Response{
		Status:  "success",
		Data:    id_chat,
		Message: "Спор добавлен",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)
	return
}

func (repo *MyRepository) SigReviewSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, rdb *redis.Client, сonnection map[int]*websocket.Conn, user_id, ads_id, rating int, comment string, state int) (err error) {
	request, err := rep.Query(
		ctx,
		`
		WITH wallet AS (
			SELECT id, user_id FROM finance.wallets WHERE user_id = $1
		),
		transactionn AS (
			SELECT id FROM finance.transactions WHERE wallet_id = (SELECT id FROM wallet)
		),
		booking AS (
			SELECT order_id FROM orders.bookings 
			WHERE ads_id = $2 AND 
				transaction_id IN (SELECT id FROM transactionn)
			LIMIT 1
		),
		reviews AS (
			SELECT reviews.id
			FROM ads.reviews
			JOIN
				orders.bookings
				ON bookings.ads_id = $2
			JOIN
				finance.transactions
				ON transactions.id = bookings.transaction_id
			JOIN
				finance.wallets
				ON wallets.id = transactions.wallet_id AND user_id = $1
			WHERE reviews.order_id = bookings.order_id
		),
		buddy AS (
			SELECT owner_id
			FROM ads.ads
			WHERE ads.id = $2
		)
		INSERT INTO ads.reviews(order_id, rating, comment, state)
		SELECT (SELECT order_id FROM booking), $3, $4, $5
		WHERE $1 = (SELECT user_id FROM wallet) AND $5 >= 1 AND $5 <= 4 AND (SELECT id FROM reviews) IS NULL
		RETURNING id, (SELECT order_id FROM booking), (SELECT owner_id FROM buddy), reviews.created_at;
		`,

		user_id,
		ads_id,
		rating,
		comment,
		state)
	errorr(err)

	var order_id int
	var buddy_id int
	var created_at time.Time

	var rev_id int
	for request.Next() {
		err := request.Scan(
			&rev_id,
			&order_id,
			&buddy_id,
			&created_at,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err != nil || rev_id <= 0 {
		response := Response{
			Status:  "fatal",
			Message: "Ошибка",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	}
	response := Response{
		Status:  "success",
		Data:    rev_id,
		Message: "Отзыв добавлен",
	}

	//достаём аву и имя
	request, err = rep.Query(
		ctx,
		`
		SELECT
			users.avatar_path,
			COALESCE(individual_user.name, company_user.name_of_company),
			users.user_role
		FROM users.users
		LEFT JOIN users.individual_user ON individual_user.user_id = users.id
		LEFT JOIN users.company_user ON company_user.user_id = users.id
		WHERE users.id = $1
		`,

		user_id,
	)
	errorr(err)

	var user_role int
	var avatar_path string
	var name string

	for request.Next() {
		err := request.Scan(
			&avatar_path,
			&name,
			&user_role,
		)
		if err != nil {
			fmt.Println(err)

			continue
		}
	}

	Reviews_Notification(ctx, buddy_id, сonnection[buddy_id], "reg_review", rev_id, "Вам оставили отзыв", created_at, user_id, user_role, ServeSpecificMediaBase64(rw, r, avatar_path), name, rdb)

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)
	return
}

func (repo *MyRepository) UpdReviewSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, reviewer_id, review_id, rating int, comment string) (err error) {
	request, err := rep.Query(
		ctx,
		`
		UPDATE ads.reviews 
		SET rating = $1, comment = $2, updated_at = NOW() 
		WHERE reviewer_id = $3 AND id = $4 RETURNING reviews.id;
		`,

		rating,
		comment,
		reviewer_id,
		review_id,
	)

	errorr(err)

	var reviews_id int
	for request.Next() {
		err := request.Scan(
			&reviews_id,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err != nil || reviews_id <= 0 {
		response := Response{
			Status:  "fatal",
			Data:    0,
			Message: "Ошибка",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	}

	response := Response{
		Status:  "success",
		Data:    reviews_id,
		Message: "Отзыв обновлён",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)
	return
}

// func (repo *MyRepository) UpdReviewSQL(ctx context.Context, rw http.ResponseWriter, reviewer_id int, review_id int, rating int, comment string, rep *pgxpool.Pool) (err error) {
// 	request, err := rep.Query(
// 		ctx,
// 		`
// 			UPDATE ads.reviews
// 			SET rating = $1, comment = $2, updated_at = NOW()
// 			WHERE reviewer_id = $3 AND id = $4 RETURNING reviews.id;
// 		`,

// 		rating,
// 		comment,
// 		reviewer_id,
// 		review_id,
// 	)

// 	errorr(err)

// 	var reviews_id int
// 	for request.Next() {
// 		err := request.Scan(
// 			&reviews_id,
// 		)
// 		if err != nil {
// 			fmt.Println(err)
// 			continue
// 		}
// 	}

// 	type Response struct {
// 		Status  string `json:"status"`
// 		Data    int    `json:"data,omitempty"`
// 		Message string `json:"message"`
// 	}

// 	if err != nil || reviews_id <= 0 {
// 		response := Response{
// 			Status:  "fatal",
// 			Data:    0,
// 			Message: "Ошибка",
// 		}

// 		rw.WriteHeader(http.StatusOK)
// 		json.NewEncoder(rw).Encode(response)
// 		return err
// 	}

// 	response := Response{
// 		Status:  "success",
// 		Data:    reviews_id,
// 		Message: "Отзыв обновлён",
// 	}

// 	rw.WriteHeader(http.StatusOK)
// 	json.NewEncoder(rw).Encode(response)
// 	return
// }

type DisputeChat struct {
	Review_id int    `json:"Review_id"`
	Rating    int    `json:"Rating"`
	Comment   string `json:"Comment"`
}

func (repo *MyRepository) MediatorEnterInChatSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, id_chat, user_id int) (err error) {
	type Product_user struct {
		Message_id int
		User_id    int
		User_role  int
		Name       string
		Text       string
		Media      []string
		Date       time.Time
		Media_pwd  []string
	}
	Products_user_mass := []Product_user{}

	type Response struct {
		Status       string         `json:"status"`
		Ads_id       int            `json:"ads_id"`
		Disput_state bool           `json:"disput_state"`
		Mediator_id  int            `json:"moderator_id"`
		Slave_id     int            `json:"slave_id"`
		Owner_id     int            `json:"owner_id"`
		Global_rate  int            `json:"global_rate"`
		Data         []Product_user `json:"data,omitempty"`
		Message      string         `json:"message"`
	}

	var chat_id_flag int

	request_upd_chat, erro := rep.Query( //это запрос на вывод сообщений нашего кента
		ctx,
		`
		UPDATE chat.chats
		SET mediator_id = $1
		WHERE id = $2 AND (mediator_id IS NULL OR mediator_id = $1)
		RETURNING id;
		`,

		user_id,
		id_chat,
	)
	errorr(erro)

	for request_upd_chat.Next() {
		err := request_upd_chat.Scan(
			&chat_id_flag,
		)

		if err != nil {
			fmt.Println(err)

			continue
		}
	}

	if chat_id_flag == 0 {
		response := Response{
			Status:  "fatal",
			Message: "Не показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return err
	}

	var Message_idd int
	var User_iddd int
	var User_rolee int
	var Namee string
	var Text string
	var Date time.Time
	var Media_pwd []string

	request_2, err := rep.Query( //это запрос на вывод сообщений нашего кента
		ctx,
		`
		WITH i AS (
			SELECT id, text, sent_at, sender_id
			FROM chat.messages 
			WHERE chat_id = $1
		),
		j AS (
			SELECT message_id, path_to_file 
			FROM chat.attachments 
			WHERE message_id IN (SELECT id FROM i)
		),
		company_user AS (
			SELECT user_id, name_of_company 
			FROM users.company_user 
			WHERE user_id = (SELECT sender_id FROM i LIMIT 1)
		),
		individual_user AS (
			SELECT user_id, name 
			FROM users.individual_user 
			WHERE user_id = (SELECT sender_id FROM i LIMIT 1)
		),
		chat_info AS (
			SELECT user_1_id, user_2_id, ad_id, have_disput, mediator_id FROM  chat.chats WHERE chats.id = $1
		),
		ownerr AS (
			SELECT owner_id FROM ads.ads WHERE id = (SELECT ad_id FROM chat_info)
		),
		global_rate_info AS (
			WITH i AS (
				SELECT ad_id FROM chat.chats WHERE chats.id = $1
			)
			SELECT global_rate FROM finance.bidding WHERE bidding.ads_id = (SELECT ad_id FROM i) AND renter_id = $2 AND end_at > NOW()
			ORDER BY id desc
			LIMIT 1
		)
		SELECT
			(SELECT ad_id FROM chat_info) AS ads_id,
			(SELECT have_disput FROM chat_info) AS disput_state,
			COALESCE((SELECT mediator_id FROM chat_info), 0) AS mediator_id,
			COALESCE((SELECT user_1_id FROM chat_info WHERE user_1_id != (SELECT owner_id FROM ownerr)),
				(SELECT user_2_id FROM chat_info WHERE user_2_id != (SELECT owner_id FROM ownerr))) AS slsve_id,
			(SELECT owner_id FROM ownerr),
			COALESCE((SELECT global_rate FROM global_rate_info), 0),

			i.id AS message_id,
			i.sender_id AS user_id,
			COALESCE(individual_user.name, 'company_user.name_of_company') AS name,
			i.text,
			i.sent_at,
			j.path_to_file,
			users.user_role
		FROM i
		JOIN users.users ON users.id = i.sender_id
		LEFT JOIN j ON j.message_id = i.id
		LEFT JOIN company_user ON company_user.user_id = i.sender_id
		LEFT JOIN individual_user ON individual_user.user_id = i.sender_id
		ORDER BY i.id desc;
		`,

		id_chat,
		user_id,
	)
	errorr(err)

	var ads_id int
	var disput_state bool
	var mediator_id int
	var slave_id int
	var owner_id int
	var global_rate int

	for request_2.Next() {
		err := request_2.Scan(
			&ads_id,
			&disput_state,
			&mediator_id,
			&slave_id,
			&owner_id,
			&global_rate,

			&Message_idd,
			&User_iddd,
			&Namee,
			&Text,
			&Date,
			&Media_pwd,
			&User_rolee,
		)
		if err != nil {
			fmt.Println(err)

			continue
		}

		Products_user_mass = append(Products_user_mass, Product_user{Message_id: Message_idd, User_id: User_iddd, User_role: User_rolee, Name: Namee, Text: Text, Date: Date, Media_pwd: Media_pwd})
	}

	if err == nil && (Products_user_mass != nil) {
		for i := 0; i < len(Products_user_mass); i++ {
			for j := 0; j < len(Products_user_mass[i].Media_pwd); j++ {
				media, err := DownloadFile(Products_user_mass[i].Media_pwd[j])
				Products_user_mass[i].Media_pwd[j] = media

				errorr(err)
			}
		}

		response := Response{
			Status:       "success",
			Ads_id:       ads_id,
			Disput_state: disput_state,
			Mediator_id:  mediator_id,
			Slave_id:     slave_id,
			Owner_id:     owner_id,
			Global_rate:  global_rate,
			Data:         Products_user_mass,
			Message:      "Показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return
	}

	response := Response{
		Status:  "fatal",
		Message: "Не показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) RebookListSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, ads_id, user_id int) (err error) {
	request, err := rep.Query( //это запрос на вывод наших сообщений
		ctx,
		`
		WITH wallet AS (
			SELECT id FROM finance.wallets WHERE user_id = $1
		),
		transact AS (
			SELECT id FROM finance.transactions WHERE wallet_id = (SELECT id FROM wallet)
		),
		owner AS (
			SELECT order_id FROM orders.bookings WHERE ads_id = $2 AND transaction_id IN (SELECT id FROM transact) ORDER BY id DESC LIMIT 1
		)
		SELECT id FROM orders.bookings WHERE order_id = (SELECT order_id FROM owner) AND typee = 2
		`,

		user_id,
		ads_id,
	)
	errorr(err)

	var bookIdList []int
	var bookId int

	for request.Next() {
		err := request.Scan(
			&bookId,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}

		bookIdList = append(bookIdList, bookId)
	}

	type Response struct {
		Status  string `json:"status"`
		Data    []int  `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err == nil && len(bookIdList) != 0 {
		response := Response{
			Status:  "success",
			Data:    bookIdList,
			Message: "Показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return
	}

	response := Response{
		Status:  "fatal",
		Message: "Не показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) MediatorFinishJobInChatSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, chat_id int) (err error) {
	request, err := rep.Query( //это запрос на вывод наших сообщений
		ctx,
		`
		UPDATE chat.chats SET mediator_id = null, have_disput = false
		WHERE id = $1
		RETURNING id;`,

		chat_id,
	)
	errorr(err)

	var id_chat int

	for request.Next() {
		err := request.Scan(
			&id_chat,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err == nil && id_chat != 0 {
		response := Response{
			Status:  "success",
			Data:    id_chat,
			Message: "Показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return
	}

	response := Response{
		Status:  "fatal",
		Message: "Не показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) MediatorFinishJobUserSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, logger zerolog.Logger, chat_id int, amount int, comment string, user_id int) (err error) {
	op := "internal.database.adsDB.MediatorFinishJobUserSQL"

	request, err := rep.Query( //это запрос на вывод наших сообщений
		ctx,
		`
		WITH dispute AS (
			SELECT id
			FROM orders.disputes 
			WHERE chat_id = $1 AND winner_id IS NULL
		),
		adss AS (
			SELECT ad_id 
			FROM chat.chats 
			WHERE id = $1
		),
		owner AS (
			SELECT owner_id 
			FROM ads.ads
			WHERE id = (SELECT ad_id FROM adss)
		),
		user_1 AS (
			SELECT user_1_id 
			FROM chat.chats 
			WHERE id = $1 AND user_1_id != (SELECT owner_id FROM owner)
		),
		user_2 AS (
			SELECT user_2_id 
			FROM chat.chats 
			WHERE id = $1 AND user_2_id != (SELECT owner_id FROM owner)
		),
		user_combined AS (
			SELECT user_1_id AS user_id FROM user_1
			UNION
			SELECT user_2_id AS user_id FROM user_2
		),
		i AS (
			UPDATE chat.chats
			SET have_disput = false,
				mediator_id = null
			WHERE id = $1 
				AND (user_1_id = (SELECT COALESCE(MAX(user_id), 0) AS user_id FROM user_combined)
					OR user_2_id = (SELECT COALESCE(MAX(user_id), 0) AS user_id FROM user_combined))
			RETURNING (SELECT COALESCE(MAX(user_id), 0) AS user_id FROM user_combined) AS user_id
		),
		wallet AS (
			SELECT id
			FROM finance.wallets
			WHERE user_id = (SELECT COALESCE(MAX(user_id), 0) AS user_id FROM user_combined)
		),
		book_transaction AS (
			SELECT id
			FROM finance.transactions
			WHERE wallet_id IN (SELECT id FROM wallet)
		),
		amount_transact AS (
			SELECT transaction_id
			FROM orders.bookings
			WHERE ads_id = (SELECT ad_id FROM adss)
				AND ends_at >= NOW()
				AND transaction_id IN (SELECT id FROM book_transaction)
			ORDER BY id DESC
			LIMIT 1
		),
		amountt AS (
			SELECT 
				id,
				amount
			FROM finance.transactions
			WHERE id IN (SELECT transaction_id FROM amount_transact)
		),
		wall_upd AS (
			UPDATE finance.wallets
			SET total_balance = total_balance + $2,
				frozen_funds = frozen_funds - $2
			WHERE user_id = (SELECT COALESCE(MAX(user_id), 0) AS user_id FROM user_combined)
				AND $2 <= frozen_funds AND $2 <= (SElECT amount FROM amountt)
			RETURNING (SELECT COALESCE(MAX(user_id), 0) AS user_id FROM user_combined) AS user_id, id, frozen_funds
		),
		transact AS (
			INSERT INTO finance.transactions (wallet_id, amount, typee, user_2)
			SELECT (SELECT id FROM wall_upd),
				(SELECT frozen_funds FROM wall_upd),
				3,
				$4
			RETURNING id, amount
		)
		UPDATE orders.disputes
		SET winner_id = (SELECT user_id FROM i),
			date_of_win = NOW(),
			comments = $3
		WHERE chat_id = $1 AND winner_id IS NULL 
		RETURNING id;
		`,

		chat_id,
		amount,
		comment,
		user_id,
	)
	fmt.Println(err)
	if err != nil {
		logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с обработкой SQL запроса", op))

		return
	}

	var id_chat int

	for request.Next() {
		err := request.Scan(
			&id_chat,
		)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с обработкой извлечением данных из SQL запроса", op))

			continue
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if id_chat != 0 {
		response := Response{
			Status:  "success",
			Data:    id_chat,
			Message: "Показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return
	}

	response := Response{
		Status:  "fatal",
		Message: "Не показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) MediatorFinishJobOwnerSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, logger zerolog.Logger, chat_id int, comment string) (err error) {
	op := "internal.database.adsDB.MediatorFinishJobOwnerSQL"

	request, err := rep.Query( //это запрос на вывод наших сообщений
		ctx,
		`
		WITH dispute AS (
			SELECT id 
			FROM orders.disputes 
			WHERE chat_id = $1 AND winner_id IS NULL
		),
		adss AS (
			SELECT ad_id 
			FROM chat.chats 
			WHERE id = $1
		),
		owner AS (
			SELECT owner_id 
			FROM ads.ads 
			WHERE id = (SELECT ad_id FROM adss)
		),
		i AS (
			UPDATE chat.chats
			SET have_disput = false,
				mediator_id = null
			WHERE id = $1 
				AND (user_1_id = (SELECT owner_id FROM owner) OR user_2_id = (SELECT owner_id FROM owner)) 
			RETURNING (SELECT owner_id FROM owner) AS user_id
		)
		UPDATE orders.disputes 
		SET winner_id = (SELECT user_id FROM i),
			date_of_win = NOW(),
			comments = $2
		WHERE chat_id = $1 AND winner_id IS NULL 
		RETURNING id;
		`,

		chat_id,
		comment,
	)
	if err != nil {
		logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с обработкой SQL запроса", op))

		return
	}

	var disput_id int

	for request.Next() {
		err := request.Scan(
			&disput_id,
		)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с обработкой извлечением данных из SQL запроса", op))

			continue
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err == nil && disput_id != 0 {
		response := Response{
			Status:  "success",
			Data:    disput_id,
			Message: "Показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return
	}

	response := Response{
		Status:  "fatal",
		Message: "Не показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) SigFavAdsSQL(ctx context.Context, rep *pgxpool.Pool, rw http.ResponseWriter, r *http.Request, rdb *redis.Client, сonnection map[int]*websocket.Conn, user_id int, ads_id int) (err error) {
	request, err := rep.Query(
		ctx,
		`
		WITH fav_ads AS (
			INSERT INTO Ads.favorite_ads(user_id, ad_id) VALUES ($1, $2) RETURNING ad_id, reg_at
		)
		SELECT
			(SELECT ad_id FROM fav_ads),
			(SELECT reg_at FROM fav_ads),
			ads.owner_id
		FROM ads.ads
		WHERE ads.id = (SELECT ad_id FROM fav_ads);
		`,

		user_id,
		ads_id,
	)

	var ad_id int
	var reg_at time.Time
	var buddy_id int
	for request.Next() {
		err := request.Scan(
			&ad_id,
			&reg_at,
			&buddy_id,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err != nil || ad_id == 0 {
		err = fmt.Errorf("failed to exec data: %w", err)
		response := Response{
			Status:  "fatal",
			Message: "Объявление не добавлено",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	}
	response := Response{
		Status:  "success",
		Data:    ad_id,
		Message: "Объявление добавлено",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	//достаём аву и имя
	request, err = rep.Query(
		ctx,
		`
		SELECT
			users.avatar_path,
			COALESCE(individual_user.name, company_user.name_of_company),
			users.user_role
		FROM users.users
		LEFT JOIN users.individual_user ON individual_user.user_id = users.id
		LEFT JOIN users.company_user ON company_user.user_id = users.id
		WHERE users.id = $1
		`,

		user_id,
	)
	errorr(err)

	var user_role int
	var avatar_path string
	var name string

	for request.Next() {
		err := request.Scan(
			&avatar_path,
			&name,
			&user_role,
		)
		if err != nil {
			fmt.Println(err)

			continue
		}
	}

	Reviews_Notification(ctx, buddy_id, сonnection[buddy_id], "reg_fav_ads", ad_id, "Ваше объявление добавили в избранное", reg_at, user_id, user_role, ServeSpecificMediaBase64(rw, r, avatar_path), name, rdb)

	return
}

func (repo *MyRepository) DelFavAdsSQL(ctx context.Context, rep *pgxpool.Pool, rw http.ResponseWriter, user_id, ads_id int) (err error) {
	request, err := rep.Query(
		ctx,
		"DELETE FROM Ads.favorite_ads WHERE user_id = $1 AND ad_id = $2 RETURNING ad_id;",

		user_id,
		ads_id,
	)

	var ad_id int
	for request.Next() {
		err := request.Scan(
			&ad_id,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err != nil || ad_id == 0 {
		err = fmt.Errorf("failed to exec data: %w", err)
		response := Response{
			Status:  "fatal",
			Message: "Объявление не удалено",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	} else {
		response := Response{
			Status:  "success",
			Data:    ad_id,
			Message: "Объявление удалено",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
	}

	return
}

func (repo *MyRepository) GroupAdsByHourlyRateSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool) (err error) {
	request, err := rep.Query(
		ctx,
		"SELECT id, hourly_rate FROM Ads.ads GROUP BY id ORDER BY hourly_rate DESC;",
	)

	type Product struct {
		Id          int
		Hourly_rate int
	}
	products := []Product{}

	for request.Next() {
		p := Product{}
		err := request.Scan(
			&p.Id,
			&p.Hourly_rate)
		if err != nil {
			fmt.Println(err)
			continue
		}
		products = append(products, p)
	}

	type Response struct {
		Status  string    `json:"status"`
		Data    []Product `json:"data,omitempty"`
		Message string    `json:"message"`
	}

	if err != nil {
		response := Response{
			Status:  "fatal",
			Message: "Не сгруппировано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	} else {
		response := Response{
			Status:  "success",
			Data:    products,
			Message: "Сгруппировано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
	}
	return
}

func (repo *MyRepository) GroupAdsByDailyRateSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool) (err error) {
	request, err := rep.Query(
		ctx,
		"SELECT id, daily_rate FROM Ads.ads GROUP BY id ORDER BY daily_rate DESC;",
	)

	errorr(err)

	type Product struct {
		Id          int
		Hourly_rate int
	}
	products := []Product{}

	for request.Next() {
		p := Product{}
		err := request.Scan(
			&p.Id,
			&p.Hourly_rate)
		if err != nil {
			fmt.Println(err)
			continue
		}
		products = append(products, p)
	}

	type Response struct {
		Status  string    `json:"status"`
		Data    []Product `json:"data,omitempty"`
		Message string    `json:"message"`
	}

	if err != nil {
		response := Response{
			Status:  "fatal",
			Message: "Не сгруппировано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	} else {
		response := Response{
			Status:  "success",
			Data:    products,
			Message: "Сгруппировано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return
	}
}

func (repo *MyRepository) GroupFavByRecentSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, user_id int) (err error) {
	type Product struct {
		Ads_path    string //это фотки объявления
		Avatar_path string //это аватарка юзера

		Ads_photo string
		Avatar    string

		Title         string
		Hourly_rate   int
		Daily_rate    int
		Description   string
		Duration      string
		Created_at    time.Time
		Favorite_flag bool
		User_name     string
		Rating        float64
		Review_count  int

		Ads_id      int
		Owner_id    int
		Category_id int
	}

	var Duration_mass []string
	var Favorite_flag_mass []bool
	var Review_count_mass []int

	products := []Product{}
	request, err := rep.Query(
		ctx,
		`
		WITH duration AS (
			SELECT
				ARRAY_AGG(ads.id) AS ad_ids, -- Собираем все id в массив
				ARRAY_AGG(ARRAY[bookings.starts_at, bookings.ends_at]) AS date_range -- Собираем массив пар дат
			FROM ads.ads
			LEFT JOIN orders.bookings 
				ON bookings.ads_id = ads.id
			LEFT JOIN orders.orders
				ON orders.id = bookings.order_id
			INNER JOIN ads.favorite_ads 
				ON ads.id = favorite_ads.ad_id AND favorite_ads.user_id = $1
		)
		SELECT
			COALESCE(t1.File_path::TEXT, '/root/'),
			t2.Title::TEXT,
			t2.Hourly_rate,
			t2.Daily_rate,
			t2.Description::TEXT,
			Duration(
				(SELECT d.date_range::date[] FROM duration d WHERE t2.id = ANY(d.ad_ids))
			) AS duration_result, -- Функция принимает массив
			t2.Created_at,
			Favorite_flag($1, (SELECT ad_ids FROM duration)::INT[]),
			t4.Avatar_path::TEXT as User_avatar,
			COALESCE(t3.Name::TEXT, t5.name_of_company::TEXT) as User_name,
			t4.Rating,
			Review_count((SELECT ad_ids FROM duration)::INT[]),
			t2.Id as Ads_id,
			t2.Owner_id,
			t2.Category_id
		FROM
			ads.ads t2
		LEFT JOIN
			ads.ad_photos t1
			ON t2.id = t1.ad_id  -- Соединение на уровне объявления
		LEFT JOIN
			users.individual_user t3
			ON t3.user_id = t2.owner_id
		LEFT JOIN
			users.company_user t5
			ON t5.user_id = t2.owner_id
		LEFT JOIN
			users.users t4
			ON t4.id = t2.owner_id
		WHERE
			t2.status = true
			AND t2.id = ANY((SELECT ad_ids FROM duration)::INT[])
		GROUP BY 
			COALESCE(t1.File_path::TEXT, '/root/'),
			t2.Title::TEXT,
			t2.Hourly_rate,
			t2.Description::TEXT,
			duration_result, -- Функция принимает массив
			t2.Created_at,
			Favorite_flag($1, (SELECT ad_ids FROM duration)::INT[]),
			User_avatar,
			User_name,
			t4.Rating,
			Review_count((SELECT ad_ids FROM duration)::INT[]),
			Ads_id,
			t2.Owner_id,
			t2.Category_id
		ORDER BY t2.Created_at desc
		`,

		user_id)
	errorr(err)

	for request.Next() {
		p := Product{}
		err := request.Scan(
			&p.Ads_path, //кладем сюда множество, длиною в три ӕлемента, с путями фоток
			&p.Title,
			&p.Hourly_rate,
			&p.Daily_rate,
			&p.Description,
			&Duration_mass,
			&p.Created_at,
			&Favorite_flag_mass,
			&p.Avatar_path,
			&p.User_name,
			&p.Rating,
			&Review_count_mass,

			&p.Ads_id,
			&p.Owner_id,
			&p.Category_id,
		)
		if err != nil {
			fmt.Errorf("Error", err)
			continue
		}
		products = append(products, p)
	}

	for i := 0; i < len(products); i++ { //пока что у нас три объявления
		// products[i].Duration = Duration_mass[i]
		products[i].Favorite_flag = Favorite_flag_mass[i]
		products[i].Review_count = Review_count_mass[i]

		for j := 0; j < len(products[i].Ads_path); j++ {
			products[i].Ads_photo = ServeSpecificMediaBase64(rw, r, products[i].Ads_path)
		}

		products[i].Avatar = ServeSpecificMediaBase64(rw, r, products[i].Avatar_path)
	}

	type Response struct {
		Status  string    `json:"status"`
		Data    []Product `json:"data,omitempty"`
		Message string    `json:"message"`
	}

	if err != nil || len(products) == 0 {
		response := Response{
			Status:  "fatal",
			Message: "Не показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return err
	} else if len(products) == 0 {
		response := Response{
			Status:  "success",
			Data:    []Product{{}},
			Message: "Показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return
	}

	response := Response{
		Status:  "success",
		Data:    products,
		Message: "Показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)
	return
}

func (repo *MyRepository) GroupFavByCheaperSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, user_id int) (err error) {
	type Product struct {
		Ads_path    string //это фотки объявления
		Avatar_path string //это аватарка юзера

		Ads_photo string
		Avatar    string

		Title         string
		Hourly_rate   int
		Daily_rate    int
		Description   string
		Duration      string
		Created_at    time.Time
		Favorite_flag bool
		User_name     string
		Rating        float64
		Review_count  int

		Ads_id      int
		Owner_id    int
		Category_id int
	}

	var Duration_mass []string
	var Favorite_flag_mass []bool
	var Review_count_mass []int

	products := []Product{}
	request, err := rep.Query(
		ctx,
		`
		WITH duration AS (
			SELECT
				ARRAY_AGG(ads.id) AS ad_ids, -- Собираем все id в массив
				ARRAY_AGG(ARRAY[bookings.starts_at, bookings.ends_at]) AS date_range -- Собираем массив пар дат
			FROM ads.ads
			LEFT JOIN orders.bookings 
				ON bookings.ads_id = ads.id
			LEFT JOIN orders.orders
				ON orders.id = bookings.order_id
			INNER JOIN ads.favorite_ads 
				ON ads.id = favorite_ads.ad_id AND favorite_ads.user_id = $1
		)
		SELECT
			COALESCE(t1.File_path::TEXT, '/root/'),
			t2.Title::TEXT,
			t2.Hourly_rate,
			t2.Daily_rate,
			t2.Description::TEXT,
			Duration(
				(SELECT d.date_range::date[] FROM duration d WHERE t2.id = ANY(d.ad_ids))
			) AS duration_result, -- Функция принимает массив
			t2.Created_at,
			Favorite_flag($1, (SELECT ad_ids FROM duration)::INT[]),
			t4.Avatar_path::TEXT as User_avatar,
			COALESCE(t3.Name::TEXT, t5.name_of_company::TEXT) as User_name,
			t4.Rating,
			Review_count((SELECT ad_ids FROM duration)::INT[]),
			t2.Id as Ads_id,
			t2.Owner_id,
			t2.Category_id
		FROM
			ads.ads t2
		LEFT JOIN
			ads.ad_photos t1
			ON t2.id = t1.ad_id  -- Соединение на уровне объявления
		LEFT JOIN
			users.individual_user t3
			ON t3.user_id = t2.owner_id
		LEFT JOIN
			users.company_user t5
			ON t5.user_id = t2.owner_id
		LEFT JOIN
			users.users t4
			ON t4.id = t2.owner_id
		WHERE
			t2.status = true
			AND t2.id = ANY((SELECT ad_ids FROM duration)::INT[])
		GROUP BY 
			COALESCE(t1.File_path::TEXT, '/root/'),
			t2.Title::TEXT,
			t2.Hourly_rate,
			t2.Daily_rate,
			t2.Description::TEXT,
			duration_result, -- Функция принимает массив
			t2.Created_at,
			Favorite_flag($1, (SELECT ad_ids FROM duration)::INT[]),
			User_avatar,
			User_name,
			t4.Rating,
			Review_count((SELECT ad_ids FROM duration)::INT[]),
			Ads_id,
			t2.Owner_id,
			t2.Category_id
		ORDER BY (t2.Hourly_rate + t2.Daily_rate) DESC;
		`,
		user_id)
	errorr(err)

	for request.Next() {
		p := Product{}
		err := request.Scan(
			&p.Ads_path, //кладем сюда множество, длиною в три ӕлемента, с путями фоток
			&p.Title,
			&p.Hourly_rate,
			&p.Daily_rate,
			&p.Description,
			&Duration_mass,
			&p.Created_at,
			&Favorite_flag_mass,
			&p.Avatar_path,
			&p.User_name,
			&p.Rating,
			&Review_count_mass,

			&p.Ads_id,
			&p.Owner_id,
			&p.Category_id,
		)
		if err != nil {
			fmt.Errorf("Error", err)
			continue
		}
		products = append(products, p)
	}

	for i := 0; i < len(products); i++ { //пока что у нас три объявления
		// products[i].Duration = Duration_mass[i]
		products[i].Favorite_flag = Favorite_flag_mass[i]
		products[i].Review_count = Review_count_mass[i]

		for j := 0; j < len(products[i].Ads_path); j++ {
			products[i].Ads_photo = ServeSpecificMediaBase64(rw, r, products[i].Ads_path)
		}

		products[i].Avatar = ServeSpecificMediaBase64(rw, r, products[i].Avatar_path)
	}

	type Response struct {
		Status  string    `json:"status"`
		Data    []Product `json:"data,omitempty"`
		Message string    `json:"message"`
	}

	if err != nil {
		response := Response{
			Status:  "fatal",
			Message: "Не показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return err
	} else if len(products) == 0 {
		response := Response{
			Status:  "success",
			Data:    []Product{{}},
			Message: "Показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return
	}

	response := Response{
		Status:  "success",
		Data:    products,
		Message: "Показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)
	return
}

func (repo *MyRepository) GroupFavByDearlySQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, user_id int) (err error) {
	type Product struct {
		Ads_path    string //это фотки объявления
		Avatar_path string //это аватарка юзера

		Ads_photo string
		Avatar    string

		Title         string
		Hourly_rate   int
		Daily_rate    int
		Description   string
		Duration      string
		Created_at    time.Time
		Favorite_flag bool
		User_name     string
		Rating        float64
		Review_count  int

		Ads_id      int
		Owner_id    int
		Category_id int
	}

	var Duration_mass []string
	var Favorite_flag_mass []bool
	var Review_count_mass []int

	products := []Product{}
	request, err := rep.Query(
		ctx,
		`
		WITH duration AS (
			SELECT
				ARRAY_AGG(ads.id) AS ad_ids, -- Собираем все id в массив
				ARRAY_AGG(ARRAY[bookings.starts_at, bookings.ends_at]) AS date_range -- Собираем массив пар дат
			FROM ads.ads
			LEFT JOIN orders.bookings 
				ON bookings.ads_id = ads.id
			LEFT JOIN orders.orders
				ON orders.id = bookings.order_id
			INNER JOIN ads.favorite_ads 
				ON ads.id = favorite_ads.ad_id AND favorite_ads.user_id = $1
		)
		SELECT
			COALESCE(t1.File_path::TEXT, '/root/'),
			t2.Title::TEXT,
			t2.Hourly_rate,
			t2.Daily_rate,
			t2.Description::TEXT,
			Duration(
				(SELECT d.date_range::date[] FROM duration d WHERE t2.id = ANY(d.ad_ids))
			) AS duration_result, -- Функция принимает массив
			t2.Created_at,
			Favorite_flag($1, (SELECT ad_ids FROM duration)::INT[]),
			t4.Avatar_path::TEXT as User_avatar,
			COALESCE(t3.Name::TEXT, t5.name_of_company::TEXT) as User_name,
			t4.Rating,
			Review_count((SELECT ad_ids FROM duration)::INT[]),
			t2.Id as Ads_id,
			t2.Owner_id,
			t2.Category_id
		FROM
			ads.ads t2
		LEFT JOIN
			ads.ad_photos t1
			ON t2.id = t1.ad_id  -- Соединение на уровне объявления
		LEFT JOIN
			users.individual_user t3
			ON t3.user_id = t2.owner_id
		LEFT JOIN
			users.company_user t5
			ON t5.user_id = t2.owner_id
		LEFT JOIN
			users.users t4
			ON t4.id = t2.owner_id
		WHERE
			t2.status = true
			AND t2.id = ANY((SELECT ad_ids FROM duration)::INT[])
		GROUP BY 
			COALESCE(t1.File_path::TEXT, '/root/'),
			t2.Title::TEXT,
			t2.Hourly_rate,
			t2.Daily_rate,
			t2.Description::TEXT,
			duration_result, -- Функция принимает массив
			t2.Created_at,
			Favorite_flag($1, (SELECT ad_ids FROM duration)::INT[]),
			User_avatar,
			User_name,
			t4.Rating,
			Review_count((SELECT ad_ids FROM duration)::INT[]),
			Ads_id,
			t2.Owner_id,
			t2.Category_id
		ORDER BY (t2.Hourly_rate + t2.Daily_rate) ASC;
		`,
		user_id)
	errorr(err)

	for request.Next() {
		p := Product{}
		err := request.Scan(
			&p.Ads_path, //кладем сюда множество, длиною в три ӕлемента, с путями фоток
			&p.Title,
			&p.Hourly_rate,
			&p.Daily_rate,
			&p.Description,
			&Duration_mass,
			&p.Created_at,
			&Favorite_flag_mass,
			&p.Avatar_path,
			&p.User_name,
			&p.Rating,
			&Review_count_mass,

			&p.Ads_id,
			&p.Owner_id,
			&p.Category_id,
		)
		if err != nil {
			fmt.Errorf("Error", err)
			continue
		}
		products = append(products, p)
	}

	for i := 0; i < len(products); i++ { //пока что у нас три объявления
		// products[i].Duration = Duration_mass[i]
		products[i].Favorite_flag = Favorite_flag_mass[i]
		products[i].Review_count = Review_count_mass[i]

		for j := 0; j < len(products[i].Ads_path); j++ {
			products[i].Ads_photo = ServeSpecificMediaBase64(rw, r, products[i].Ads_path)
		}

		products[i].Avatar = ServeSpecificMediaBase64(rw, r, products[i].Avatar_path)
	}

	type Response struct {
		Status  string    `json:"status"`
		Data    []Product `json:"data,omitempty"`
		Message string    `json:"message"`
	}

	if err != nil {
		response := Response{
			Status:  "fatal",
			Message: "Не показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return err
	} else if len(products) == 0 {
		response := Response{
			Status:  "success",
			Data:    []Product{{}},
			Message: "Показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return
	}

	response := Response{
		Status:  "success",
		Data:    products,
		Message: "Показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)
	return
}

func (repo *MyRepository) GroupAdsByRentedSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, user_id int) (err error) {
	type Product struct {
		Id          int
		Title       string
		Description string
		Hourly_rate int
		Daily_rate  int
		Owner_id    int
		Category_id int
		Position    pgtype.Point
		Created_at  time.Time
		Updated_at  time.Time
		Duration    string
	}
	products := []Product{}

	request, err := rep.Query(
		ctx,
		`
		WITH duration AS (
			WITH duration AS (
				SELECT
					ads.id,
					ads.owner_id,
					ARRAY[
						COALESCE(MAX(bookings.starts_at), '1900-01-01'),
						COALESCE(MAX(bookings.ends_at), '1900-01-01')
					] AS date_range
				FROM 
					ads.ads
				LEFT JOIN 
					orders.bookings
				ON 
					bookings.ads_id = ads.id
				GROUP BY 
					ads.id, ads.owner_id
			)
			SELECT * FROM duration
		)
		SELECT 
			id, 
			title, 
			description, 
			hourly_rate, 
			daily_rate, 
			owner_id, 
			category_id, 
			position, 
			created_at, 
			updated_at,
			COALESCE((Duration(
				(SELECT d.date_range::date[] FROM duration d WHERE d.id = ads.id)
			))[1], 'Нет точной информации')
		FROM Ads.ads 
		WHERE 
			status = true AND owner_id = $1;
		`,

		user_id,
	)
	errorr(err)

	for request.Next() {
		p := Product{}
		err := request.Scan(
			&p.Id,
			&p.Title,
			&p.Description,
			&p.Hourly_rate,
			&p.Daily_rate,
			&p.Owner_id,
			&p.Category_id,
			&p.Position,
			&p.Created_at,
			&p.Updated_at,
			&p.Duration,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
		products = append(products, p)
	}

	type Response struct {
		Status  string    `json:"status"`
		Data    []Product `json:"data,omitempty"`
		Message string    `json:"message"`
	}

	if err != nil {
		response := Response{
			Status:  "fatal",
			Message: "Не сгруппировано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	}

	response := Response{
		Status:  "success",
		Data:    products,
		Message: "Сгруппировано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)
	return
}

func (repo *MyRepository) GroupAdsByArchivedSQL(ctx context.Context, rw http.ResponseWriter, user_id int, rep *pgxpool.Pool) (err error) {
	type Product struct {
		Id          int
		Title       string
		Description string
		Hourly_rate int
		Daily_rate  int
		Owner_id    int
		Category_id int
		Position    pgtype.Point
		Created_at  time.Time
		Updated_at  time.Time
		Bool_state  bool
		Duration    string
	}
	products := []Product{}

	request, err := rep.Query(
		ctx,
		`
		WITH duration AS (
			WITH duration AS (
				SELECT
					ads.id,
					ads.owner_id,
					ARRAY[
						COALESCE(MAX(bookings.starts_at), '1900-01-01'),
						COALESCE(MAX(bookings.ends_at), '1900-01-01')
					] AS date_range
				FROM 
					ads.ads
				LEFT JOIN 
					orders.bookings
				ON 
					bookings.ads_id = ads.id
				GROUP BY 
					ads.id, ads.owner_id
			)
			SELECT * FROM duration
		)
		SELECT 
			id, 
			title, 
			description, 
			hourly_rate, 
			daily_rate, 
			owner_id, 
			category_id, 
			position, 
			created_at, 
			updated_at, 
			false,
			COALESCE((Duration(
				(SELECT d.date_range::date[] FROM duration d WHERE d.id = ads.id)
			))[1], 'Нет точной информации')
		FROM Ads.ads
		WHERE status = false AND owner_id = $1;`,

		user_id,
	)
	if err != nil {
		err = fmt.Errorf("failed to exec data: %w", err)
		return
	}

	for request.Next() {
		p := Product{}
		err := request.Scan(
			&p.Id,
			&p.Title,
			&p.Description,
			&p.Hourly_rate,
			&p.Daily_rate,
			&p.Owner_id,
			&p.Category_id,
			&p.Position,
			&p.Created_at,
			&p.Updated_at,
			&p.Bool_state,
			&p.Duration,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
		products = append(products, p)
	}

	type Response struct {
		Status  string    `json:"status"`
		Data    []Product `json:"data,omitempty"`
		Message string    `json:"message"`
	}

	if err != nil {
		response := Response{
			Status:  "fatal",
			Message: "Не сгруппировано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	} else {
		fmt.Println(products)

		response := Response{
			Status:  "success",
			Data:    products,
			Message: "Сгруппировано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return
	}
}
