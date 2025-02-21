package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

func (repo *MyRepository) TransactionToAnotherSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, user_id int, user_2 int, amount int) (err error) {
	your_wallet_id, err := rep.Query(ctx, "SELECT id FROM Finance.wallets WHERE user_id = $1;", user_id)
	errorr(err)

	var your_wallet_id_int int //дата создания репорта
	for your_wallet_id.Next() {
		err := your_wallet_id.Scan(&your_wallet_id_int)
		if err != nil {
			log.Fatal(err)
		}
	}

	user_2_wallet_id, err := rep.Query(ctx, "SELECT id FROM Finance.wallets WHERE user_id = $1;", user_2)
	errorr(err)
	var user_2_wallet_id_int int //дата создания репорта
	for user_2_wallet_id.Next() {
		err := user_2_wallet_id.Scan(&user_2_wallet_id_int)
		if err != nil {
			log.Fatal(err)
		}
	}

	transact_id, err := rep.Query(
		ctx,
		`
		WITH i AS (
			INSERT INTO Finance.transactions (wallet_id, amount, user_2, typee)
			VALUES ($1, $2, $3, $4)
			RETURNING id, amount
		)
		INSERT INTO Finance.transactions (wallet_id, amount, user_2, typee)
		SELECT $5, i.amount, $6, $7
		FROM i
		RETURNING id;
		`,

		your_wallet_id_int,
		amount,
		user_2,
		1,

		user_2_wallet_id_int,
		user_id,
		2,
	)

	var transact_id_int int
	for transact_id.Next() {
		err := transact_id.Scan(&transact_id_int)
		if err != nil {
			log.Fatal(err)
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err != nil || transact_id_int == 0 {
		response := Response{
			Status:  "fatal",
			Message: "Не прошла",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	}

	response := Response{
		Status:  "success",
		Data:    transact_id_int,
		Message: "Транзакция прошла успешно",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)
	return
}

func (repo *MyRepository) TransactionToSomethingSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, user_id int, amount int) (err error) {
	wallet_id, err := rep.Query(ctx, "SELECT id FROM Finance.wallets WHERE user_id = $1;", user_id)
	errorr(err)
	var wallet_id_int int
	for wallet_id.Next() {
		err := wallet_id.Scan(&wallet_id_int)
		if err != nil {
			log.Fatal(err)
		}
	}

	transact_id, err := rep.Query(
		ctx,
		`INSERT INTO Finance.transactions (wallet_id, amount, typee) 
			VALUES ($1, $2, 3) 
			RETURNING id;`,

		wallet_id_int,
		amount,
	)
	errorr(err)

	var transact_id_int int //дата создания репорта
	for transact_id.Next() {
		err := transact_id.Scan(&transact_id_int)
		if err != nil {
			log.Fatal(err)
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err != nil || transact_id_int == 0 {
		response := Response{
			Status:  "fatal",
			Message: "Не прошла",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	}

	response := Response{
		Status:  "success",
		Data:    transact_id_int,
		Message: "Транзакция прошла успешно",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return
}

func (repo *MyRepository) TransactionToReturnAmountSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, user_id int, amount int) (err error) {
	wallet_id, err := rep.Query(ctx, "SELECT id FROM Finance.wallets WHERE user_id = $1;", user_id)
	errorr(err)
	var wallet_id_int int //дата создания репорта
	for wallet_id.Next() {
		err := wallet_id.Scan(&wallet_id_int)
		if err != nil {
			log.Fatal(err)
		}
	}

	transact_id, err := rep.Query(
		ctx,
		`INSERT INTO Finance.transactions (wallet_id, amount, typee)
		 VALUES ($1, $2, 3)
		 RETURNING id;`,

		wallet_id_int,
		amount,
	)
	errorr(err)

	var transact_id_int int //дата создания репорта
	for transact_id.Next() {
		err := transact_id.Scan(&transact_id_int)
		if err != nil {
			log.Fatal(err)
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err != nil || transact_id_int == 0 {
		response := Response{
			Status:  "fatal",
			Message: "Не прошла",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	}
	response := Response{
		Status:  "success",
		Data:    transact_id_int,
		Message: "Транзакция прошла успешно",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return
}

func (repo *MyRepository) WalletHistorySQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, user_id, typee int) (err error) {
	type WallHist struct {
		ID          int
		Avatar_path string
		Avatar      string
		User_name   *string
		Amount      int
		Created_at  time.Time
		Typee       int
	}
	products := []WallHist{}

	if err != nil {
		err = fmt.Errorf("failed to exec data: %w", err)

		return
	}

	request, err := rep.Query(
		ctx,
		`
		WITH wallet AS (
			SELECT id AS wallet_id
			FROM finance.wallets
			WHERE user_id = $1
		),
		transact AS (
			SELECT id, wallet_id, amount, created_at, user_2, typee
			FROM finance.transactions
			WHERE wallet_id = (SELECT wallet_id FROM wallet)
			AND typee = $2
		)
		SELECT
			transact.id,
			COALESCE(users.avatar_path, '/home/'),
			COALESCE(COALESCE(individual_user.name || ' ' || individual_user.patronymic, company_user.name_of_company), 'Нет Имени') AS user_name,
			transact.amount, 
			transact.created_at, 
			transact.typee
		FROM 
			transact
		LEFT JOIN users.users ON users.id = transact.user_2
		LEFT JOIN users.individual_user ON transact.user_2 = individual_user.user_id
		LEFT JOIN users.company_user ON transact.user_2 = company_user.user_id;
		`,

		user_id,
		typee)

	errorr(err)

	for request.Next() {
		p := WallHist{}
		err := request.Scan(
			&p.ID,
			&p.Avatar_path,
			&p.User_name,
			&p.Amount,
			&p.Created_at,
			&p.Typee,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}

		products = append(products, p)
	}

	for i := 0; i < len(products); i++ {
		products[i].Avatar = ServeSpecificMediaBase64(rw, r, products[i].Avatar_path)

		products[i].Avatar_path = ""
	}

	type Response struct {
		Status  string     `json:"status"`
		Data    []WallHist `json:"data,omitempty"`
		Message string     `json:"message"`
	}

	if err != nil || request == nil || len(products) == 0 {
		response := Response{
			Status:  "fatal",
			Message: "Транзакций не найденно",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	}

	response := Response{
		Status:  "success",
		Data:    products,
		Message: "Транзакции успешно найденны",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return
}

func (repo *MyRepository) WalletListSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, user_id int) (err error) {
	type WallHist struct {
		Total_balance int
		Frozen_funds  int
		Avatar_path   string
		Avatar        string
		User_name     *string
	}
	products := []WallHist{}

	if err != nil {
		err = fmt.Errorf("failed to exec data: %w", err)

		return
	}

	request, err := rep.Query(
		ctx,
		`
		WITH wallet AS (
			SELECT id AS wallet_id, total_balance, frozen_funds
			FROM finance.wallets
			WHERE user_id = $1
		)
		SELECT
			wallet.total_balance,
			wallet.frozen_funds,
			users.avatar_path,
			COALESCE(individual_user.name || ' ' || individual_user.patronymic, company_user.name_of_company) AS user_name
		FROM wallet
		LEFT JOIN users.users ON users.id = $1
		LEFT JOIN users.individual_user ON $1 = individual_user.user_id
		LEFT JOIN users.company_user ON $1 = company_user.user_id;
		`,

		user_id)

	errorr(err)

	for request.Next() {
		p := WallHist{}
		err := request.Scan(
			&p.Total_balance,
			&p.Frozen_funds,
			&p.Avatar_path,
			&p.User_name,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}

		products = append(products, p)
	}

	for i := 0; i < len(products); i++ {
		products[i].Avatar = ServeSpecificMediaBase64(rw, r, products[i].Avatar_path)

		products[i].Avatar_path = ""
	}

	type Response struct {
		Status  string     `json:"status"`
		Data    []WallHist `json:"data,omitempty"`
		Message string     `json:"message"`
	}

	if err != nil || request == nil {
		response := Response{
			Status:  "fatal",
			Message: "Не прошла",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	}

	response := Response{
		Status:  "success",
		Data:    products,
		Message: "Транзакция прошла успешно",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return
}

func (repo *MyRepository) FavProfilsFirstNewSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, user_id int) (err error) {
	type FavProfils struct {
		Owner_id    int
		Avatar_path string
		Avatar      string
		User_name   string
	}
	products := []FavProfils{}

	errorr(err)

	request, err := rep.Query(
		ctx,
		`
		SELECT
			ads.owner_id,
			COALESCE(users.Avatar_path::TEXT, '/home/') as User_avatar,
			COALESCE(t3.Name::TEXT, t5.name_of_company::TEXT) as User_name
		FROM
			ads.favorite_ads
		LEFT JOIN
			ads.ads
			ON ads.id = favorite_ads.ad_id
		LEFT JOIN
			users.individual_user t3
			ON t3.user_id = ads.owner_id
		LEFT JOIN
			users.company_user t5
			ON t5.user_id = ads.owner_id
		LEFT JOIN
			users.users
			ON users.id = ads.owner_id
		WHERE
			favorite_ads.user_id = $1
		ORDER BY favorite_ads.reg_at ASC
		`,

		user_id)

	errorr(err)

	for request.Next() {
		p := FavProfils{}
		err := request.Scan(
			&p.Owner_id,
			&p.Avatar_path,
			&p.User_name,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}

		products = append(products, p)
	}

	for i := 0; i < len(products); i++ {
		products[i].Avatar = ServeSpecificMediaBase64(rw, r, products[i].Avatar_path)
	}

	type Response struct {
		Status  string       `json:"status"`
		Data    []FavProfils `json:"data,omitempty"`
		Message string       `json:"message"`
	}

	if err != nil || request == nil {
		response := Response{
			Status:  "fatal",
			Message: "Не прошла",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	}

	response := Response{
		Status:  "success",
		Data:    products,
		Message: "Транзакция прошла успешно",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return
}

func (repo *MyRepository) FavProfilsFirstOldSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, user_id int) (err error) {
	type FavProfils struct {
		Owner_id    int
		Avatar_path string
		Avatar      string
		User_name   string
	}
	products := []FavProfils{}

	errorr(err)

	request, err := rep.Query(
		ctx,
		`
		SELECT
			ads.owner_id,
			COALESCE(users.Avatar_path::TEXT, '/home/') as User_avatar,
			COALESCE(t3.Name::TEXT, t5.name_of_company::TEXT) as User_name
		FROM
			ads.favorite_ads
		LEFT JOIN
			ads.ads
			ON ads.id = favorite_ads.ad_id
		LEFT JOIN
			users.individual_user t3
			ON t3.user_id = ads.owner_id
		LEFT JOIN
			users.company_user t5
			ON t5.user_id = ads.owner_id
		LEFT JOIN
			users.users
			ON users.id = ads.owner_id
		WHERE
			favorite_ads.user_id = $1
		ORDER BY favorite_ads.reg_at DESC
		`,

		user_id)

	errorr(err)

	for request.Next() {
		p := FavProfils{}
		err := request.Scan(
			&p.Owner_id,
			&p.Avatar_path,
			&p.User_name,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}

		products = append(products, p)
	}

	for i := 0; i < len(products); i++ {
		products[i].Avatar = ServeSpecificMediaBase64(rw, r, products[i].Avatar_path)
	}

	type Response struct {
		Status  string       `json:"status"`
		Data    []FavProfils `json:"data,omitempty"`
		Message string       `json:"message"`
	}

	if err != nil || request == nil {
		response := Response{
			Status:  "fatal",
			Message: "Не прошла",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	}

	response := Response{
		Status:  "success",
		Data:    products,
		Message: "Транзакция прошла успешно",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return
}

func (repo *MyRepository) FavProfilsFirstCheapSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, user_id int) (err error) {
	type FavProfils struct {
		Owner_id    int
		Avatar_path string
		Avatar      string
		User_name   string
	}
	products := []FavProfils{}

	errorr(err)

	request, err := rep.Query(
		ctx,
		`
		SELECT
			ads.owner_id,
			COALESCE(users.Avatar_path::TEXT, '/home/') as User_avatar,
			COALESCE(t3.Name::TEXT, t5.name_of_company::TEXT) as User_name
		FROM
			ads.favorite_ads
		LEFT JOIN
			ads.ads
			ON ads.id = favorite_ads.ad_id
		LEFT JOIN
			users.individual_user t3
			ON t3.user_id = ads.owner_id
		LEFT JOIN
			users.company_user t5
			ON t5.user_id = ads.owner_id
		LEFT JOIN
			users.users
			ON users.id = ads.owner_id
		WHERE
			favorite_ads.user_id = $1
		ORDER BY ads.hourly_rate ASC
		`,

		user_id)

	errorr(err)

	for request.Next() {
		p := FavProfils{}
		err := request.Scan(
			&p.Owner_id,
			&p.Avatar_path,
			&p.User_name,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}

		products = append(products, p)
	}

	for i := 0; i < len(products); i++ {
		products[i].Avatar = ServeSpecificMediaBase64(rw, r, products[i].Avatar_path)
	}

	type Response struct {
		Status  string       `json:"status"`
		Data    []FavProfils `json:"data,omitempty"`
		Message string       `json:"message"`
	}

	if err != nil || request == nil {
		response := Response{
			Status:  "fatal",
			Message: "Не прошла",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	}

	response := Response{
		Status:  "success",
		Data:    products,
		Message: "Транзакция прошла успешно",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return
}

func (repo *MyRepository) FavProfilsFirstDearlSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, user_id int) (err error) {
	type FavProfils struct {
		Owner_id    int
		Avatar_path string
		Avatar      string
		User_name   string
	}
	products := []FavProfils{}

	errorr(err)

	request, err := rep.Query(
		ctx,
		`
		SELECT
			ads.owner_id,
			COALESCE(users.Avatar_path::TEXT, '/home/') as User_avatar,
			COALESCE(t3.Name::TEXT, t5.name_of_company::TEXT) as User_name
		FROM
			ads.favorite_ads
		LEFT JOIN
			ads.ads
			ON ads.id = favorite_ads.ad_id
		LEFT JOIN
			users.individual_user t3
			ON t3.user_id = ads.owner_id
		LEFT JOIN
			users.company_user t5
			ON t5.user_id = ads.owner_id
		LEFT JOIN
			users.users
			ON users.id = ads.owner_id
		WHERE
			favorite_ads.user_id = $1
		ORDER BY ads.hourly_rate DESC
		`,

		user_id)

	errorr(err)

	for request.Next() {
		p := FavProfils{}
		err := request.Scan(
			&p.Owner_id,
			&p.Avatar_path,
			&p.User_name,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}

		products = append(products, p)
	}

	for i := 0; i < len(products); i++ {
		products[i].Avatar = ServeSpecificMediaBase64(rw, r, products[i].Avatar_path)
	}

	type Response struct {
		Status  string       `json:"status"`
		Data    []FavProfils `json:"data,omitempty"`
		Message string       `json:"message"`
	}

	if err != nil || request == nil {
		response := Response{
			Status:  "fatal",
			Message: "Не прошла",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	}

	response := Response{
		Status:  "success",
		Data:    products,
		Message: "Транзакция прошла успешно",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return
}

func (repo *MyRepository) OpenUserProfileSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, user_id int) (err error) {
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
		Ads_Rating    float64
		Review_count  int

		Ads_id      int
		Owner_id    int
		Category_id int

		Id                        int
		Login                     string
		Name                      string
		Surname_or_Ind_num        string
		Patronomic_or_Addres_name string
		User_Rating               float32
		Total_balance             int
		User_role                 int
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
		),
		login AS (
			SELECT 
				users.id,
				users.email,
				COALESCE(individual_user.Name::TEXT, company_user.Name_of_company::TEXT) AS Name,
				COALESCE(individual_user.Surname::TEXT, company_user.Ind_num_taxp::TEXT) AS Surname_or_Ind_num,
				COALESCE(individual_user.Patronymic::TEXT, company_user.Address_name::TEXT) AS Patronomic_or_Addres_name,
				users.rating,
				wallets.total_balance,
				users.user_role
			FROM Users.users
				LEFT JOIN Users.individual_user ON users.id = individual_user.user_id
				LEFT JOIN Users.company_user ON users.id = company_user.user_id
				JOIN finance.wallets ON users.id = wallets.user_id
				WHERE users.id = $1
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
			t2.Category_id,
			-- теперь login
			(SELECT id FROM login),
			(SELECT email FROM login),
			(SELECT Name FROM login),
			(SELECT Surname_or_Ind_num FROM login),
			(SELECT Patronomic_or_Addres_name FROM login),
			(SELECT rating FROM login),
			(SELECT total_balance FROM login),
			(SELECT user_role FROM login)
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
			&p.Ads_Rating,
			&Review_count_mass,

			&p.Ads_id,
			&p.Owner_id,
			&p.Category_id,

			&p.Id,
			&p.Login,
			&p.Name,
			&p.Surname_or_Ind_num,
			&p.Patronomic_or_Addres_name,
			&p.User_Rating,
			&p.Total_balance,
			&p.User_role,
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
