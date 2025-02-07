package mediator

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

func DisputeChatPanel(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.mediator.DisputeChatPanel"

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))
		}

		flag, _, user_role := jwt.IsAuthorized(token.Value)

		if user_role != 2 {
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

		err = repo.DisputeChatPanelSQL(ctx, dbpool, w)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой DisputeChatPanelSQL", op))
		}
	}
}

func MediatorEnterInChat(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.mediator.MediatorEnterInChat"

		var openChat model.Chat

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&openChat)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))

			return
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))
		}

		flag, mediator_id, user_role := jwt.IsAuthorized(token.Value)

		if user_role != 2 {
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

		err = repo.MediatorEnterInChatSQL(ctx, w, dbpool, openChat.Id_chat, mediator_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой MediatorEnterInChatSQL", op))

			return
		}
	}
}

func RebookList(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.mediator.RebookList"

		var ads_id model.Ads_id

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&ads_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))

			return
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))

			return
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

		err = repo.RebookListSQL(ctx, w, dbpool, ads_id.Ads_id, user_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой RebookListSQL", op))

			return
		}
	}
}

func MediatorFinishJobUser(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.mediator.MediatorFinishJobUser"

		var mediatorFinishJob model.MediatorFinishJob

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&mediatorFinishJob)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))

			response := model.Response{
				Status:  "fatal",
				Message: "Не получилось обработать данные",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))

			return
		}

		flag, user_id, user_role := jwt.IsAuthorized(token.Value)

		if user_role != 2 {
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

		err = repo.MediatorFinishJobUserSQL(ctx, w, dbpool, logger, mediatorFinishJob.Chat_id, mediatorFinishJob.Amount, mediatorFinishJob.Comment, user_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой MediatorFinishJobInChatSQL", op))

			return
		}
	}
}

func MediatorFinishJobOwner(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.mediator.MediatorFinishJobOwner"

		var mediatorFinishJob model.MediatorFinishJob

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&mediatorFinishJob)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))

			return
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))

			return
		}

		flag, _, user_role := jwt.IsAuthorized(token.Value)

		if user_role != 2 {
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

		err = repo.MediatorFinishJobOwnerSQL(ctx, w, dbpool, logger, mediatorFinishJob.Chat_id, mediatorFinishJob.Comment)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой MediatorFinishJobInChatSQL", op))

			return
		}
	}
}

func RegReport(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.mediator.RegReport"

		var report model.Report

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&report)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))
		}

		flag, _, user_role := jwt.IsAuthorized(token.Value)

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

		err = repo.RegReportSQL(ctx, w, dbpool, report.Order_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой RegReportSQL", op))
		}
	}
}
