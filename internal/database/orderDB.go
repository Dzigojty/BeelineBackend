package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/jung-kurt/gofpdf"
)

func dateInterpreter(starts_at, ends_at time.Time) (int, float64) {
	//интервал
	duration := ends_at.Sub(starts_at)

	// Получаем количество дней (целое число)
	days := int(duration.Hours() / 24)

	// Получаем количество часов (оставшиеся часы после вычитания дней)
	hours := duration.Hours() - float64(days*24)

	return days, hours
}

func pdfCreater(User_id int, Ads_id int, Starts_at time.Time, Ends_at time.Time, Order_id int, Updated_frozen_funds int, Booking_id int, Transaction_id int, Amount int) {
	// Создаем новый PDF-документ
	pdf := gofpdf.New("P", "mm", "A4", "")

	// Добавляем страницу
	pdf.AddPage()

	// Заголовок
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(0, 10, "PDF file.")
	pdf.Ln(12)

	// Данные для документа
	pdf.SetFont("Arial", "", 14)
	data := map[string]string{
		"User_id":              strconv.Itoa(User_id),
		"Ads_id":               strconv.Itoa(Ads_id),
		"Starts_at":            Starts_at.Format("2006-01-02 15:04:05"),
		"Ends_at":              Ends_at.Format("2006-01-02 15:04:05"),
		"Order_id":             strconv.Itoa(Order_id),
		"Updated_frozen_funds": strconv.Itoa(Updated_frozen_funds),
		"Booking_id":           strconv.Itoa(Booking_id),
		"Transaction_id":       strconv.Itoa(Transaction_id),
		"Amount":               strconv.Itoa(Amount),
	}

	// Добавляем данные в PDF, каждое поле на отдельной строке
	for key, value := range data {
		pdf.Cell(0, 10, fmt.Sprintf("%s: %s", key, value))
		pdf.Ln(8) // Переход на новую строку
	}

	// Сохраняем PDF
	outputPath := "/home/beeline/beeline_data_base_8070/example_with_table.pdf"
	err := pdf.OutputFileAndClose(outputPath)
	if err != nil {
		log.Fatalf("Ошибка создания PDF: %v", err)
	}

	log.Println("PDF успешно создан:", outputPath)
}

func (repo *MyRepository) RegOrderHourlySQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, rdb *redis.Client, conn map[int]*websocket.Conn, user_id, ads_id int, starts_at, ends_at time.Time, positionX, positionY float64) (err error) {
	type Product struct {
		Order_id             int `json:"Order_id"`
		Updated_frozen_funds int `json:"Updated_frozen_funds"`
		Booking_id           int `json:"Booking_id"`
		Transaction_id       int `json:"Transaction_id"`
		Amount               int `json:"Amount"`
	}

	products := []Product{}

	request, err := rep.Query(
		ctx,
		`
		WITH owner AS ( -- данные хозяина объявления
			SELECT owner_id, hourly_rate FROM ads.ads WHERE id = $1
		),
		wallet AS ( -- данные арендатора
			SELECT id AS wallet_id, total_balance FROM finance.wallets WHERE user_id = $2
		), 
		transact AS (
			INSERT INTO finance.transactions (wallet_id, amount, typee, user_2)
			SELECT (SELECT wallet_id FROM wallet),
				(SELECT $4::timestamp::date - $3::timestamp::date)
				* (SELECT hourly_rate FROM owner),
				3,
				(SELECT owner_id FROM owner)
			WHERE (SELECT $4::timestamp::date - $3::timestamp::date)
				* (SELECT hourly_rate FROM owner) <= (SELECT total_balance FROM wallet)
			RETURNING id, amount
		),
		wallet_update AS (
			UPDATE finance.wallets
			SET frozen_funds = frozen_funds + (SELECT amount FROM transact),
				total_balance = total_balance - (SELECT amount FROM transact)
			WHERE user_id = $2 AND (SELECT amount FROM transact) <= total_balance
			RETURNING frozen_funds
		),
		orderr AS (
			INSERT INTO orders.orders(position, total_price)
			SELECT POINT($5, $6),
			(SELECT $4::timestamp::date - $3::timestamp::date) * (SELECT hourly_rate FROM owner)
			WHERE (SELECT $4::timestamp::date - $3::timestamp::date)
				* (SELECT hourly_rate FROM owner) <= (SELECT total_balance FROM wallet)
			RETURNING id, created_at, position
		),
		booking AS (
			INSERT INTO orders.bookings(ads_id, starts_at, ends_at, transaction_id, typee, order_id)
			SELECT $1, $3, $4,
				(SELECT id FROM transact), 1, (SELECT id FROM orderr)
			WHERE (SELECT $4::timestamp::date - $3::timestamp::date)
				* (SELECT hourly_rate FROM owner) <= (SELECT total_balance FROM wallet)
			RETURNING id, starts_at, ends_at
		)
		SELECT
			(SELECT owner_id FROM owner) AS owner_id,
			(SELECT id FROM orderr) AS order_id,
			(SELECT frozen_funds FROM wallet_update) AS updated_frozen_funds,
			(SELECT id FROM booking) AS booking_id,
			(SELECT id FROM transact) AS transaction_id,
			(SELECT starts_at FROM booking),
			(SELECT ends_at FROM booking),
			(SELECT created_at FROM orderr),
			(SELECT amount FROM transact);
		`,

		ads_id,
		user_id,
		starts_at,
		ends_at,
		positionX,
		positionY,
	)

	type Response struct {
		Status  string    `json:"status"`
		Data    []Product `json:"data,omitempty"`
		Message string    `json:"message"`
	}

	if request == nil {
		response := Response{
			Status:  "fatal",
			Message: "Не прошла",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	}

	var buddy_id int
	var start_at time.Time
	var end_at time.Time
	var created_at time.Time

	for request.Next() {
		p := Product{}
		err := request.Scan(
			&buddy_id,
			&p.Order_id,
			&p.Updated_frozen_funds,
			&p.Booking_id,
			&p.Transaction_id,
			&start_at,
			&end_at,
			&created_at,
			&p.Amount)
		if err != nil {
			fmt.Println(err)
			continue
		}

		products = append(products, Product{Order_id: p.Order_id, Transaction_id: p.Transaction_id, Booking_id: p.Booking_id, Updated_frozen_funds: p.Updated_frozen_funds, Amount: p.Amount})
	}

	if err != nil || len(products) == 0 {
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

	// запускаем горутину на формирование документа
	pdfCreater(user_id, ads_id, starts_at, ends_at, products[0].Order_id, products[0].Updated_frozen_funds, products[0].Booking_id, products[0].Transaction_id, products[0].Amount)

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

	// buddy_id это кому отправляем сообщение
	Order_Notification(ctx, buddy_id, conn[buddy_id], "reg_order", ads_id, start_at, end_at, "У вас арендовали технику", created_at, user_id, user_role, ServeSpecificMediaBase64(rw, r, avatar_path), name, rdb)

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)
	return
}

func (repo *MyRepository) RegOrderDailySQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, rdb *redis.Client, conn map[int]*websocket.Conn, user_id, ads_id int, starts_at, ends_at time.Time, positionX, positionY float64) (err error) {
	type Product struct {
		Order_id             int `json:"Order_id"`
		Updated_frozen_funds int `json:"Updated_frozen_funds"`
		Booking_id           int `json:"Booking_id"`
		Transaction_id       int `json:"Transaction_id"`
		Amount               int `json:"Amount"`
	}

	products := []Product{}

	request, err := rep.Query(
		ctx,
		`
		WITH owner AS ( -- данные хозяина объявления
			SELECT owner_id, daily_rate FROM ads.ads WHERE id = $1
		),
		wallet AS ( -- данные арендатора
			SELECT id AS wallet_id, total_balance FROM finance.wallets WHERE user_id = $2
		), 
		transact AS (
			INSERT INTO finance.transactions (wallet_id, amount, typee, user_2)
			SELECT (SELECT wallet_id FROM wallet),
				(SELECT $4::timestamp::date - $3::timestamp::date)
				* (SELECT daily_rate FROM owner),
				3,
				(SELECT owner_id FROM owner)
			WHERE (SELECT $4::timestamp::date - $3::timestamp::date)
				* (SELECT daily_rate FROM owner) <= (SELECT total_balance FROM wallet)
			RETURNING id, amount
		),
		wallet_update AS (
			UPDATE finance.wallets
			SET frozen_funds = frozen_funds + (SELECT amount FROM transact),
				total_balance = total_balance - (SELECT amount FROM transact)
			WHERE user_id = $2 AND (SELECT amount FROM transact) <= total_balance
			RETURNING frozen_funds
		),
		orderr AS (
			INSERT INTO orders.orders(position, total_price)
			SELECT POINT($5, $6),
			(SELECT $4::timestamp::date - $3::timestamp::date) * (SELECT daily_rate FROM owner)
			WHERE (SELECT $4::timestamp::date - $3::timestamp::date)
				* (SELECT daily_rate FROM owner) <= (SELECT total_balance FROM wallet)
			RETURNING id, created_at
		),
		booking AS (
			INSERT INTO orders.bookings(ads_id, starts_at, ends_at, transaction_id, typee, order_id)
			SELECT $1, $3, $4,
				(SELECT id FROM transact), 1, (SELECT id FROM orderr)
			WHERE (SELECT $4::timestamp::date - $3::timestamp::date)
				* (SELECT daily_rate FROM owner) <= (SELECT total_balance FROM wallet)
			RETURNING id, starts_at, ends_at
		)
		SELECT
			(SELECT owner_id FROM owner) AS owner_id,
			(SELECT id FROM orderr) AS order_id,
			(SELECT frozen_funds FROM wallet_update) AS updated_frozen_funds,
			(SELECT id FROM booking) AS booking_id,
			(SELECT id FROM transact) AS transaction_id,
			(SELECT starts_at FROM booking),
			(SELECT ends_at FROM booking),
			(SELECT created_at FROM orderr),
			(SELECT amount FROM transact);
		`,

		ads_id,
		user_id,
		starts_at,
		ends_at,
		positionX,
		positionY,
	)
	fmt.Println(err)
	errorr(err)

	type Response struct {
		Status  string    `json:"status"`
		Data    []Product `json:"data,omitempty"`
		Message string    `json:"message"`
	}

	if request == nil {
		response := Response{
			Status:  "fatal",
			Message: "Не прошла",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	}

	var buddy_id int
	var start_at time.Time
	var end_at time.Time
	var created_at time.Time

	for request.Next() {
		p := Product{}
		err := request.Scan(
			&buddy_id,
			&p.Order_id,
			&p.Updated_frozen_funds,
			&p.Booking_id,
			&p.Transaction_id,
			&start_at,
			&end_at,
			&created_at,
			&p.Amount)
		if err != nil {
			fmt.Println(err)
			continue
		}

		products = append(products, Product{Order_id: p.Order_id, Transaction_id: p.Transaction_id, Booking_id: p.Booking_id, Updated_frozen_funds: p.Updated_frozen_funds, Amount: p.Amount})
	}

	if err != nil || len(products) == 0 {
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

	// запускаем на формирование документа
	pdfCreater(user_id, ads_id, starts_at, ends_at, products[0].Order_id, products[0].Updated_frozen_funds, products[0].Booking_id, products[0].Transaction_id, products[0].Amount)

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
	fmt.Println(err)
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

	// buddy_id это кому отправляем сообщение
	Order_Notification(ctx, buddy_id, conn[buddy_id], "reg_order", ads_id, start_at, end_at, "У вас арендовали технику", created_at, user_id, user_role, ServeSpecificMediaBase64(rw, r, avatar_path), name, rdb)

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)
	return
}

func (repo *MyRepository) RegOrderWithBiddingSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, rdb *redis.Client, conn map[int]*websocket.Conn, chat_id, global_rate, user_id int, start_at, end_at time.Time, positX, positY float64) (err error) {
	type Product struct {
		Order_id             *int `json:"Order_id"`
		Updated_frozen_funds *int `json:"Updated_frozen_funds"`
		Booking_id           *int `json:"Booking_id"`
		Transaction_id       *int `json:"Transaction_id"`
		Price                *int `json:"Price"`
	}

	products := []Product{}

	// Выполняем SQL-запрос и проверяем ошибки
	request, err := rep.Query(
		ctx,
		`
		WITH chat AS (
			SELECT ad_id FROM chat.chats WHERE id = $1
		),
		owner AS (
			SELECT owner_id FROM ads.ads WHERE id = (SELECT ad_id FROM chat)
		),
		price AS (
			INSERT INTO Finance.bidding (ads_id, renter_id, global_rate, start_at, end_at, position)
			SELECT (SELECT ad_id FROM chat), $2, $3, $4, $5, POINT($6, $7)
			WHERE (SELECT owner_id FROM owner) != $2
			RETURNING id, global_rate
		),
		wallet AS ( -- данные арендатора
			SELECT id AS wallet_id, total_balance FROM finance.wallets WHERE user_id = $2
		), 
		transact AS (
			INSERT INTO finance.transactions (wallet_id, amount, typee, user_2)
			SELECT (SELECT wallet_id FROM wallet),
				(SELECT global_rate FROM price),
				3,
				(SELECT owner_id FROM owner)
			WHERE (SELECT global_rate FROM price) <= (SELECT total_balance FROM wallet)
			RETURNING id, amount
		),
		wallet_update AS (
			UPDATE finance.wallets
			SET frozen_funds = frozen_funds + (SELECT amount FROM transact) -- ,
				-- total_balance = total_balance - (SELECT amount FROM transact)
			WHERE user_id = $2 AND (SELECT amount FROM transact) <= total_balance
			RETURNING frozen_funds, total_balance
		),
		orderr AS (
			INSERT INTO orders.orders(position, total_price)
			SELECT POINT($6, $7),
			(SELECT global_rate FROM price)
			WHERE (SELECT global_rate FROM price) <= (SELECT total_balance FROM wallet)
			RETURNING id
		),
		booking AS (
			INSERT INTO orders.bookings(ads_id, starts_at, ends_at, transaction_id, typee, order_id)
			SELECT (SELECT ad_id FROM chat), $4, $5,
				(SELECT id FROM transact), 1, (SELECT id FROM orderr)
			WHERE (SELECT global_rate FROM price) <= (SELECT total_balance FROM wallet)
			RETURNING id
		)
		SELECT
			(SELECT id FROM orderr) AS order_id,
			(SELECT frozen_funds FROM wallet_update) AS updated_frozen_funds,
			(SELECT id FROM booking) AS booking_id,
			(SELECT id FROM transact) AS transaction_id,
			(SELECT global_rate FROM price);
		`,
		chat_id,
		user_id,
		global_rate,
		start_at,
		end_at,
		positX,
		positY)
	errorr(err)
	fmt.Println(err)

	defer request.Close() // Закрываем запрос в конце выполнения функции

	if request.Next() {
		// Используем &bidding_id, так как Scan требует указатель
		p := Product{}
		err := request.Scan(
			&p.Order_id,
			&p.Updated_frozen_funds,
			&p.Booking_id,
			&p.Transaction_id,
			&p.Price)
		if err != nil {
			fmt.Println(err)
			return err
		}

		products = append(products, Product{Order_id: p.Order_id, Transaction_id: p.Transaction_id, Booking_id: p.Booking_id, Updated_frozen_funds: p.Updated_frozen_funds, Price: p.Price})
	}

	// Структура ответа
	type Response struct {
		Status  string    `json:"status"`
		Data    []Product `json:"data,omitempty"`
		Message string    `json:"message"`
	}

	// Проверяем, был ли id успешным
	if err != nil {
		response := Response{
			Status:  "fatal",
			Message: "Не прошла",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	}

	// Успешный ответ с данными
	response := Response{
		Status:  "success",
		Data:    products,
		Message: "Транзакция прошла успешно",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)
	return nil
}

func (repo *MyRepository) RebookOrderHourlySQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, user_id, order_id int, starts_at, ends_at time.Time) (err error) {
	var Booking_id int

	request, err := rep.Query(
		ctx,
		`
		WITH ads AS (
			SELECT bookings.ads_id, bookings.transaction_id
			FROM orders.bookings, ads.ads, finance.transactions, finance.wallets, users.users
			WHERE bookings.order_id = $1 AND bookings.ads_id = ads.id AND
				bookings.transaction_id = transactions.id AND transactions.wallet_id = wallets.id AND
				wallets.user_id = users.id AND users.id = $2
			LIMIT 1
		),
		booking AS (
			INSERT INTO orders.bookings(ads_id, starts_at, ends_at, typee, order_id)
			SELECT (SELECT ads_id FROM ads),
				$3,
				$4,
				2,
				$1
			RETURNING id
		)
		SELECT
			(SELECT id FROM booking) AS booking_id;
		`,

		order_id,
		user_id,
		starts_at,
		ends_at,
	)
	errorr(err)

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if request == nil {
		response := Response{
			Status:  "fatal",
			Message: "Процедура не прошла",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	}

	for request.Next() {
		err := request.Scan(
			&Booking_id,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	if err != nil || Booking_id == 0 {
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
		Data:    Booking_id,
		Message: "Транзакция прошла успешно",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)
	return
}

func (repo *MyRepository) RebookOrderDailySQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, user_id, order_id int, starts_at, ends_at time.Time) (err error) {
	type Product struct {
		Order_id             int  `json:"Order_id"`
		Updated_frozen_funds *int `json:"Updated_frozen_funds"`
		Booking_id           *int `json:"Booking_id"`
		Transaction_id       *int `json:"Transaction_id"`
	}

	products := []Product{}

	request, err := rep.Query(
		ctx,
		`
		WITH ads AS (
			SELECT bookings.ads_id
			FROM orders.bookings, ads.ads, finance.transactions, finance.wallets, users.users
			WHERE bookings.order_id = $1 AND bookings.ads_id = ads.id AND
				bookings.transaction_id = transactions.id AND transactions.wallet_id = wallets.id AND
				wallets.user_id = users.id AND users.id = $2
			LIMIT 1
		),
		owner AS ( -- данные хозяина объявления
			SELECT owner_id, daily_rate FROM ads.ads WHERE id = (SELECT ads_id FROM ads)
		),
		wallet AS ( -- данные арендатора
			SELECT id AS wallet_id, total_balance FROM finance.wallets WHERE user_id = $2
		), 
		transact AS (
			INSERT INTO finance.transactions (wallet_id, amount, typee, user_2)
			SELECT (SELECT wallet_id FROM wallet),
				(SELECT $4::timestamp::date - $3::timestamp::date)
				* (SELECT daily_rate FROM owner),
				3,
				(SELECT owner_id FROM owner)
			WHERE (SELECT $4::timestamp::date - $3::timestamp::date)
				* (SELECT daily_rate FROM owner) <= (SELECT total_balance FROM wallet)
			RETURNING id, amount
		),
		wallet_update AS (
			UPDATE finance.wallets
			SET frozen_funds = frozen_funds + (SELECT amount FROM transact),
				total_balance = total_balance - (SELECT amount FROM transact)
			WHERE user_id = $2 AND frozen_funds + (SELECT amount FROM transact) <= total_balance
			RETURNING frozen_funds
		),
		booking AS (
			INSERT INTO orders.bookings(ads_id, starts_at, ends_at, transaction_id, typee, order_id)
			SELECT (SELECT ads_id FROM ads),
				$3,
				$4,
				(SELECT id FROM transact), 2, $1
			WHERE (SELECT $4::timestamp::date - $3::timestamp::date)
				* (SELECT daily_rate FROM owner) <= (SELECT total_balance FROM wallet)
			RETURNING id
		)
		SELECT
			(SELECT frozen_funds FROM wallet_update) AS updated_frozen_funds,
			(SELECT id FROM booking) AS booking_id,
			(SELECT id FROM transact) AS transaction_id;
		`,

		order_id,
		user_id,
		starts_at,
		ends_at,
	)
	errorr(err)

	type Response struct {
		Status  string    `json:"status"`
		Data    []Product `json:"data,omitempty"`
		Message string    `json:"message"`
	}

	if request == nil {
		response := Response{
			Status:  "fatal",
			Message: "Не прошла",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	}

	for request.Next() {
		p := Product{}
		err := request.Scan(
			&p.Updated_frozen_funds,
			&p.Booking_id,
			&p.Transaction_id)
		if err != nil {
			fmt.Println(err)
			continue
		}

		products = append(products, Product{Order_id: order_id, Transaction_id: p.Transaction_id, Booking_id: p.Booking_id, Updated_frozen_funds: p.Updated_frozen_funds})
	}

	if err != nil || len(products) == 0 {
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

func (repo *MyRepository) SucBookingSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, order_id []int, id_user int) (err error) {
	type Product struct {
		Amount  float64
		Type    int
		User_id int
	}
	products := []Product{}

	for i := range len(order_id) {
		request, err := rep.Query(
			ctx,
			`
			WITH booking AS (
				SELECT amount, transaction_id FROM Orders.bookings WHERE id = 80 AND type != 3
			),
			del_booking AS (
				UPDATE Orders.bookings
				SET type = 3
				WHERE id = 80 AND type != 3
				RETURNING type
			),
			wallet AS (
				SELECT user_2 FROM Finance.transactions
				WHERE id = (SELECT transaction_id FROM booking)
			),
			calculation_money AS (
				UPDATE Finance.wallets
				SET total_balance = total_balance + (SELECT amount FROM booking),
				frozen_funds = frozen_funds + (SELECT amount FROM booking)
				WHERE user_id = (SELECT user_2 FROM wallet) AND user_id = 29
				RETURNING user_id
			)
			SELECT
				(SELECT amount FROM booking),
				(SELECT type FROM del_booking) AS deleted_type,
				(SELECT user_id FROM calculation_money);
		`,

			order_id[i],
			id_user)
		errorr(err)

		for request.Next() {
			p := Product{}
			err := request.Scan(
				&p.Amount,
				&p.Type,
				&p.User_id)
			if err != nil {
				fmt.Println(err)
				continue
			}

			products = append(products, Product{Amount: p.Amount, Type: p.Type, User_id: p.User_id})
		}

		type Response struct {
			Status  string    `json:"status"`
			Data    []Product `json:"data,omitempty"`
			Message string    `json:"message"`
		}

		if err != nil || len(products) == 0 {
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
	}
	return
}

func (repo *MyRepository) BookingListSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, id_users int) (err error) {
	type Product struct {
		Booking_id int
	}
	products := []Product{}

	request, err := rep.Query(
		ctx,
		`
		WITH wallet AS (
			SELECT id AS wallet_id FROM finance.wallets WHERE user_id = $1
		),
		transact AS (
			SELECT id AS transact_id, wallet_id FROM finance.transactions
		),
		bookings AS (
			SELECT id AS booking_id, transaction_id FROM orders.bookings
		)
		SELECT bookings.booking_id
		FROM wallet
		JOIN transact ON wallet.wallet_id = transact.wallet_id
		JOIN bookings ON bookings.transaction_id = transact.transact_id;
		`,

		id_users,
	)

	for request.Next() {
		p := Product{}
		err := request.Scan(
			&p.Booking_id,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
		products = append(products, Product{Booking_id: p.Booking_id})
	}

	type Response struct {
		Status  string
		Data    []Product
		Message string
	}

	if err != nil || len(products) == 0 {
		response := Response{
			Status:  "fatal",
			Message: "Операция не прошла",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return err
	}

	response := Response{
		Status:  "success",
		Data:    products,
		Message: "Бронирования показаны",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return
}

func (repo *MyRepository) GroupOrdersByRentedSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool) (err error) {
	type Product struct {
		Id          int
		Ad_id       int
		Renter_id   int
		Total_price int
		Created_at  time.Time
	}
	products := []Product{}

	request, err := rep.Query(
		ctx,
		"SELECT id, ad_id, renter_id, total_price, created_at FROM Orders.orders WHERE status = 1;",
	)
	if err != nil {
		err = fmt.Errorf("failed to exec data: %w", err)
		return
	}

	for request.Next() {
		p := Product{}
		err := request.Scan(
			&p.Id,
			&p.Ad_id,
			&p.Renter_id,
			&p.Total_price,
			&p.Created_at,
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

	if err != nil || len(products) == 0 {
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

func (repo *MyRepository) GroupOrdersByUnRentedSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool) (err error) {
	type Product struct {
		Id          int
		Ad_id       int
		Renter_id   int
		Total_price int
		Created_at  time.Time
	}
	products := []Product{}

	request, err := rep.Query(
		ctx,
		"SELECT id, ad_id, renter_id, total_price, created_at FROM Orders.orders WHERE status = 2;",
	)
	if err != nil {
		err = fmt.Errorf("failed to exec data: %w", err)
		return
	}

	fmt.Fprintln(rw, "id, ad_id, renter_id, total_price, created_at")

	for request.Next() {
		p := Product{}
		err := request.Scan(
			&p.Id,
			&p.Ad_id,
			&p.Renter_id,
			&p.Total_price,
			&p.Created_at,
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

	if err != nil || len(products) == 0 {
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

func (repo *MyRepository) SigPDFfileSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, order_id, user_id int) (err error) {
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
		WITH orderr AS (
			SELECT total_price
			FROM orders.orders
			WHERE id = 30
		),
		booking AS (
			SELECT 
				ARRAY_AGG(starts_at) AS starts_at,
				ARRAY_AGG(ends_at) AS ends_at,
				ARRAY_AGG(ads_id) AS ads_id
			FROM orders.bookings
			WHERE order_id = 30
		),
		ads AS (
			SELECT
				(SELECT total_price FROM orderr),
				(SELECT starts_at FROM booking),
				(SELECT ends_at FROM booking),
				(SELECT ads_id FROM booking),
				owner_id
			FROM ads.ads
			WHERE id IN (
				SELECT UNNEST((SELECT ads_id FROM booking))
			)
		),
		chat AS (
			SELECT id
			FROM chat.chats
			WHERE ad_id = (SELECT ads_id FROM booking) AND (user_1_id = $2 OR user_2_id = $2)
		),
		i AS (
			SELECT id, text, sent_at, sender_id
			FROM chat.messages 
			WHERE chat_id = (SELECT id FROM chat)
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
			SELECT user_1_id, user_2_id, ad_id, have_disput, mediator_id FROM  chat.chats WHERE chats.id = (SELECT id FROM chat)
		),
		ownerr AS (
			SELECT owner_id FROM ads.ads WHERE id = (SELECT ad_id FROM chat_info)
		),
		global_rate_info AS (
			WITH i AS (
				SELECT ad_id FROM chat.chats WHERE chats.id = (SELECT id FROM chat)
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

		order_id,
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

	request, err := rep.Query( //это запрос на вывод сообщений нашего кента
		ctx,
		`
		WITH orderr AS (
			SELECT total_price
			FROM orders.orders
			WHERE id = $1
		),
		booking AS (
			SELECT 
				ARRAY_AGG(starts_at) AS starts_at,
				ARRAY_AGG(ends_at) AS ends_at,
				ARRAY_AGG(ads_id) AS ads_id
			FROM orders.bookings
			WHERE order_id = $1
		),
		ads AS (
			SELECT
				(SELECT total_price FROM orderr),
				(SELECT starts_at FROM booking),
				(SELECT ends_at FROM booking),
				(SELECT ads_id FROM booking),
				owner_id
			FROM ads.ads
			WHERE id IN (
				SELECT UNNEST((SELECT ads_id FROM booking))
			)
		),
		
		`,

		order_id,
	)
	errorr(err)

	var total_price int
	var start_at []time.Time
	var end_at []time.Time

	for request.Next() {
		err := request_2.Scan(
			&total_price,
			&start_at,
			&end_at,
		)
		if err != nil {
			fmt.Println(err)

			continue
		}
	}

	type Response struct {
		Status      string      `json:"status"`
		Total_price int         `json:"total_price"`
		Start_at    []time.Time `json:"start_at"`
		End_at      []time.Time `json:"end_at"`

		Ads_id       int            `json:"ads_id"`
		Disput_state bool           `json:"disput_state"`
		Mediator_id  int            `json:"moderator_id"`
		Slave_id     int            `json:"slave_id"`
		Owner_id     int            `json:"owner_id"`
		Global_rate  int            `json:"global_rate"`
		Data         []Product_user `json:"data,omitempty"`
		Message      string         `json:"message"`
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
			Status:      "success",
			Total_price: total_price,
			Start_at:    start_at,
			End_at:      end_at,

			Ads_id:       ads_id,
			Disput_state: disput_state,
			Mediator_id:  mediator_id,
			Slave_id:     slave_id,
			Owner_id:     owner_id,
			Global_rate:  global_rate,
			Data:         Products_user_mass,
			Message:      "Показано",
		}

		// запускаем на формирование документа
		pdfCreater(user_id, ads_id, time.Now(), time.Now(), 0, 0, 0, 0, 0)

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
