package order

import (
	"context"
	"encoding/json"
	"fmt"
	"myproject/internal/database"
	"myproject/internal/jwt"
	"myproject/internal/model"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"
)

func RegOrderHourly(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool, сonnection map[int]*websocket.Conn) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.order.RegOrderHourly"

		var booking model.Booking

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&booking)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))
		}

		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать JWT", op))
		} else {
			flag, user_id, user_role := jwt.IsAuthorized(token.Value)

			//проверка средств и отслеживание пополнения
			request, err := dbpool.Query(
				ctx,
				`
				WITH wallet AS (
					SELECT total_balance FROM finance.wallets WHERE user_id = $1
				),
				owner AS (
					SELECT owner_id, hourly_rate FROM ads.ads WHERE id = $2
				)
				SELECT (SELECT total_balance FROM wallet) >= (SELECT $3::timestamp::date - $4::timestamp::date) * 
					(SELECT hourly_rate FROM owner) AS summ_of_money
				`,

				user_id,
				booking.Ads_id,
				booking.Ends_at,
				booking.Ads_id,
			)

			var summOfMoney bool

			for request.Next() {
				err := request.Scan(&summOfMoney)

				if err != nil {
					fmt.Println(err)
					continue
				}
			}

			type Response struct {
				Status  string `json:"status"`
				Data    string `json:"data,omitempty"`
				Message string `json:"message"`
			}

			if !summOfMoney {
				response := Response{
					Status:  "fatal",
					Data:    "key:" + time.Now().Format("020106"),
					Message: "Сведств недостаточно, пополните счёт на необходимую сумму",
				}

				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(response)
				return
			}

			if flag && user_role == 1 {
				err = repo.RegOrderHourlySQL(ctx, w, dbpool, r, redisClient, сonnection, user_id, booking.Ads_id, time.Unix(booking.Starts_at, 0).UTC(), time.Unix(booking.Ends_at, 0).UTC(), booking.PositionX, booking.PositionY)
				if err != nil {
					logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой RegOrderHourlySQL", op))
				}
			}
		}
	}
}

func RegOrderDaily(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool, conn map[int]*websocket.Conn) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.order.RegOrderDaily"

		var booking model.Booking

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&booking)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))
		}

		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать JWT", op))
		}
		flag, user_id, user_role := jwt.IsAuthorized(token.Value)

		//проверка средств и отслеживание пополнения
		request, err := dbpool.Query(
			ctx,
			`
			WITH wallet AS (
				SELECT total_balance FROM finance.wallets WHERE user_id = $1
			),
			owner AS (
				SELECT owner_id, daily_rate FROM ads.ads WHERE id = $2
			)
			SELECT (SELECT total_balance FROM wallet) >= (SELECT $3::timestamp::date - $4::timestamp::date) * 
				(SELECT daily_rate FROM owner) AS summ_of_money
			`,

			user_id,
			booking.Ads_id,
			booking.Ends_at,
			booking.Ads_id,
		)

		var summOfMoney bool

		for request.Next() {
			err := request.Scan(&summOfMoney)

			if err != nil {
				fmt.Println(err)
				continue
			}
		}

		type Response struct {
			Status  string `json:"status"`
			Data    string `json:"data,omitempty"`
			Message string `json:"message"`
		}

		if !summOfMoney {
			response := Response{
				Status:  "fatal",
				Data:    "key:" + time.Now().Format("020106"),
				Message: "Сведств недостаточно, пополните счёт на необходимую сумму",
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}

		if user_role != 1 {
			response := model.Response{
				Status:  "fatal",
				Message: "У вас нет доступа на эту операцию",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		if !flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		err = repo.RegOrderDailySQL(ctx, w, dbpool, r, redisClient, conn, user_id, booking.Ads_id, time.Unix(booking.Starts_at, 0).UTC(), time.Unix(booking.Ends_at, 0).UTC(), booking.PositionX, booking.PositionY)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой RegOrderDailySQL", op))
		}
	}
}

func RebookOrder(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.order.RebookOrder"

		var booking model.Booking_rebookOrderHourly

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&booking)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))
		}

		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать JWT", op))
		}
		flag, user_id, user_role := jwt.IsAuthorized(token.Value)

		if user_role != 1 {
			response := model.Response{
				Status:  "fatal",
				Message: "У вас нет доступа на эту операцию",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		if !flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		err = repo.RebookOrderHourlySQL(ctx, w, dbpool, user_id, booking.Order_id, time.Unix(booking.Starts_at, 0).UTC(), time.Unix(booking.Ends_at, 0).UTC())
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой RebookOrderHourlySQL", op))
		}
	}
}

func GroupOrdersByRented(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.order.GroupOrdersByRented"

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))
		}

		flag, user_id, user_role := jwt.IsAuthorized(token.Value)
		if user_role != 1 {
			response := model.Response{
				Status:  "fatal",
				Message: "У вас нет доступа на эту операцию",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		if !flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		err = repo.GroupAdsByRentedSQL(ctx, w, dbpool, user_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой GroupAdsByRentedSQL", op))
		}
	}
}

func GroupOrdersByUnRented(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.order.GroupOrdersByUnRented"

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))
		}

		flag, user_id, user_role := jwt.IsAuthorized(token.Value)
		if user_role != 1 {
			response := model.Response{
				Status:  "fatal",
				Message: "У вас нет доступа на эту операцию",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		if !flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		err = repo.GroupAdsByArchivedSQL(ctx, w, user_id, dbpool)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой GroupAdsByArchivedSQL", op))
		}
	}
}

func RegOrderBidding(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool, conn map[int]*websocket.Conn) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.order.Bidding"

		var bidding model.Bidding

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&bidding)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))
		}

		flag, user_id, user_role := jwt.IsAuthorized(token.Value)

		//проверка средств и отслеживание пополнения
		request, err := dbpool.Query(
			ctx,
			`
			WITH wallet AS (
				SELECT total_balance FROM finance.wallets WHERE user_id = $1
			)
			SELECT (SELECT total_balance FROM wallet) >= $2 AS summ_of_money
			`,

			user_id,
			bidding.Global_rate,
		)

		var summOfMoney bool

		for request.Next() {
			err := request.Scan(&summOfMoney)

			if err != nil {
				fmt.Println(err)
				continue
			}
		}

		type Response struct {
			Status  string `json:"status"`
			Data    string `json:"data,omitempty"`
			Message string `json:"message"`
		}

		if !summOfMoney {
			response := Response{
				Status:  "fatal",
				Data:    "key:" + time.Now().Format("020106"),
				Message: "Сведств недостаточно, пополните счёт на необходимую сумму",
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}

		if user_role != 1 {
			response := model.Response{
				Status:  "fatal",
				Message: "У вас нет доступа на эту операцию",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		if !flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		err = repo.RegOrderWithBiddingSQL(ctx, w, dbpool, r, redisClient, conn, bidding.Chat_id, bidding.Global_rate, user_id, time.Unix(bidding.Start_at, 0).UTC(), time.Unix(bidding.End_at, 0).UTC(), bidding.PositionX, bidding.PositionY)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой BiddingSQL", op))
		}
	}
}

func ComplBooking(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.order.ComplBooking"

		var booking model.ComplBooking

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&booking)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in ", op, "; ошибка с парсингом JSON-запроса"))
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in ", op, "; ошибка с попыткой прочитать куку"))
		}

		flag, user_id, user_role := jwt.IsAuthorized(token.Value)

		if user_role != 1 {
			response := model.Response{
				Status:  "fatal",
				Message: "У вас нет доступа на эту операцию",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		if !flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		err = repo.SucBookingSQL(ctx, w, dbpool, booking.Bookings_id, user_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой SucBookingSQL", op))
		}
	}
}

func BookingList(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.order.BookingList"

		repo := database.NewRepo(ctx, dbpool)

		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))
		}

		flag, user_id, user_role := jwt.IsAuthorized(token.Value)
		if user_role != 1 {
			response := model.Response{
				Status:  "fatal",
				Message: "У вас нет доступа на эту операцию",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		if !flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		err = repo.BookingListSQL(ctx, w, dbpool, user_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой BookingListSQL", op))
		}
	}
}

func SigPDFfile(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.order.BookingList"

		var booking model.Owner_id

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&booking)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in ", op, "; ошибка с парсингом JSON-запроса"))
		}

		repo := database.NewRepo(ctx, dbpool)

		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))
		}

		flag, user_id, user_role := jwt.IsAuthorized(token.Value)
		if user_role != 1 {
			response := model.Response{
				Status:  "fatal",
				Message: "У вас нет доступа на эту операцию",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		if !flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		err = repo.SigPDFfileSQL(ctx, w, dbpool, booking.Owner_id, user_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой BookingListSQL", op))
		}
	}
}

// func CompletBookingOutput(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool, conn map[int]*websocket.Conn) httprouter.Handle {
// 	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 		op := "internal.services.order.CompletBookingOutput"

// 		repo := database.NewRepo(ctx, dbpool)

// 		token, err := r.Cookie("token")
// 		if err != nil {
// 			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))
// 		}

// 		flag, user_id, user_role := jwt.IsAuthorized(token.Value)
// 		if user_role != 1 {
// 			response := model.Response{
// 				Status:  "fatal",
// 				Message: "У вас нет доступа на эту операцию",
// 			}

// 			json.NewEncoder(w).Encode(response)

// 			return
// 		}

// 		if !flag {
// 			response := model.Response{
// 				Status:  "fatal",
// 				Message: "Что-то не так с JWT",
// 			}

// 			json.NewEncoder(w).Encode(response)

// 			return
// 		}

// 		var coplBooking model.Owner_id

// 		// Парсинг JSON-запроса
// 		err = json.NewDecoder(r.Body).Decode(&coplBooking)
// 		if err != nil {
// 			logger.Err(err).Msg(fmt.Sprintf("error in ", op, "; ошибка с парсингом JSON-запроса"))
// 		}

// 		err = repo.CompletBookingOutputSQL(ctx, w, dbpool, coplBooking.Owner_id, user_id)
// 		if err != nil {
// 			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой BookingListSQL", op))
// 		}
// 	}
// }
