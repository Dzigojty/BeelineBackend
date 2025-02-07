package finance

import (
	"context"
	"encoding/json"
	"fmt"
	"myproject/internal/database"
	"myproject/internal/jwt"
	"myproject/internal/model"
	"net/http"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"
)

func TransactionToAnother(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.finance.TransactionToAnother"

		var transact model.Transact

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&transact)
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

		err = repo.TransactionToAnotherSQL(ctx, w, dbpool, user_id, transact.User_2, transact.Amount)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s", op, "; ошибка с процедурой TransactionToAnotherSQL"))
		}
	}
}

func TransactionToSomething(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.finance.TransactionToSomething"

		var transact model.Transact

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&transact)
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

		err = repo.TransactionToSomethingSQL(ctx, w, dbpool, user_id, transact.Amount)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s", op, "; ошибка с процедурой TransactionToSomethingSQL"))
		}
	}
}

func TransactionToReturnAmount(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.finance.TransactionToReturnAmount"

		var transact model.Transact

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&transact)
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

		err = repo.TransactionToReturnAmountSQL(ctx, w, dbpool, user_id, transact.Amount)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s", op, "; ошибка с процедурой TransactionToReturnAmountSQL"))
		}
	}
}

func WalletHistory(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.finance.WalletHistory"

		var walletHist model.WalletHistory

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&walletHist)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in ", op, "; ошибка с парсингом JSON-запроса"))
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in ", op, "; ошибка с попыткой прочитать куку"))
		}

		token_flag, user_id, user_role := jwt.IsAuthorized(token.Value)

		if user_role != 1 {
			response := model.Response{
				Status:  "fatal",
				Message: "У вас нет доступа на эту операцию",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		if !token_flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		err = repo.WalletHistorySQL(ctx, w, dbpool, r, user_id, walletHist.Typee)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s", op, "; ошибка с процедурой WalletHistorySQL"))
		}
	}
}

func WalletList(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.finance.WalletList"

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in ", op, "; ошибка с попыткой прочитать куку"))
		}

		token_flag, user_id, user_role := jwt.IsAuthorized(token.Value)

		if user_role != 1 {
			response := model.Response{
				Status:  "fatal",
				Message: "У вас нет доступа на эту операцию",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		if !token_flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		err = repo.WalletListSQL(ctx, w, dbpool, r, user_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s", op, "; ошибка с процедурой WalletListSQL"))
		}
	}
}
