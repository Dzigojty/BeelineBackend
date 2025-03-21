package database

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v4/pgxpool"
)

type Message struct {
	User_id int `json:"User_id"`
}

func Order_Notification(ctx context.Context, buddy_id int, conn *websocket.Conn, header string, ads_id int, start_at time.Time, end_at time.Time, text string, reg_at time.Time, user_id, user_role int, avatar, name string, rdb *redis.Client) {
	type NotifType struct {
		Header    string
		Ads_id    int
		Text      string
		Start_at  time.Time
		End_at    time.Time
		Reg_at    time.Time
		User_id   int
		User_role int
		Avatar    string
		Name      string
	}

	// Создаем объект структуры
	notification := NotifType{
		Header:    header,
		Ads_id:    ads_id,
		Text:      text,
		Start_at:  start_at,
		End_at:    end_at,
		Reg_at:    reg_at,
		User_id:   user_id,
		User_role: user_role,
		Avatar:    avatar,
		Name:      name,
	}

	// Сериализуем структуру в JSON
	message, err := json.Marshal(notification)
	if err != nil {
		fmt.Println("Ошибка при сериализации уведомления:", err)
		return
	}

	if conn == nil {
		// Сохраняем код подтверждения в Redis с TTL на 60 дней
		err := rdb.LPush(ctx, strconv.Itoa(buddy_id), []byte(message), (2 * 30 * 24 * time.Hour)).Err()
		errorr(err)

		return
	}

	err = conn.WriteMessage(websocket.TextMessage, []byte(message))
	errorr(err)
}

func Reviews_Notification(ctx context.Context, buddy_id int, conn *websocket.Conn, header string, order_id int, text string, reg_at time.Time, user_id, user_role int, avatar, name string, rdb *redis.Client) {
	type NotifType struct {
		Header    string
		Order_id  int
		Text      string
		Reg_at    time.Time
		User_id   int
		User_role int
		Avatar    string
		Name      string
	}

	// Создаем объект структуры
	notification := NotifType{
		Header:    header,
		Order_id:  order_id,
		Text:      text,
		Reg_at:    reg_at,
		User_id:   user_id,
		User_role: user_role,
		Avatar:    avatar,
		Name:      name,
	}

	// Сериализуем структуру в JSON
	message, err := json.Marshal(notification)
	if err != nil {
		fmt.Println("Ошибка при сериализации уведомления:", err)
		return
	}

	if conn == nil {
		// Сохраняем код подтверждения в Redis с TTL на 60 дней
		err := rdb.LPush(ctx, strconv.Itoa(buddy_id), []byte(message), (2 * 30 * 24 * time.Hour)).Err()
		errorr(err)

		return
	}

	err = conn.WriteMessage(websocket.TextMessage, []byte(message))
	errorr(err)
}

func Notification(ctx context.Context, buddy_id int, conn *websocket.Conn, header string, chat_id, mess_id int, text string, sent_at time.Time, user_id, user_role int, avatar, name string, rdb *redis.Client) {
	type NotifType struct {
		Header    string
		Chat_id   int
		Mess_id   int
		Text      string
		Sent_at   time.Time
		User_id   int
		User_role int
		Avatar    string
		Name      string
	}

	// Создаем объект структуры
	notification := NotifType{
		Header:    header,
		Chat_id:   chat_id,
		Mess_id:   mess_id,
		Text:      text,
		Sent_at:   sent_at,
		User_id:   user_id,
		User_role: user_role,
		Avatar:    avatar,
		Name:      name,
	}

	// Сериализуем структуру в JSON
	message, err := json.Marshal(notification)
	if err != nil {
		fmt.Println("Ошибка при сериализации уведомления:", err)
		return
	}

	if conn == nil {
		// Сохраняем код подтверждения в Redis с TTL на 60 дней
		err := rdb.LPush(ctx, strconv.Itoa(buddy_id), []byte(message), (2 * 30 * 24 * time.Hour)).Err()
		errorr(err)

		return
	}

	err = conn.WriteMessage(websocket.TextMessage, []byte(message))
	errorr(err)
}

func (repo *MyRepository) ChatButtonInAdsSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, id_user int, id_ads int) (err error) {
	request, err := rep.Query(
		ctx,
		"SELECT chat.reg_or_open_chat($1, $2);",

		id_user,
		id_ads,
	)
	errorr(err)

	var chat_id int
	for request.Next() {
		err := request.Scan(
			&chat_id,
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

	if err == nil && request != nil && chat_id != 0 {
		response := Response{
			Status:  "success",
			Data:    chat_id,
			Message: "Показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return
	}

	response := Response{
		Status:  "fatal",
		Message: "Не показано, или не зарегистрировано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) SigChatSQL(ctx context.Context, rw http.ResponseWriter, id_user int, id_ads int, rep *pgxpool.Pool) (err error) {
	request, err := rep.Query(
		ctx,
		"SELECT ads.owner_id FROM ads.ads WHERE id = $1;",

		id_ads,
	)
	errorr(err)

	var id_buddy int
	for request.Next() {
		err := request.Scan(
			&id_buddy,
		)
		if err != nil {
			fmt.Println(err)

			continue
		}
	}

	request, err = rep.Query(
		ctx,
		"SELECT Chat.add_chat($1, $2, $3);",

		id_user,
		id_buddy,
		id_ads,
	)
	errorr(err)

	var chat_id int
	for request.Next() {
		err := request.Scan(
			&chat_id,
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

	if err == nil && request != nil && chat_id != 0 {
		response := Response{
			Status:  "success",
			Data:    chat_id,
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

func (repo *MyRepository) OpenChatSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, id_chat, user_id int) (err error) {
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
			WHERE user_id IN (SELECT sender_id FROM i)
		),
		individual_user AS (
			SELECT user_id, name 
			FROM users.individual_user 
			WHERE user_id IN (SELECT sender_id FROM i)
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
		),
		mediator_info AS (
			SELECT name, surname, patronymic FROM users.individual_user WHERE user_id = (SELECT mediator_id FROM chat_info)
		)
		SELECT
			COALESCE((SELECT COALESCE(individual_user.name, company_user.name_of_company) WHERE COALESCE(individual_user.user_id, company_user.user_id) != $2), 'наше имя') AS buddy_name,
			(SELECT ad_id FROM chat_info) AS ads_id,
			(SELECT have_disput FROM chat_info) AS disput_state,
			COALESCE((SELECT name FROM mediator_info), 'Нет имени'),
			COALESCE((SELECT surname FROM mediator_info), 'Нет фамилии'),
			COALESCE((SELECT patronymic FROM mediator_info), 'Нет отчества'),
			COALESCE((SELECT user_1_id FROM chat_info WHERE user_1_id != (SELECT owner_id FROM ownerr)),
				(SELECT user_2_id FROM chat_info WHERE user_2_id != (SELECT owner_id FROM ownerr))) AS slsve_id,
			(SELECT owner_id FROM ownerr),
			COALESCE((SELECT global_rate FROM global_rate_info), 0),

			i.id AS message_id,
			i.sender_id AS user_id,
			COALESCE(individual_user.name, company_user.name_of_company) AS name,
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

	var buddyName string
	var ads_id int
	var disput_state bool
	var mediator_name string
	var mediator_surname string
	var mediator_patronymic string
	var slave_id int
	var owner_id int
	var global_rate int

	for request_2.Next() {
		err := request_2.Scan(
			&buddyName,
			&ads_id,
			&disput_state,
			&mediator_name,
			&mediator_surname,
			&mediator_patronymic,
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

	type Response struct {
		Status              string         `json:"status"`
		BuddyName           string         `json:"buddyName"`
		Ads_id              int            `json:"ads_id"`
		Disput_state        bool           `json:"disput_state"`
		Mediator_name       string         `json:"mediator_name"`
		Mediator_surname    string         `json:"mediator_surname"`
		Mediator_patronymic string         `json:"mediator_patronymic"`
		Slave_id            int            `json:"slave_id"`
		Owner_id            int            `json:"owner_id"`
		Global_rate         int            `json:"global_rate"`
		Data                []Product_user `json:"data,omitempty"`
		Message             string         `json:"message"`
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
			Status:              "success",
			BuddyName:           buddyName,
			Ads_id:              ads_id,
			Disput_state:        disput_state,
			Mediator_name:       mediator_name,
			Mediator_surname:    mediator_surname,
			Mediator_patronymic: mediator_patronymic,
			Slave_id:            slave_id,
			Owner_id:            owner_id,
			Global_rate:         global_rate,
			Data:                Products_user_mass,
			Message:             "Показано",
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

func Notofication(rep *pgxpool.Pool, ctx context.Context, rw http.ResponseWriter, id_chat, id_user int, text string, date time.Time) {
	request, err := rep.Query(
		ctx,
		`
			WITH i AS (
				SELECT user_1_id FROM Chat.chats WHERE id = $1 AND user_1_id != $2 AND user_2_id = $2
			),
			j AS (
				SELECT user_2_id FROM Chat.chats WHERE id = $1 AND user_1_id = $2 AND user_2_id != $2
			)
			SELECT i.user_1_id FROM i
			UNION ALL
			SELECT j.user_2_id FROM j
			WHERE j.user_2_id IS NOT NULL
			LIMIT 1;  -- Ограничиваем результат до одного значения
		`,

		id_chat,
		id_user)
	errorr(err)

	var recipient int

	for request.Next() {
		err := request.Scan(
			&recipient,
		)
		if err != nil {
			fmt.Println(err)

			continue
		}
	}

	type A struct {
		Recipient int       `json:"Recipient"`
		Text      string    `json:"Text"`
		Date      time.Time `json:"Date"`
	}

	type Response struct {
		Status  string `json:"status"`
		Data    A      `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err == nil && recipient != 0 {
		response := Response{
			Status:  "success",
			Data:    A{Recipient: recipient, Text: text, Date: date},
			Message: "Уведомление показано",
		}

		json.NewEncoder(rw).Encode(response)

		return
	}

	response := Response{
		Status:  "fatal",
		Message: "Такого сообщения не найденно ",
	}

	if err != nil {
		response.Message = err.Error()
	}

	json.NewEncoder(rw).Encode(response)
}

func (repo *MyRepository) SendMessageAndMediaSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, id_chat, id_user int, text string, file_paths []string, conn map[int]*websocket.Conn, rdb *redis.Client) (err error) {
	request, err := rep.Query(
		ctx,
		`
		INSERT INTO Chat.messages(chat_id, sender_id, text)
		VALUES ($1, $2, $3)
		RETURNING id;
	`,

		id_chat,
		id_user, // тот, кто отправляет сообщение
		text,
	)
	errorr(err)
	fmt.Println(err)

	var mess_id int

	for request.Next() {
		err := request.Scan(
			&mess_id,
		)
		if err != nil {
			fmt.Println(err)

			continue
		}
	}
	fmt.Println(err)

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err == nil && mess_id != 0 {
		request, err := rep.Query(
			ctx,
			`SELECT sent_at FROM Chat.messages WHERE id = $1;`,

			mess_id)
		errorr(err)
		fmt.Println(err)

		var sent_at time.Time

		for request.Next() {
			err := request.Scan(&sent_at)

			if err != nil {
				fmt.Println(err)

				continue
			}
		}
		fmt.Println(err)
	} else {

		response := Response{
			Status:  "fatal",
			Message: "Не отправленно",
		}

		json.NewEncoder(rw).Encode(response)
	}

	for i := range file_paths {
		request, err := rep.Query(
			ctx,
			`
				INSERT INTO Chat.Attachments (message_id, path_to_file)
				SELECT $1, $2
				RETURNING id;
			`,

			mess_id,
			file_paths[i],
		)
		errorr(err)
		fmt.Println(err)

		var attachments_id int

		for request.Next() {
			err := request.Scan(
				&attachments_id,
			)
			if err != nil {
				fmt.Println(err)

				continue
			}
		}
		fmt.Println(err)

		type A struct {
			Mess_id  int `json:"Mess_id"`
			Photo_id int `json:"Photo_id"`
		}

		type Response struct {
			Status  string `json:"status"`
			Data    string `json:"data,omitempty"`
			Message string `json:"message"`
		}

		image, err := DownloadFile(file_paths[i])

		if err == nil && mess_id != 0 {
			response := Response{
				Status:  "success",
				Data:    image,
				Message: fmt.Sprintf("Фото № %d доставленно. Текст № %d доставленн", attachments_id, mess_id),
			}

			request, err := rep.Query(
				ctx,
				`SELECT sent_at FROM Chat.messages WHERE id = $1;`,

				mess_id)
			errorr(err)
			fmt.Println(err)

			var sent_at time.Time

			for request.Next() {
				err := request.Scan(&sent_at)

				if err != nil {
					fmt.Println(err)

					continue
				}
			}
			fmt.Println(err)

			//достаём аву и имя
			request, err = rep.Query(
				ctx,
				`
						WITH i AS (
							SELECT user_1_id, user_2_id 
							FROM Chat.chats 
							WHERE id = $1
						)
						SELECT COALESCE(
							(SELECT user_1_id FROM i WHERE user_1_id != $2),
							(SELECT user_2_id FROM i WHERE user_2_id != $2)
						);
					`,
				id_chat,
				id_user,
			)
			errorr(err)

			var buddy_id int

			for request.Next() {
				err := request.Scan(
					&buddy_id,
				)
				if err != nil {
					fmt.Println(err)

					continue
				}
			}

			//достаём аву, имя и роль
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
				id_user,
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

			json.NewEncoder(rw).Encode(response)

			// buddy_id это кому отправляем сообщение
			Notification(ctx, buddy_id, conn[buddy_id], "message", id_chat, mess_id, text, sent_at, id_user, user_role, ServeSpecificMediaBase64(rw, r, avatar_path), name, rdb)
		} else {

			response := Response{
				Status:  "fatal",
				Message: "Не отправленно",
			}

			rw.WriteHeader(http.StatusOK)
			json.NewEncoder(rw).Encode(response)

		}
	}
	return
}

func (repo *MyRepository) SendImageSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, id_chat, id_user int, file_path []string, conn map[int]*websocket.Conn, rdb *redis.Client) (err error) {
	request, err := rep.Query(
		ctx,
		`
		INSERT INTO Chat.messages(chat_id, sender_id, text)
		VALUES ($1, $2, $3)
		RETURNING id;
	`,

		id_chat,
		id_user,
		"$IMAGE$",
	)
	errorr(err)

	var mess_id int

	for request.Next() {
		err := request.Scan(
			&mess_id,
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

	if err == nil && mess_id != 0 {
		response := Response{
			Status:  "success",
			Data:    mess_id,
			Message: "Текст сообщения принято",
		}

		request, err := rep.Query(
			ctx,
			`SELECT sent_at FROM Chat.messages WHERE id = $1;`,

			mess_id)
		errorr(err)

		var sent_at time.Time

		for request.Next() {
			err := request.Scan(&sent_at)

			if err != nil {
				fmt.Println(err)

				continue
			}
		}

		json.NewEncoder(rw).Encode(response)

	} else {

		response := Response{
			Status:  "fatal",
			Message: "Не отправленно",
		}

		json.NewEncoder(rw).Encode(response)
	}

	for i := range file_path {

		request, err := rep.Query(
			ctx,
			`
				INSERT INTO Chat.Attachments (message_id, path_to_file)
				SELECT $1, $2
				RETURNING id;
			`,

			mess_id,
			file_path[i],
		)
		errorr(err)

		var attachments_id int

		for request.Next() {
			err := request.Scan(
				&attachments_id,
			)
			if err != nil {
				fmt.Println(err)

				continue
			}
		}

		type A struct {
			Mess_id  int `json:"Mess_id"`
			Photo_id int `json:"Photo_id"`
		}

		type Response struct {
			Status  string `json:"status"`
			Data    string `json:"data,omitempty"`
			Message string `json:"message"`
		}

		image, err := DownloadFile(file_path[i])

		if err == nil && mess_id != 0 {
			response := Response{
				Status:  "success",
				Data:    image,
				Message: fmt.Sprintf("Фото № %d доставленно", attachments_id),
			}

			request, err := rep.Query(
				ctx,
				`SELECT sent_at FROM Chat.messages WHERE id = $1;`,

				mess_id)
			errorr(err)

			var sent_at time.Time

			for request.Next() {
				err := request.Scan(&sent_at)

				if err != nil {
					fmt.Println(err)

					continue
				}
			}

			json.NewEncoder(rw).Encode(response)

			//достаём аву и имя
			request, err = rep.Query(
				ctx,
				`
						WITH i AS (
							SELECT user_1_id, user_2_id 
							FROM Chat.chats 
							WHERE id = $1
						)
						SELECT COALESCE(
							(SELECT user_1_id FROM i WHERE user_1_id != $2),
							(SELECT user_2_id FROM i WHERE user_2_id != $2)
						);
					`,
				id_chat,
				id_user,
			)
			errorr(err)

			var buddy_id int

			for request.Next() {
				err := request.Scan(
					&buddy_id,
				)
				if err != nil {
					fmt.Println(err)

					continue
				}
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
				id_user,
			)
			errorr(err)

			var user_role int
			var avatar_path string
			var name string

			for request.Next() {
				err := request.Scan(
					&avatar_path,
					&name,
					user_role,
				)
				if err != nil {
					fmt.Println(err)

					continue
				}
			}

			json.NewEncoder(rw).Encode(response)

			Notification(ctx, buddy_id, conn[buddy_id], "message", id_chat, mess_id, "image", sent_at, id_user, user_role, ServeSpecificMediaBase64(rw, r, avatar_path), name, rdb)

			return err

		} else {

			response := Response{
				Status:  "fatal",
				Message: "Не отправленно",
			}

			rw.WriteHeader(http.StatusOK)
			json.NewEncoder(rw).Encode(response)

		}
	}
	return err
}

func (repo *MyRepository) SendMessageSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, conn map[int]*websocket.Conn, id_chat, id_user int, text string, rdb *redis.Client) (err error) {
	//текст есть, но изображений нет
	request, err := rep.Query(
		ctx,
		`
			INSERT INTO Chat.messages(chat_id, sender_id, text)
			VALUES ($1, $2, $3)
			RETURNING id;
		`,

		id_chat,
		id_user,
		text,
	)
	errorr(err)

	var mess_id int

	for request.Next() {
		err := request.Scan(
			&mess_id,
		)
		if err != nil {
			fmt.Println(err)

			continue
		}
	}

	type Ints struct {
		Mess_id int
		Text    string
		Sent_at time.Time
	}

	type Response struct {
		Status  string `json:"status"`
		Data    Ints   `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err == nil && true {
		request, err := rep.Query(
			ctx,
			`SELECT sent_at FROM Chat.messages WHERE id = $1;`,

			mess_id)
		errorr(err)

		var sent_at time.Time

		for request.Next() {
			err := request.Scan(&sent_at)

			if err != nil {
				fmt.Println(err)

				continue
			}
		}

		//достаём аву и имя
		request, err = rep.Query(
			ctx,
			`
				WITH i AS (
					SELECT user_1_id, user_2_id 
					FROM Chat.chats 
					WHERE id = $1
				)
				SELECT COALESCE(
					(SELECT user_1_id FROM i WHERE user_1_id != $2),
					(SELECT user_2_id FROM i WHERE user_2_id != $2)
				);
			`,
			id_chat,
			id_user,
		)
		errorr(err)

		var buddy_id int

		for request.Next() {
			err := request.Scan(
				&buddy_id,
			)
			if err != nil {
				fmt.Println(err)

				continue
			}
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

			id_user,
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

		response := Response{
			Status:  "success",
			Data:    Ints{Mess_id: mess_id, Text: text, Sent_at: sent_at},
			Message: "Показано",
		}

		json.NewEncoder(rw).Encode(response)

		Notification(ctx, buddy_id, conn[buddy_id], "message", id_chat, mess_id, text, sent_at, id_user, user_role, ServeSpecificMediaBase64(rw, r, avatar_path), name, rdb)

		return err
	}

	response := Response{
		Status:  "fatal",
		Message: "Не показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) SendVideoSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, id_chat, id_user int, file_path []string, conn map[int]*websocket.Conn, rdb *redis.Client) (err error) {
	request, err := rep.Query(
		ctx,
		`
		INSERT INTO Chat.messages(chat_id, sender_id, text)
		VALUES ($1, $2, $3)
		RETURNING id;
	`,

		id_chat,
		id_user,
		"$VIDEO$",
	)
	errorr(err)

	var mess_id int

	for request.Next() {
		err := request.Scan(
			&mess_id,
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

	if err == nil && mess_id != 0 {
		request, err := rep.Query(
			ctx,
			`SELECT sent_at FROM Chat.messages WHERE id = $1;`,

			mess_id)
		errorr(err)

		var sent_at time.Time

		for request.Next() {
			err := request.Scan(&sent_at)

			if err != nil {
				fmt.Println(err)

				continue
			}
		}
	} else {

		response := Response{
			Status:  "fatal",
			Message: "Не отправленно",
		}

		json.NewEncoder(rw).Encode(response)
	}

	for i := range file_path {

		request, err := rep.Query(
			ctx,
			`
				INSERT INTO Chat.Attachments (message_id, path_to_file)
				SELECT $1, $2
				RETURNING id;
			`,

			mess_id,
			file_path[i],
		)
		errorr(err)

		var attachments_id int

		for request.Next() {
			err := request.Scan(
				&attachments_id,
			)
			if err != nil {
				fmt.Println(err)

				continue
			}
		}

		type A struct {
			Mess_id  int `json:"Mess_id"`
			Photo_id int `json:"Photo_id"`
		}

		type Response struct {
			Status  string `json:"status"`
			Data    string `json:"data,omitempty"`
			Message string `json:"message"`
		}

		image, err := DownloadFile(file_path[i])

		if err == nil && mess_id != 0 {
			response := Response{
				Status:  "success",
				Data:    image,
				Message: fmt.Sprintf("Фото № %d доставленно. Сообщение № %d доставленно", attachments_id, mess_id),
			}

			request, err := rep.Query(
				ctx,
				`SELECT sent_at FROM Chat.messages WHERE id = $1;`,

				mess_id)
			errorr(err)

			var sent_at time.Time

			for request.Next() {
				err := request.Scan(&sent_at)

				if err != nil {
					fmt.Println(err)

					continue
				}
			}

			//достаём аву и имя
			request, err = rep.Query(
				ctx,
				`
						WITH i AS (
							SELECT user_1_id, user_2_id 
							FROM Chat.chats 
							WHERE id = $1
						)
						SELECT COALESCE(
							(SELECT user_1_id FROM i WHERE user_1_id != $2),
							(SELECT user_2_id FROM i WHERE user_2_id != $2)
						);
					`,
				id_chat,
				id_user,
			)
			errorr(err)

			var buddy_id int

			for request.Next() {
				err := request.Scan(
					&buddy_id,
				)
				if err != nil {
					fmt.Println(err)

					continue
				}
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
				id_user,
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

			json.NewEncoder(rw).Encode(response)

			Notification(ctx, buddy_id, conn[buddy_id], "message", id_chat, mess_id, "image", sent_at, id_user, user_role, ServeSpecificMediaBase64(rw, r, avatar_path), name, rdb)

			return err

		} else {

			response := Response{
				Status:  "fatal",
				Message: "Не отправленно",
			}

			rw.WriteHeader(http.StatusOK)
			json.NewEncoder(rw).Encode(response)

		}
	}
	return err
}

func (repo *MyRepository) PrintChatSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, id_user int) (err error) {
	request, err := rep.Query(
		ctx,
		`
		WITH i AS (
			SELECT id AS chai_id, user_1_id, user_2_id
			FROM chat.chats
			WHERE user_1_id = $1 OR user_2_id = $1 OR mediator_id = $1
		),
		latest_messages AS (
			SELECT DISTINCT ON (m.chat_id) m.sender_id, m.text, m.sent_at, m.chat_id, attachm.message_id
			FROM chat.messages m
			LEFT JOIN chat.attachments attachm ON attachm.message_id = m.id
			WHERE m.chat_id IN (SELECT chai_id FROM i)
			AND m.sender_id IN (SELECT user_1_id FROM i UNION SELECT user_2_id FROM i)
			ORDER BY m.chat_id, m.sent_at DESC
		),
		buddy AS (
			SELECT id AS chai_id,
				CASE 
					WHEN user_1_id != $1 THEN user_1_id 
					WHEN user_2_id != $1 THEN user_2_id 
				END AS buddy_id
			FROM chat.chats
			WHERE user_1_id = $1 OR user_2_id = $1 OR mediator_id = $1
		),
		buddy_info AS (
			SELECT buddy.chai_id, name::text AS info
			FROM users.individual_user 
			JOIN buddy ON buddy.buddy_id = users.individual_user.user_id
			UNION
			SELECT buddy.chai_id, name_of_company::text AS info
			FROM users.company_user 
			JOIN buddy ON buddy.buddy_id = users.company_user.user_id
		),
		avatar AS (
			SELECT buddy.chai_id, avatar_path 
			FROM users.users 
			JOIN buddy ON buddy.buddy_id = users.users.id
		)
		SELECT 
			i.chai_id,
			latest_messages.sender_id,
			latest_messages.text,
			latest_messages.sent_at,
			buddy_info.info,
			COALESCE(avatar.avatar_path, '/root/home/beeline_project/media/user/image_10290308543_ava.png') AS avatar_path
		FROM i
		LEFT JOIN latest_messages ON latest_messages.chat_id = i.chai_id
		LEFT JOIN buddy_info ON buddy_info.chai_id = i.chai_id
		LEFT JOIN avatar ON avatar.chai_id = i.chai_id;
		`,

		id_user,
	)

	type Productt struct {
		Chat_id   int        `json:"Chat_id"`
		Sender_id *int       `json:"sender_id"`
		Text      *string    `json:"text"`
		Sent_at   *time.Time `json:"sent_at"`
		Info      *string    `json:"info"`
		Avatar    string     `json:"avatar"`
	}

	products := []Productt{}

	type Response struct {
		Status  string     `json:"status"`
		Data    []Productt `json:"data,omitempty"`
		Message string     `json:"message"`
	}

	var Avatar_path string

	for request.Next() {
		p := Productt{}
		err := request.Scan(
			&p.Chat_id,
			&p.Sender_id,
			&p.Text,
			&p.Sent_at,
			&p.Info,
			&Avatar_path,
		)

		if err != nil {
			response := Response{
				Status:  "fatal",
				Message: "Возникла ошибка" + err.Error(),
			}

			json.NewEncoder(rw).Encode(response)
			return err
		}

		p.Avatar, err = DownloadFile(Avatar_path)
		// if err != nil {
		// 	response := Response{
		// 		Status:  "fatal",
		// 		Message: "Возникла ошибка с изображением " + err.Error() + ". Id чата: " + strconv.Itoa(p.Chat_id),
		// 	}

		// 	json.NewEncoder(rw).Encode(response)
		// }

		products = append(products, Productt{
			Chat_id:   p.Chat_id,
			Sender_id: p.Sender_id,
			Text:      p.Text,
			Sent_at:   p.Sent_at,
			Info:      p.Info,
			Avatar:    p.Avatar,
		})
	}

	if err != nil {
		response := Response{
			Status:  "fatal",
			Message: "Не сгруппировано",
		}

		json.NewEncoder(rw).Encode(response)
		return err
	} else if len(products) == 0 {
		response := Response{
			Status:  "success",
			Data:    products,
			Message: "Чатов не найденно, либо они не созданны",
		}

		json.NewEncoder(rw).Encode(response)

		return
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
