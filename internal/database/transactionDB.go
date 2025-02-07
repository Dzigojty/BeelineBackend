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
